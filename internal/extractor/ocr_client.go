package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"legal-extractor/internal/config"
	"legal-extractor/internal/gateway"
)

// OcrClient AI Studio PaddleOCR v2 作业式客户端（提交 → 轮询 → 下载 JSONL）。
//
// 双模式，配置在每次调用时经 viper 读取（环境变量修改即刻生效，
// conf.yaml 修改需重启）：
//   - 直连：用户自备 Token（ocr.token）→ 官方作业接口，bearer 鉴权
//   - 网关：应用不携带密钥，请求带机器码/授权头经中继网关换真实 Token 转发
//
// 结果 JSONL 为免鉴权签名 URL，两种模式都直连下载。
type OcrClient struct {
	httpClient   *http.Client
	logger       *slog.Logger
	pollInterval time.Duration // 轮询间隔（测试可缩短）
	retryBackoff time.Duration // 提交/下载重试退避（测试可缩短）
	jobTimeout   time.Duration // 作业总时长上限，防 pending 永久卡死
}

// NewOcrClient 创建云端 OCR 客户端
func NewOcrClient(logger *slog.Logger) *OcrClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &OcrClient{
		httpClient: &http.Client{
			Timeout: 90 * time.Second, // 提交/轮询/下载均为小请求；识别耗时在服务端异步进行
		},
		logger:       logger,
		pollInterval: 5 * time.Second,
		retryBackoff: 5 * time.Second,
		jobTimeout:   15 * time.Minute,
	}
}

// CloudAvailable 云端 OCR 是否可用：用户自备 Token（直连）或已配置网关。
func (c *OcrClient) CloudAvailable() bool {
	ocr := config.GetOcr()
	return ocr.Token != "" || ocr.GatewayUrl != ""
}

// endpoint 解析当前模式：返回作业接口基址与鉴权注入函数。
// 自备 Token 优先（用户用自己的配额，也是网关故障时的自救通道）。
func (c *OcrClient) endpoint() (string, func(*http.Request), error) {
	ocr := config.GetOcr()
	if ocr.Token != "" {
		token := ocr.Token
		return ocr.JobUrl, func(r *http.Request) {
			r.Header.Set("Authorization", "bearer "+token)
		}, nil
	}
	if ocr.GatewayUrl != "" {
		machineID := config.GetMachineID()
		license := config.GetLicenseKey()
		return ocr.GatewayUrl, func(r *http.Request) {
			r.Header.Set(gateway.HeaderMachineID, machineID)
			if license != "" {
				r.Header.Set(gateway.HeaderLicense, license)
			}
		}, nil
	}
	return "", nil, fmt.Errorf("%w: 未配置网关或自备 Token", ErrTokenMissing)
}

// httpStatusError 标记云端返回的非 200 状态码，供重试逻辑做类型化判断。
type httpStatusError struct {
	StatusCode int
	Message    string // 网关/上游返回的可读错误（可为空）
}

func (e *httpStatusError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("云端服务响应异常 (HTTP %d)", e.StatusCode)
}

// isRetryableStatus 仅对云端 5xx 状态错误重试。
func isRetryableStatus(err error) bool {
	var se *httpStatusError
	return errors.As(err, &se) && se.StatusCode >= 500
}

// statusError 组装状态错误：尽力提取响应体中的可读信息
// （网关返回 {"error": …}，携带配额/鉴权等中文文案，需原样透传给用户）。
func statusError(status int, body []byte) *httpStatusError {
	var parsed struct {
		Error    string `json:"error"`
		ErrorMsg string `json:"errorMsg"`
		Msg      string `json:"msg"`
	}
	msg := ""
	if err := json.Unmarshal(body, &parsed); err == nil {
		switch {
		case parsed.Error != "":
			msg = parsed.Error
		case parsed.ErrorMsg != "":
			msg = parsed.ErrorMsg
		case parsed.Msg != "":
			msg = parsed.Msg
		}
	}
	return &httpStatusError{StatusCode: status, Message: msg}
}

// ParseDocument 提交文档到云端识别并聚合结果。
//
// totalPages 由调用方传入（extractPdf 已完成页数探测，图片固定为 1），
// 仅用于进度展示的初始分母；服务端 extractProgress 就绪后以服务端为准。
// 返回结构化记录与逐页归一化坐标文字行（供前端字段高亮）。
func (c *OcrClient) ParseDocument(ctx context.Context, fileData []byte, totalPages int, isPdf bool, onProgress ProgressCallback) (ExtractOutput, error) {
	c.logger.Info("开始云端识别", "isPdf", isPdf, "totalPages", totalPages, "dataSize", len(fileData))
	if len(fileData) == 0 {
		return ExtractOutput{}, ErrEmptyFile
	}
	base, applyAuth, err := c.endpoint()
	if err != nil {
		return ExtractOutput{}, err
	}
	if totalPages < 1 {
		totalPages = 1
	}

	if onProgress != nil {
		onProgress(0, totalPages, "正在上传至云端识别...")
	}
	jobID, err := c.submitJob(ctx, base, applyAuth, fileData, isPdf)
	if err != nil {
		return ExtractOutput{}, err
	}
	c.logger.Info("云端作业已受理", "jobId", jobID)

	jsonURL, err := c.pollJob(ctx, base, applyAuth, jobID, totalPages, onProgress)
	if err != nil {
		return ExtractOutput{}, err
	}

	data, err := c.downloadResult(ctx, jsonURL)
	if err != nil {
		return ExtractOutput{}, err
	}

	pages, err := c.parseJobResult(data, isPdf, fileData)
	if err != nil {
		return ExtractOutput{}, err
	}

	// 聚合：按页解析 Markdown，坐标行按全局页序号归档
	c.logger.Info("云端识别完成，开始按页提取法律实体", "totalFetchedPages", len(pages))
	var allRecords []Record
	var ocrPages []OcrPageLines
	fetched := len(pages)
	for i, pg := range pages {
		if err := ctx.Err(); err != nil {
			return ExtractOutput{}, ErrCancelled
		}
		if onProgress != nil {
			onProgress(i+1, fetched, fmt.Sprintf("正在结构化提取第 %d/%d 页的法律信息...", i+1, fetched))
		}
		for _, rec := range ParseMarkdown(pg.markdown) {
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

// submitJob multipart 提交作业（免 base64 33% 膨胀），5xx 退避重试。
// 网络层错误不重试：连接中断时作业可能已被云端受理，盲目重试会重复扣配额。
func (c *OcrClient) submitJob(ctx context.Context, base string, applyAuth func(*http.Request), fileData []byte, isPdf bool) (string, error) {
	fileName := "document.pdf"
	if !isPdf {
		fileName = "image.png"
	}
	optional, _ := json.Marshal(map[string]any{
		"useDocOrientationClassify": false,
		"useDocUnwarping":           false,
		"useChartRecognition":       false,
	})

	buildBody := func() (*bytes.Buffer, string, error) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := mw.WriteField("model", config.GetOcr().Model); err != nil {
			return nil, "", err
		}
		if err := mw.WriteField("optionalPayload", string(optional)); err != nil {
			return nil, "", err
		}
		fw, err := mw.CreateFormFile("file", fileName)
		if err != nil {
			return nil, "", err
		}
		if _, err := fw.Write(fileData); err != nil {
			return nil, "", err
		}
		if err := mw.Close(); err != nil {
			return nil, "", err
		}
		return &buf, mw.FormDataContentType(), nil
	}

	const maxRetries = 2
	for retry := 0; ; retry++ {
		if retry > 0 {
			c.logger.Warn("作业提交重试", "attempt", retry)
			if err := sleepCtx(ctx, c.retryBackoff); err != nil {
				return "", ErrCancelled
			}
		}

		body, contentType, err := buildBody()
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, body)
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", contentType)
		applyAuth(req)

		jobID, err := c.doSubmit(req)
		if err == nil {
			return jobID, nil
		}
		if ctx.Err() != nil {
			return "", ErrCancelled
		}
		if !isRetryableStatus(err) || retry >= maxRetries {
			return "", err
		}
	}
}

func (c *OcrClient) doSubmit(req *http.Request) (string, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接云端服务失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取云端响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp.StatusCode, body)
	}
	var parsed struct {
		Data struct {
			JobID string `json:"jobId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Data.JobID == "" {
		return "", fmt.Errorf("云端响应缺少作业标识")
	}
	return parsed.Data.JobID, nil
}

// jobStatus 轮询响应的关注字段
type jobStatus struct {
	Data struct {
		State           string `json:"state"`
		ErrorMsg        string `json:"errorMsg"`
		ExtractProgress struct {
			TotalPages     int `json:"totalPages"`
			ExtractedPages int `json:"extractedPages"`
		} `json:"extractProgress"`
		ResultURL struct {
			JSONURL string `json:"jsonUrl"`
		} `json:"resultUrl"`
	} `json:"data"`
}

// pollJob 轮询作业状态直至 done/failed/超时；服务端真实页级进度上报 UI。
// 连续 3 次瞬时错误（网络/5xx）判失败；4xx（配额/归属/鉴权）立即失败。
func (c *OcrClient) pollJob(ctx context.Context, base string, applyAuth func(*http.Request), jobID string, totalPages int, onProgress ProgressCallback) (string, error) {
	deadline := time.Now().Add(c.jobTimeout)
	transient := 0
	seenRunning := false // 已进入 running 后，未知/pending 态不再把进度打回 0
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("云端识别超时（超过 %d 分钟），请稍后重试", int(c.jobTimeout.Minutes()))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+jobID, nil)
		if err != nil {
			return "", err
		}
		applyAuth(req)

		st, err := c.doPoll(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ErrCancelled
			}
			if isRetryableStatus(err) || isNetworkErr(err) {
				transient++
				if transient >= 3 {
					return "", fmt.Errorf("云端状态查询连续失败: %w", err)
				}
			} else {
				return "", err // 4xx：配额/归属/鉴权，重试无意义
			}
		} else {
			transient = 0
			switch st.Data.State {
			case "done":
				if st.Data.ResultURL.JSONURL == "" {
					return "", fmt.Errorf("云端作业完成但缺少结果地址")
				}
				return st.Data.ResultURL.JSONURL, nil
			case "failed":
				msg := st.Data.ErrorMsg
				if msg == "" {
					msg = "未知原因"
				}
				return "", fmt.Errorf("云端识别失败: %s", msg)
			case "running":
				seenRunning = true
				if onProgress != nil {
					total := st.Data.ExtractProgress.TotalPages
					if total <= 0 {
						total = totalPages
					}
					onProgress(st.Data.ExtractProgress.ExtractedPages, total, "云端识别中...")
				}
			default: // pending 或未知状态：继续等待（已见 running 后不回退进度）
				if onProgress != nil && !seenRunning {
					onProgress(0, totalPages, "云端排队中，请稍候...")
				}
			}
		}

		if err := sleepCtx(ctx, c.pollInterval); err != nil {
			return "", ErrCancelled
		}
	}
}

func (c *OcrClient) doPoll(req *http.Request) (*jobStatus, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &networkError{cause: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, &networkError{cause: err}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, body)
	}
	var st jobStatus
	if err := json.Unmarshal(body, &st); err != nil {
		return nil, &networkError{cause: fmt.Errorf("解析状态响应失败: %w", err)}
	}
	return &st, nil
}

// networkError 标记网络层瞬时错误（连接/读取/响应形状），轮询时可容忍。
type networkError struct{ cause error }

func (e *networkError) Error() string { return e.cause.Error() }
func (e *networkError) Unwrap() error { return e.cause }

func isNetworkErr(err error) bool {
	var ne *networkError
	return errors.As(err, &ne)
}

// maxResultBytes 结果 JSONL 大小上限；超限显式报错而非静默截断丢页。
const maxResultBytes = 64 << 20

// downloadResult 下载结果 JSONL（免鉴权签名 URL，两种模式都直连）。
func (c *OcrClient) downloadResult(ctx context.Context, jsonURL string) ([]byte, error) {
	const maxRetries = 2
	for retry := 0; ; retry++ {
		if retry > 0 {
			if err := sleepCtx(ctx, c.retryBackoff); err != nil {
				return nil, ErrCancelled
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResultBytes+1))
			_ = resp.Body.Close()
			if readErr == nil {
				if len(data) > maxResultBytes {
					return nil, fmt.Errorf("识别结果过大（超过 %d MB），暂不支持", maxResultBytes>>20)
				}
				return data, nil
			}
			err = readErr
		} else if err == nil {
			_ = resp.Body.Close()
			err = &httpStatusError{StatusCode: resp.StatusCode}
		}
		if ctx.Err() != nil {
			return nil, ErrCancelled
		}
		if retry >= maxRetries {
			return nil, fmt.Errorf("下载识别结果失败: %w", err)
		}
		c.logger.Warn("结果下载重试", "attempt", retry+1, "error", err)
	}
}

// v2 JSONL 行结构：{"result": {"dataInfo": …, "layoutParsingResults": […]}}
type jobResultLine struct {
	Result struct {
		DataInfo             json.RawMessage `json:"dataInfo"`
		LayoutParsingResults []struct {
			Markdown struct {
				Text string `json:"text"`
			} `json:"markdown"`
			PrunedResult json.RawMessage `json:"prunedResult"`
		} `json:"layoutParsingResults"`
	} `json:"result"`
}

// v2PrunedResult PaddleOCR-VL 的版面解析结果：块级坐标（无行级 overall_ocr_res）。
// 尺寸随结果自带（与 block_bbox 同一像素空间），无需跨行对 dataInfo 做下标映射。
type v2PrunedResult struct {
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	ParsingResList []struct {
		BlockLabel   string    `json:"block_label"`
		BlockContent string    `json:"block_content"`
		BlockBbox    []float64 `json:"block_bbox"`
	} `json:"parsing_res_list"`
}

// baiduDataInfo 输入数据信息：图片为顶层宽高，PDF 为逐页栅格尺寸。
// 作为 prunedResult 自带尺寸缺失时的兜底。
type baiduDataInfo struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Pages  []struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"pages"`
}

// ocrPage 单页产物：markdown 用于字段提取，lines 为归一化坐标文字行。
type ocrPage struct {
	markdown string
	lines    []OcrLine
}

// parseJobResult 逐行逐 result 聚合为全局页序。增强字段（prunedResult/dataInfo）
// 为 best-effort：形状异常只降级该页坐标行，markdown 主流程不受影响。
func (c *OcrClient) parseJobResult(data []byte, isPdf bool, fileData []byte) ([]ocrPage, error) {
	var pages []ocrPage
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var parsed jobResultLine
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			return nil, fmt.Errorf("解析识别结果失败: %w", err)
		}

		var dataInfo baiduDataInfo
		if len(parsed.Result.DataInfo) > 0 {
			if err := json.Unmarshal(parsed.Result.DataInfo, &dataInfo); err != nil {
				c.logger.Warn("dataInfo 形状异常，页尺寸兜底不可用", "error", err)
				dataInfo = baiduDataInfo{}
			}
		}

		for i, result := range parsed.Result.LayoutParsingResults {
			var pruned v2PrunedResult
			if len(result.PrunedResult) > 0 {
				if err := json.Unmarshal(result.PrunedResult, &pruned); err != nil {
					c.logger.Warn("prunedResult 形状异常，该页坐标行退化", "pageIndex", len(pages), "error", err)
					pruned = v2PrunedResult{}
				}
			}

			// 页尺寸：优先结果自带 → dataInfo 兜底（仅当行内数量自洽，
			// 防止跨行 JSONL 携带全文档页表时下标错位画错位置）→ 图片 PNG 本地读头
			pageW, pageH := pruned.Width, pruned.Height
			if pageW <= 0 || pageH <= 0 {
				if isPdf {
					if len(dataInfo.Pages) == len(parsed.Result.LayoutParsingResults) && i < len(dataInfo.Pages) {
						pageW, pageH = dataInfo.Pages[i].Width, dataInfo.Pages[i].Height
					}
				} else {
					pageW, pageH = dataInfo.Width, dataInfo.Height
					if (pageW <= 0 || pageH <= 0) && isPng(fileData) {
						// 仅 PNG 兜底：JPEG 可能带 EXIF 旋转，本地尺寸基准不可信
						if w, h, ok := imageDims(fileData); ok {
							pageW, pageH = w, h
						}
					}
				}
			}

			// 块级坐标 → 文字行（表格块内容为 HTML，匹配自然失败无害）
			var texts []string
			var boxes [][]float64
			for _, b := range pruned.ParsingResList {
				if strings.TrimSpace(b.BlockContent) == "" || len(b.BlockBbox) < 4 {
					continue
				}
				texts = append(texts, b.BlockContent)
				boxes = append(boxes, b.BlockBbox)
			}
			lines := buildOcrLines(texts, boxes, nil, pageW, pageH)
			if len(texts) > 0 && lines == nil {
				c.logger.Warn("该页坐标行不可用，字段高亮将回退角标", "pageIndex", len(pages), "blocks", len(texts), "pageW", pageW, "pageH", pageH)
			}
			pages = append(pages, ocrPage{markdown: result.Markdown.Text, lines: lines})
		}
	}
	if len(pages) == 0 {
		c.logger.Warn("云端返回结果为空")
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
