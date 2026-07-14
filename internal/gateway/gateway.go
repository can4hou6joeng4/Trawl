// Package gateway 实现 OCR 中继网关：应用不携带云端密钥，请求经网关校验
// 机器码/授权并换上真实 Token 后转发 AI Studio v2 作业式接口。
//
// 设计要点（见 docs/superpowers/specs/2026-07-14-ocr-v2-jobs-and-builtin-token-prd.md §6.5）：
//   - 无状态转发 + 内存计量（单实例）；结果 JSONL 为免鉴权签名 URL，客户端直连不过网关
//   - 授权校验与应用端同构：HMAC-SHA256(machineID, LicenseSecret) 前 16 位十六进制
//   - 试用档（无/无效授权码）与授权档分别限每日提交数；轮询不计提交配额但受频率限制
//   - 作业归属：提交时记录 jobId→machineID，轮询校验归属，防跨机器窥探
package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// 客户端鉴权头（与 internal/extractor 的网关模式约定一致）
const (
	HeaderMachineID = "X-LE-Machine-ID"
	HeaderLicense   = "X-LE-License"
)

// Config 网关配置，全部经环境变量注入（见 cmd/ocr-gateway）。
type Config struct {
	UpstreamJobURL string        // 上游作业接口（默认官方端点）
	UpstreamToken  string        // 真实 AI Studio Token（必填，永不下发）
	LicenseSecret  string        // 与应用构建注入的 LicenseSecret 相同；为空则所有调用按试用档
	TrialDaily     int           // 试用档每日提交上限
	LicensedDaily  int           // 授权档每日提交上限
	PerMinute      int           // 单机器码每分钟请求上限（提交+轮询）
	MaxUploadBytes int64         // 单次提交体积上限
	Timeout        time.Duration // 上游请求超时
}

// 默认值：与 UI 宣称的单文件 ≤50MB 对齐，轮询 5s 间隔下 120/min 足够并行多作业
func (c *Config) fillDefaults() {
	if c.UpstreamJobURL == "" {
		c.UpstreamJobURL = "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs"
	}
	if c.TrialDaily <= 0 {
		c.TrialDaily = 20
	}
	if c.LicensedDaily <= 0 {
		c.LicensedDaily = 200
	}
	if c.PerMinute <= 0 {
		c.PerMinute = 120
	}
	if c.MaxUploadBytes <= 0 {
		c.MaxUploadBytes = 60 << 20
	}
	if c.Timeout <= 0 {
		c.Timeout = 90 * time.Second
	}
}

var (
	machineIDPattern = regexp.MustCompile(`^[0-9A-F]{8}$`)
	jobIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`) // 白名单：杜绝路径/查询注入
)

// Gateway 是可测试的 http.Handler。
type Gateway struct {
	cfg    Config
	logger *slog.Logger
	client *http.Client
	now    func() time.Time // 可注入时钟（测试跨日/限频）

	mu        sync.Mutex
	submits   map[string]*windowCounter // machineID → 当日提交数
	requests  map[string]*windowCounter // machineID → 当分钟请求数
	jobOwners map[string]jobOwner       // jobID → 归属
	sweptReq  string                    // requests 上次清扫的窗口（每窗口至多清扫一次）
	sweptSub  string                    // submits 同上
}

type windowCounter struct {
	window string // 日窗口 "2026-07-14" 或分钟窗口 "2026-07-14T18:00"
	count  int
}

type jobOwner struct {
	machineID string
	createdAt time.Time
}

// New 构造网关。cfg.UpstreamToken 必填。
func New(cfg Config, logger *slog.Logger) (*Gateway, error) {
	cfg.fillDefaults()
	if cfg.UpstreamToken == "" {
		return nil, fmt.Errorf("缺少上游 Token（OCR_GW_TOKEN）")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Gateway{
		cfg:       cfg,
		logger:    logger,
		client:    &http.Client{Timeout: cfg.Timeout},
		now:       time.Now,
		submits:   make(map[string]*windowCounter),
		requests:  make(map[string]*windowCounter),
		jobOwners: make(map[string]jobOwner),
	}, nil
}

// ServeHTTP 路由：POST /api/v2/ocr/jobs、GET /api/v2/ocr/jobs/{id}、GET /healthz。
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	const jobsPrefix = "/api/v2/ocr/jobs"
	if !strings.HasPrefix(r.URL.Path, jobsPrefix) {
		g.reply(w, http.StatusNotFound, "未知路径")
		return
	}

	machineID, tier, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	if !g.allowRequest(machineID) {
		g.logger.Warn("限频拒绝", "machine", machineID)
		g.reply(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, jobsPrefix)
	switch {
	case r.Method == http.MethodPost && (rest == "" || rest == "/"):
		g.handleSubmit(w, r, machineID, tier)
	case r.Method == http.MethodGet && strings.HasPrefix(rest, "/"):
		g.handlePoll(w, r, machineID, strings.Trim(rest, "/"))
	default:
		g.reply(w, http.StatusMethodNotAllowed, "不支持的方法")
	}
}

// authenticate 校验机器码与授权码，返回 (machineID, 档位)。失败时已写响应。
func (g *Gateway) authenticate(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	machineID := strings.ToUpper(strings.TrimSpace(r.Header.Get(HeaderMachineID)))
	if !machineIDPattern.MatchString(machineID) {
		g.reply(w, http.StatusUnauthorized, "缺少或非法的机器标识")
		return "", "", false
	}
	tier := "trial"
	if license := strings.TrimSpace(r.Header.Get(HeaderLicense)); license != "" && g.verifyLicense(machineID, license) {
		tier = "licensed"
	}
	return machineID, tier, true
}

// verifyLicense 与应用端 config.GenerateLicense 同构；常数时间比较防时序侧信道。
func (g *Gateway) verifyLicense(machineID, licenseCode string) bool {
	if g.cfg.LicenseSecret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(g.cfg.LicenseSecret))
	mac.Write([]byte(machineID))
	raw := fmt.Sprintf("%x", mac.Sum(nil))
	code := strings.ToUpper(raw[:16])
	expected := fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
	given := strings.ToUpper(strings.TrimSpace(licenseCode))
	return subtle.ConstantTimeCompare([]byte(given), []byte(expected)) == 1
}

// maxTrackedMachines 计量 map 的机器数上限触发阈值：超过即清扫过期窗口条目，
// 防止公网伪造海量机器码撑爆内存（正常用户量远低于此）。
const maxTrackedMachines = 10000

// allowRequest 分钟级限频（提交+轮询共用）。
func (g *Gateway) allowRequest(machineID string) bool {
	window := g.now().Format("2006-01-02T15:04")
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.requests) > maxTrackedMachines && g.sweptReq != window {
		sweepStale(g.requests, window)
		g.sweptReq = window
	}
	c := g.requests[machineID]
	if c == nil || c.window != window {
		c = &windowCounter{window: window}
		g.requests[machineID] = c
	}
	if c.count >= g.cfg.PerMinute {
		return false
	}
	c.count++
	return true
}

// allowSubmit 日级提交配额，按档位取限。
func (g *Gateway) allowSubmit(machineID, tier string) bool {
	limit := g.cfg.TrialDaily
	if tier == "licensed" {
		limit = g.cfg.LicensedDaily
	}
	window := g.now().Format("2006-01-02")
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.submits) > maxTrackedMachines && g.sweptSub != window {
		sweepStale(g.submits, window)
		g.sweptSub = window
	}
	c := g.submits[machineID]
	if c == nil || c.window != window {
		c = &windowCounter{window: window}
		g.submits[machineID] = c
	}
	if c.count >= limit {
		return false
	}
	c.count++
	return true
}

// sweepStale 清除窗口已过期的计数条目（调用方须持锁）。
func sweepStale(m map[string]*windowCounter, current string) {
	for k, c := range m {
		if c.window != current {
			delete(m, k)
		}
	}
}

// handleSubmit 转发作业提交，成功后记录作业归属。
func (g *Gateway) handleSubmit(w http.ResponseWriter, r *http.Request, machineID, tier string) {
	if !g.allowSubmit(machineID, tier) {
		g.logger.Warn("配额拒绝", "machine", machineID, "tier", tier)
		g.reply(w, http.StatusForbidden, "今日识别次数已达上限，请明日再试或联系开发者")
		return
	}

	start := g.now()
	body := http.MaxBytesReader(w, r.Body, g.cfg.MaxUploadBytes)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, g.cfg.UpstreamJobURL, body)
	if err != nil {
		g.reply(w, http.StatusInternalServerError, "构造上游请求失败")
		return
	}
	// multipart 边界在 Content-Type 里，必须透传
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	req.Header.Set("Authorization", "bearer "+g.cfg.UpstreamToken)

	resp, err := g.client.Do(req)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			g.reply(w, http.StatusRequestEntityTooLarge, "文件过大，超出网关限制")
			return
		}
		g.logger.Error("上游提交失败", "machine", machineID, "error", err)
		g.reply(w, http.StatusBadGateway, "云端服务暂不可用，请稍后再试")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// 提交响应是小 JSON：读全以便登记作业归属，再原样回写
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		g.reply(w, http.StatusBadGateway, "读取云端响应失败")
		return
	}
	if resp.StatusCode == http.StatusOK {
		if jobID := extractJobID(respBody); jobID != "" {
			g.recordJob(jobID, machineID)
		}
	}
	g.logger.Info("提交转发", "machine", machineID, "tier", tier, "status", resp.StatusCode, "ms", g.now().Sub(start).Milliseconds())
	passthrough(w, resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// handlePoll 校验作业归属后转发轮询。
func (g *Gateway) handlePoll(w http.ResponseWriter, r *http.Request, machineID, jobID string) {
	if !jobIDPattern.MatchString(jobID) {
		g.reply(w, http.StatusNotFound, "作业不存在")
		return
	}
	if owner, ok := g.jobOwner(jobID); !ok || owner != machineID {
		g.reply(w, http.StatusNotFound, "作业不存在或无权访问")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, g.cfg.UpstreamJobURL+"/"+jobID, nil)
	if err != nil {
		g.reply(w, http.StatusInternalServerError, "构造上游请求失败")
		return
	}
	req.Header.Set("Authorization", "bearer "+g.cfg.UpstreamToken)

	resp, err := g.client.Do(req)
	if err != nil {
		g.logger.Error("上游轮询失败", "machine", machineID, "job", jobID, "error", err)
		g.reply(w, http.StatusBadGateway, "云端服务暂不可用，请稍后再试")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		g.reply(w, http.StatusBadGateway, "读取云端响应失败")
		return
	}
	passthrough(w, resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// recordJob 登记作业归属，顺带清理 24h 前的旧条目（提交频度低，线性清理足够）。
func (g *Gateway) recordJob(jobID, machineID string) {
	cutoff := g.now().Add(-24 * time.Hour)
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, o := range g.jobOwners {
		if o.createdAt.Before(cutoff) {
			delete(g.jobOwners, id)
		}
	}
	g.jobOwners[jobID] = jobOwner{machineID: machineID, createdAt: g.now()}
}

func (g *Gateway) jobOwner(jobID string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	o, ok := g.jobOwners[jobID]
	return o.machineID, ok
}

// extractJobID 从上游提交响应中取 data.jobId。
func extractJobID(body []byte) string {
	var parsed struct {
		Data struct {
			JobID string `json:"jobId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return parsed.Data.JobID
}

func passthrough(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (g *Gateway) reply(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
