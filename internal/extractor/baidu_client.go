package extractor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"legal-extractor/internal/config"
	"log/slog"
	"net/http"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// BaiduClient 百度 AI Studio PaddleOCR 客户端。
//
// 配置（Token / ApiUrl）在每次调用时从 config 实时读取，
// 用户补填 conf.yaml 或环境变量后无需重启应用。
type BaiduClient struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// BaiduOCRResponse 百度 Layout Parsing 响应结构（PaddleX 服务化契约）。
// prunedResult/dataInfo 为可选增强：用 RawMessage 接住后逐页 best-effort
// 解析——字段缺失或形状随部署版本变化时坐标行退化，markdown 主流程不受影响。
type BaiduOCRResponse struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	Result    struct {
		LayoutParsingResults []struct {
			Markdown struct {
				Text string `json:"text"`
			} `json:"markdown"`
			PrunedResult json.RawMessage `json:"prunedResult"`
		} `json:"layoutParsingResults"`
		DataInfo json.RawMessage `json:"dataInfo"`
	} `json:"result"`
}

// baiduPrunedResult prunedResult 中与坐标相关的部分
type baiduPrunedResult struct {
	OverallOcrRes struct {
		RecTexts []string      `json:"rec_texts"`
		RecBoxes [][]float64   `json:"rec_boxes"`
		RecPolys [][][]float64 `json:"rec_polys"`
	} `json:"overall_ocr_res"`
}

// baiduDataInfo 输入数据信息：图片为顶层宽高，PDF 为逐页栅格尺寸
// （与 rec_boxes 同一像素空间）
type baiduDataInfo struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Pages  []struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"pages"`
}

// ocrPage callBaiduAPI 的单页产物：markdown 用于字段提取，
// lines 为归一化坐标文字行（可能为 nil，见 buildOcrLines）。
type ocrPage struct {
	markdown string
	lines    []OcrLine
}

// NewBaiduClient 创建百度 OCR 客户端
func NewBaiduClient(logger *slog.Logger) *BaiduClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &BaiduClient{
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // 复杂长文档需要充足的云端处理时间
		},
		logger: logger,
	}
}

// HasToken 返回当前配置是否已提供云端 OCR Token。
func (c *BaiduClient) HasToken() bool {
	return config.GetBaidu().Token != ""
}

// httpStatusError 标记云端返回的非 200 状态码，供重试逻辑做类型化判断，
// 避免对错误文本做字符串匹配。
type httpStatusError struct {
	StatusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("百度 API 响应异常 (HTTP %d)", e.StatusCode)
}

// isRetryableStatus 仅对云端 5xx 状态错误重试。
func isRetryableStatus(err error) bool {
	var se *httpStatusError
	return errors.As(err, &se) && se.StatusCode >= 500
}

// ParseDocument 调用百度 Layout Parsing 接口解析文档。
//
// totalPages 由调用方传入（extractPdf 已完成多库回退的页数探测，图片固定为 1），
// 避免此处重复解析；超过分块阈值的 PDF 会被物理切片后逐块识别。
// 返回结构化记录与逐页归一化坐标文字行（供前端字段高亮）。
func (c *BaiduClient) ParseDocument(ctx context.Context, fileData []byte, totalPages int, isPdf bool, onProgress ProgressCallback) (ExtractOutput, error) {
	c.logger.Info("开始调用百度 OCR 接口", "isPdf", isPdf, "totalPages", totalPages, "dataSize", len(fileData))
	if len(fileData) == 0 {
		return ExtractOutput{}, ErrEmptyFile
	}
	if !c.HasToken() {
		return ExtractOutput{}, fmt.Errorf("%w: 请检查 config/conf.yaml", ErrTokenMissing)
	}
	if totalPages < 1 {
		totalPages = 1
	}

	// 1. 获取所有页面的 Markdown 与坐标行（超过阈值的 PDF 物理分块，防止云端超时）
	const maxPagesPerChunk = 20 // 小分块粒度显著提升云端解析稳定性

	var allPages []ocrPage
	switch {
	case isPdf && totalPages > maxPagesPerChunk:
		c.logger.Info("启用大文件物理分块处理模式", "chunkSize", maxPagesPerChunk)
		for start := 1; start <= totalPages; start += maxPagesPerChunk {
			end := min(start+maxPagesPerChunk-1, totalPages)
			pages, err := c.parseChunkWithRetry(ctx, fileData, start, end, totalPages, onProgress)
			if err != nil {
				return ExtractOutput{}, err
			}
			allPages = append(allPages, sanitizeChunkPages(pages, end-start+1, c.logger)...)

			// 强制冷却，防止连续高压导致百度后端崩溃
			if end < totalPages {
				c.logger.Info("分块处理完成，进入 10 秒冷却期以释放云端算力...")
				if err := sleepCtx(ctx, 10*time.Second); err != nil {
					return ExtractOutput{}, ErrCancelled
				}
			}
		}
	case isPdf:
		if onProgress != nil {
			onProgress(1, totalPages, "正在进行深度识别与内容校对，请稍候...")
		}
		pages, err := c.callBaiduAPI(ctx, fileData, true, onProgress)
		if err != nil {
			return ExtractOutput{}, err
		}
		allPages = pages
	default: // 图片
		if onProgress != nil {
			onProgress(1, 1, "正在对图片进行语义化识别...")
		}
		pages, err := c.callBaiduAPI(ctx, fileData, false, onProgress)
		if err != nil {
			return ExtractOutput{}, err
		}
		allPages = pages
	}

	// 2. 按页解析汇总后的 Markdown，坐标行按全局页序号归档
	c.logger.Info("所有页面识别完成，开始按页提取法律实体", "totalFetchedPages", len(allPages))
	var allRecords []Record
	var ocrPages []OcrPageLines
	fetched := len(allPages)
	for i, pg := range allPages {
		if err := ctx.Err(); err != nil {
			return ExtractOutput{}, ErrCancelled
		}
		if onProgress != nil {
			onProgress(i+1, fetched, fmt.Sprintf("正在结构化提取第 %d/%d 页的法律信息...", i+1, fetched))
		}
		for _, rec := range ParseMarkdown(pg.markdown) {
			// 标注准确的页码
			if rec["page"] == "" {
				rec["page"] = fmt.Sprintf("%d", i+1)
			}
			allRecords = append(allRecords, rec)
		}
		if len(pg.lines) > 0 {
			ocrPages = append(ocrPages, OcrPageLines{Page: i + 1, Lines: pg.lines})
		}
	}

	c.logger.Info("数据提取完成", "recordCount", len(allRecords), "ocrPageCount", len(ocrPages))
	return ExtractOutput{Records: allRecords, OcrPages: ocrPages}, nil
}

// parseChunkWithRetry 物理切出 [start, end] 页并调用云端识别，
// 云端 5xx 时执行“避让重试”（最多 2 次，间隔 20 秒）。
func (c *BaiduClient) parseChunkWithRetry(ctx context.Context, fileData []byte, start, end, totalPages int, onProgress ProgressCallback) ([]ocrPage, error) {
	c.logger.Info(fmt.Sprintf("正在处理分块: 第 %d-%d 页", start, end))
	if onProgress != nil {
		onProgress(start, totalPages, "正在分析文档结构并准备解析...")
	}

	var chunkBuffer bytes.Buffer
	pageSelection := []string{fmt.Sprintf("%d-%d", start, end)}
	if err := api.Trim(bytes.NewReader(fileData), &chunkBuffer, pageSelection, nil); err != nil {
		return nil, fmt.Errorf("PDF 切片失败: %w", err)
	}

	const maxRetries = 2
	for retry := 0; ; retry++ {
		if retry > 0 {
			c.logger.Warn(fmt.Sprintf("分块 %d-%d 尝试第 %d 次重试...", start, end, retry))
			if err := sleepCtx(ctx, 20*time.Second); err != nil {
				return nil, ErrCancelled
			}
		}

		if onProgress != nil {
			onProgress(start, totalPages, fmt.Sprintf("正在对第 %d-%d 页进行深度识别...", start, end))
		}

		pages, err := c.callBaiduAPI(ctx, chunkBuffer.Bytes(), true, onProgress)
		if err == nil {
			return pages, nil
		}
		if !isRetryableStatus(err) || retry >= maxRetries {
			return nil, err
		}
	}
}

// callBaiduAPI 封装底层的 API 调用逻辑，返回逐页 markdown 与归一化坐标行
func (c *BaiduClient) callBaiduAPI(ctx context.Context, fileData []byte, isPdf bool, onProgress ProgressCallback) ([]ocrPage, error) {
	c.logger.Info("正在向百度 AI Studio 发送 POST 请求...")
	fileBase64 := base64.StdEncoding.EncodeToString(fileData)
	fileType := 1
	if isPdf {
		fileType = 0
	}

	payload := map[string]any{
		"file":                      fileBase64,
		"fileType":                  fileType,
		"useDocOrientationClassify": false,
		"useDocUnwarping":           false,
		"useChartRecognition":       false,
		"visualize":                 false, // 不回传可视化图片，显著减小响应体积
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	baidu := config.GetBaidu()
	req, err := http.NewRequestWithContext(ctx, "POST", baidu.ApiUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", baidu.Token))

	// 开启心跳协程，在长耗时请求期间持续反馈进度，防止 UI “假死”
	done := make(chan bool)
	go func(cb ProgressCallback) {
		ticker := time.NewTicker(7 * time.Second)
		defer ticker.Stop()
		messages := []string{
			"正在进行深度语义识别...",
			"正在校对文档布局信息...",
			"正在提取法律实体关联...",
			"云端处理中，请稍候...",
		}
		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if cb != nil {
					// 保持进度在 10% 左右摆动描述，直到 API 返回
					cb(1, 10, messages[i%len(messages)])
					i++
				}
			}
		}
	}(onProgress)

	apiStart := time.Now()
	resp, err := c.httpClient.Do(req)
	close(done) // 停止心跳
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	c.logger.Info("百度 API 响应接收成功", "status", resp.Status, "duration", time.Since(apiStart))

	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{StatusCode: resp.StatusCode}
	}

	var ocrResp BaiduOCRResponse
	// 使用流式解析，处理超大响应，减少内存压力
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		c.logger.Error("JSON 解析失败", "error", err)
		return nil, fmt.Errorf("解析云端数据失败: %w", err)
	}

	if ocrResp.ErrorCode != 0 {
		return nil, fmt.Errorf("百度 API 错误 (%d): %s", ocrResp.ErrorCode, ocrResp.ErrorMsg)
	}

	results := ocrResp.Result.LayoutParsingResults
	if len(results) == 0 {
		c.logger.Warn("百度 API 返回结果为空")
	}
	// dataInfo/prunedResult 为增强字段：形状异常只降级坐标行，不影响 markdown
	var dataInfo baiduDataInfo
	if len(ocrResp.Result.DataInfo) > 0 {
		if err := json.Unmarshal(ocrResp.Result.DataInfo, &dataInfo); err != nil {
			c.logger.Warn("dataInfo 形状异常，页尺寸不可用", "error", err)
			dataInfo = baiduDataInfo{}
		}
	}
	pages := make([]ocrPage, 0, len(results))
	for i, result := range results {
		var pruned baiduPrunedResult
		if len(result.PrunedResult) > 0 {
			if err := json.Unmarshal(result.PrunedResult, &pruned); err != nil {
				c.logger.Warn("prunedResult 形状异常，该页坐标行退化", "pageIndex", i, "error", err)
				pruned = baiduPrunedResult{}
			}
		}
		// 解析该页栅格尺寸：PDF 取 dataInfo.pages[i]；图片取 dataInfo 顶层，
		// 缺失时本地读图片头兜底。尺寸不可得则该页不产出坐标行（前端回退角标）。
		var pageW, pageH float64
		if isPdf {
			if i < len(dataInfo.Pages) {
				pageW, pageH = dataInfo.Pages[i].Width, dataInfo.Pages[i].Height
			}
		} else {
			pageW, pageH = dataInfo.Width, dataInfo.Height
			if pageW <= 0 || pageH <= 0 {
				// 仅 PNG 可安全兜底：JPEG 可能带 EXIF 旋转（本地读到的是
				// 旋转前尺寸，与云端坐标空间可能不一致），宁缺毋错不兜底
				if isPng(fileData) {
					if w, h, ok := imageDims(fileData); ok {
						pageW, pageH = w, h
					}
				}
			}
		}
		ocr := pruned.OverallOcrRes
		lines := buildOcrLines(ocr.RecTexts, ocr.RecBoxes, ocr.RecPolys, pageW, pageH)
		if len(ocr.RecTexts) > 0 && lines == nil {
			c.logger.Warn("该页坐标行不可用，字段高亮将回退角标", "pageIndex", i, "texts", len(ocr.RecTexts), "boxes", len(ocr.RecBoxes), "pageW", pageW, "pageH", pageH)
		}
		pages = append(pages, ocrPage{markdown: result.Markdown.Text, lines: lines})
	}
	return pages, nil
}

// sanitizeChunkPages 校验分块返回页数：与切片页数不符说明云端缺页，
// 全局页序号会整体漂移——抹掉该块坐标行防止高亮画到物理错页
// （markdown 记录保留，与既有行为一致：页码标注风险本就存在）。
func sanitizeChunkPages(pages []ocrPage, expect int, logger *slog.Logger) []ocrPage {
	if len(pages) == expect {
		return pages
	}
	logger.Warn("分块返回页数与切片页数不符，该块坐标行退化以防错位", "expect", expect, "got", len(pages))
	for j := range pages {
		pages[j].lines = nil
	}
	return pages
}

// sleepCtx is time.Sleep that returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
