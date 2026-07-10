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

// BaiduOCRResponse 百度 Layout Parsing 响应结构
type BaiduOCRResponse struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	Result    struct {
		LayoutParsingResults []struct {
			Markdown struct {
				Text string `json:"text"`
			} `json:"markdown"`
		} `json:"layoutParsingResults"`
	} `json:"result"`
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
func (c *BaiduClient) ParseDocument(ctx context.Context, fileData []byte, totalPages int, isPdf bool, onProgress ProgressCallback) ([]Record, error) {
	c.logger.Info("开始调用百度 OCR 接口", "isPdf", isPdf, "totalPages", totalPages, "dataSize", len(fileData))
	if len(fileData) == 0 {
		return nil, ErrEmptyFile
	}
	if !c.HasToken() {
		return nil, fmt.Errorf("%w: 请检查 config/conf.yaml", ErrTokenMissing)
	}
	if totalPages < 1 {
		totalPages = 1
	}

	// 1. 获取所有页面的 Markdown（超过阈值的 PDF 物理分块，防止云端超时）
	const maxPagesPerChunk = 20 // 小分块粒度显著提升云端解析稳定性

	var allPagesMarkdown []string
	switch {
	case isPdf && totalPages > maxPagesPerChunk:
		c.logger.Info("启用大文件物理分块处理模式", "chunkSize", maxPagesPerChunk)
		for start := 1; start <= totalPages; start += maxPagesPerChunk {
			end := min(start+maxPagesPerChunk-1, totalPages)
			pages, err := c.parseChunkWithRetry(ctx, fileData, start, end, totalPages, onProgress)
			if err != nil {
				return nil, err
			}
			allPagesMarkdown = append(allPagesMarkdown, pages...)

			// 强制冷却，防止连续高压导致百度后端崩溃
			if end < totalPages {
				c.logger.Info("分块处理完成，进入 10 秒冷却期以释放云端算力...")
				if err := sleepCtx(ctx, 10*time.Second); err != nil {
					return nil, ErrCancelled
				}
			}
		}
	case isPdf:
		if onProgress != nil {
			onProgress(1, totalPages, "正在进行深度识别与内容校对，请稍候...")
		}
		pages, err := c.callBaiduAPI(ctx, fileData, true, onProgress)
		if err != nil {
			return nil, err
		}
		allPagesMarkdown = pages
	default: // 图片
		if onProgress != nil {
			onProgress(1, 1, "正在对图片进行语义化识别...")
		}
		pages, err := c.callBaiduAPI(ctx, fileData, false, onProgress)
		if err != nil {
			return nil, err
		}
		allPagesMarkdown = pages
	}

	// 2. 按页解析汇总后的 Markdown
	c.logger.Info("所有页面识别完成，开始按页提取法律实体", "totalFetchedPages", len(allPagesMarkdown))
	var allRecords []Record
	fetched := len(allPagesMarkdown)
	for i, pageMd := range allPagesMarkdown {
		if err := ctx.Err(); err != nil {
			return nil, ErrCancelled
		}
		if onProgress != nil {
			onProgress(i+1, fetched, fmt.Sprintf("正在结构化提取第 %d/%d 页的法律信息...", i+1, fetched))
		}
		for _, rec := range ParseMarkdown(pageMd) {
			// 标注准确的页码
			if rec["page"] == "" {
				rec["page"] = fmt.Sprintf("%d", i+1)
			}
			allRecords = append(allRecords, rec)
		}
	}

	c.logger.Info("数据提取完成", "recordCount", len(allRecords))
	return allRecords, nil
}

// parseChunkWithRetry 物理切出 [start, end] 页并调用云端识别，
// 云端 5xx 时执行“避让重试”（最多 2 次，间隔 20 秒）。
func (c *BaiduClient) parseChunkWithRetry(ctx context.Context, fileData []byte, start, end, totalPages int, onProgress ProgressCallback) ([]string, error) {
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

// callBaiduAPI 封装底层的 API 调用逻辑
func (c *BaiduClient) callBaiduAPI(ctx context.Context, fileData []byte, isPdf bool, onProgress ProgressCallback) ([]string, error) {
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

	var pages []string
	if len(ocrResp.Result.LayoutParsingResults) == 0 {
		c.logger.Warn("百度 API 返回结果为空")
	}
	for _, result := range ocrResp.Result.LayoutParsingResults {
		pages = append(pages, result.Markdown.Text)
	}
	return pages, nil
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
