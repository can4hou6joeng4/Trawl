package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"legal-extractor/internal/config"
	"legal-extractor/internal/gateway"
)

// ─── 测试基建：假作业服务 ───

// mockJobServer 模拟 v2 作业式服务：提交 → 状态序列 → JSONL 结果。
type mockJobServer struct {
	t *testing.T

	mu          sync.Mutex
	submits     []submitRecord
	polls       int
	states      []string                  // 依次返回的状态序列，耗尽后停在最后一个
	failMsg     string                    // state=failed 时的 errorMsg
	jsonl       string                    // 结果内容
	submitCodes []int                     // 依次返回的提交状态码（默认全 200）
	pollHook    func(n int) (int, string) // 可选：第 n 次轮询返回 (状态码, 响应体)；返回 0 走默认
}

type submitRecord struct {
	auth      string
	machineID string
	license   string
	model     string
	optional  string
	fileName  string
	fileBytes []byte
}

func (m *mockJobServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/jobs"):
			m.handleSubmit(w, r)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/jobs/"):
			m.handlePoll(w, r)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/result.jsonl"):
			_, _ = w.Write([]byte(m.jsonl))
		default:
			http.NotFound(w, r)
		}
	}
}

func (m *mockJobServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	idx := len(m.submits)
	code := http.StatusOK
	if idx < len(m.submitCodes) {
		code = m.submitCodes[idx]
	}
	m.mu.Unlock()

	rec := submitRecord{
		auth:      r.Header.Get("Authorization"),
		machineID: r.Header.Get(gateway.HeaderMachineID),
		license:   r.Header.Get(gateway.HeaderLicense),
	}
	if mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && strings.HasPrefix(mt, "multipart/") {
		if err := r.ParseMultipartForm(64 << 20); err == nil {
			rec.model = r.FormValue("model")
			rec.optional = r.FormValue("optionalPayload")
			if f, fh, err := r.FormFile("file"); err == nil {
				rec.fileName = fh.Filename
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(f)
				rec.fileBytes = buf.Bytes()
				_ = f.Close()
			}
		}
		_ = params
	}
	m.mu.Lock()
	m.submits = append(m.submits, rec)
	m.mu.Unlock()

	if code != http.StatusOK {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":"云端暂不可用"}`))
		return
	}
	_, _ = w.Write([]byte(`{"data":{"jobId":"job-test-1"}}`))
}

func (m *mockJobServer) handlePoll(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	n := m.polls
	m.polls++
	state := "done"
	if len(m.states) > 0 {
		if n < len(m.states) {
			state = m.states[n]
		} else {
			state = m.states[len(m.states)-1]
		}
	}
	hook := m.pollHook
	failMsg := m.failMsg
	m.mu.Unlock()

	if hook != nil {
		if code, body := hook(n); code != 0 {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(body))
			return
		}
	}

	base := "http://" + r.Host
	switch state {
	case "done":
		_, _ = fmt.Fprintf(w, `{"data":{"state":"done","extractProgress":{"totalPages":2,"extractedPages":2},"resultUrl":{"jsonUrl":"%s/result.jsonl"}}}`, base)
	case "failed":
		_, _ = fmt.Fprintf(w, `{"data":{"state":"failed","errorMsg":%q}}`, failMsg)
	case "running":
		_, _ = w.Write([]byte(`{"data":{"state":"running","extractProgress":{"totalPages":2,"extractedPages":1}}}`))
	default:
		_, _ = fmt.Fprintf(w, `{"data":{"state":%q}}`, state)
	}
}

// jsonlFixture 按 2026-07-14 真实探针 schema 拼一页结果
func jsonlFixture(markdown string, blocks []map[string]any, pageW, pageH float64) string {
	pruned := map[string]any{"width": pageW, "height": pageH, "parsing_res_list": blocks}
	line := map[string]any{"result": map[string]any{
		"dataInfo":             map[string]any{"numPages": 1, "type": "pdf", "pages": []map[string]any{{"width": pageW, "height": pageH}}},
		"layoutParsingResults": []map[string]any{{"markdown": map[string]any{"text": markdown}, "prunedResult": pruned}},
	}}
	b, _ := json.Marshal(line)
	return string(b)
}

// withOcrEnv 配置直连/网关模式并在测试后还原无云端状态。
// 先清空全部相关键，隔离开发者 shell 里可能预设的环境变量。
func withOcrEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{
		"LEGAL_EXTRACTOR_OCR_TOKEN", "LEGAL_EXTRACTOR_OCR_JOB_URL",
		"LEGAL_EXTRACTOR_OCR_GATEWAY_URL", "LEGAL_EXTRACTOR_OCR_MODEL",
		"LEGAL_EXTRACTOR_BAIDU_TOKEN",
	} {
		t.Setenv(k, "") // 注册还原
		_ = os.Unsetenv(k)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	if err := config.Init(""); err != nil {
		t.Fatalf("config.Init: %v", err)
	}
	t.Cleanup(func() {
		for k := range env {
			_ = os.Unsetenv(k)
		}
		_ = config.Init("")
	})
}

func newTestClient() *OcrClient {
	c := NewOcrClient(nil)
	c.pollInterval = 5 * time.Millisecond
	c.retryBackoff = 5 * time.Millisecond
	c.jobTimeout = 5 * time.Second
	return c
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// ─── 用例 ───

func TestOcrClient_DirectMode_FullFlow(t *testing.T) {
	srv := &mockJobServer{t: t, states: []string{"pending", "running", "done"}}
	blocks := []map[string]any{
		{"block_label": "text", "block_content": "被告：张三，男，汉族。", "block_bbox": []float64{100, 200, 500, 250}},
		{"block_label": "text", "block_content": "", "block_bbox": []float64{0, 0, 10, 10}}, // 空块跳过
		{"block_label": "text", "block_content": "身份证号码：110101199001011234。", "block_bbox": []float64{100, 300, 700, 350}},
	}
	srv.jsonl = jsonlFixture("# 民事起诉状\n被告：张三，身份证号码：110101199001011234", blocks, 1000, 1400)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "user-token",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	var progress []string
	out, err := newTestClient().ParseDocument(context.Background(), []byte("fake-pdf"), 2, true,
		func(cur, total int, msg string) {
			progress = append(progress, fmt.Sprintf("%d/%d %s", cur, total, msg))
		})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	// 提交侧契约
	sub := srv.submits[0]
	if sub.auth != "bearer user-token" {
		t.Errorf("直连模式鉴权头错误: %q", sub.auth)
	}
	if sub.model != "PaddleOCR-VL-1.6" {
		t.Errorf("model 参数错误: %q", sub.model)
	}
	if !strings.Contains(sub.optional, "useChartRecognition") {
		t.Errorf("optionalPayload 缺失: %q", sub.optional)
	}
	if string(sub.fileBytes) != "fake-pdf" || sub.fileName != "document.pdf" {
		t.Errorf("multipart 文件错误: %q %q", sub.fileName, sub.fileBytes)
	}

	// 记录与坐标
	if len(out.Records) == 0 || out.Records[0]["defendant"] != "张三" {
		t.Fatalf("markdown 记录提取失败: %+v", out.Records)
	}
	if len(out.OcrPages) != 1 || len(out.OcrPages[0].Lines) != 2 {
		t.Fatalf("坐标行数量错误: %+v", out.OcrPages)
	}
	l := out.OcrPages[0].Lines[0]
	if !approx(l.X, 0.1) || !approx(l.Y, 200.0/1400) || !approx(l.W, 0.4) || !approx(l.H, 50.0/1400) {
		t.Errorf("块坐标归一化错误: %+v", l)
	}

	// 进度：上传 → 排队 → 服务端页级进度
	joined := strings.Join(progress, " | ")
	for _, want := range []string{"正在上传至云端识别", "云端排队中", "1/2 云端识别中"} {
		if !strings.Contains(joined, want) {
			t.Errorf("进度缺少 %q，got %s", want, joined)
		}
	}
}

func TestOcrClient_GatewayMode_Headers(t *testing.T) {
	srv := &mockJobServer{t: t}
	srv.jsonl = jsonlFixture("被告：李四", nil, 1000, 1000)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_GATEWAY_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	if _, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil); err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	sub := srv.submits[0]
	if sub.auth != "" {
		t.Errorf("网关模式不应携带 bearer 头: %q", sub.auth)
	}
	if sub.machineID != config.GetMachineID() {
		t.Errorf("机器码头错误: %q", sub.machineID)
	}
}

func TestOcrClient_UserTokenPreferredOverGateway(t *testing.T) {
	srv := &mockJobServer{t: t}
	srv.jsonl = jsonlFixture("被告：王五", nil, 1000, 1000)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":       "my-own-token",
		"LEGAL_EXTRACTOR_OCR_JOB_URL":     ts.URL + "/api/v2/ocr/jobs",
		"LEGAL_EXTRACTOR_OCR_GATEWAY_URL": "http://should-not-be-used.invalid",
	})

	if _, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil); err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if srv.submits[0].auth != "bearer my-own-token" {
		t.Error("自备 Token 应优先于网关直连")
	}
}

func TestOcrClient_LegacyBaiduTokenMigrated(t *testing.T) {
	srv := &mockJobServer{t: t}
	srv.jsonl = jsonlFixture("被告：老配置", nil, 1000, 1000)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_BAIDU_TOKEN": "legacy-token",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	if _, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil); err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if srv.submits[0].auth != "bearer legacy-token" {
		t.Error("旧 baidu.token 应迁移生效")
	}
}

func TestOcrClient_JobFailed(t *testing.T) {
	srv := &mockJobServer{t: t, states: []string{"running", "failed"}, failMsg: "页数超出限制"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	_, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err == nil || !strings.Contains(err.Error(), "页数超出限制") {
		t.Fatalf("failed 态应透传 errorMsg，got %v", err)
	}
}

func TestOcrClient_CancelDuringPoll(t *testing.T) {
	srv := &mockJobServer{t: t, states: []string{"running"}} // 永远 running
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	_, err := newTestClient().ParseDocument(ctx, []byte("x"), 1, true, nil)
	if err == nil || !strings.Contains(err.Error(), ErrCancelled.Error()) {
		t.Fatalf("取消应返回 ErrCancelled，got %v", err)
	}
}

func TestOcrClient_JobTimeout(t *testing.T) {
	srv := &mockJobServer{t: t, states: []string{"pending"}} // 永远 pending
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	c := newTestClient()
	c.jobTimeout = 50 * time.Millisecond
	_, err := c.ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("卡死作业应超时，got %v", err)
	}
}

func TestOcrClient_SubmitRetryOn5xx(t *testing.T) {
	srv := &mockJobServer{t: t, submitCodes: []int{503, 200}}
	srv.jsonl = jsonlFixture("被告：重试成功", nil, 1000, 1000)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	c := newTestClient()
	out, err := c.ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err != nil {
		t.Fatalf("5xx 后重试应成功: %v", err)
	}
	if len(srv.submits) != 2 {
		t.Fatalf("应提交 2 次，got %d", len(srv.submits))
	}
	if len(out.Records) == 0 {
		t.Fatal("重试成功后应有记录")
	}
}

func TestOcrClient_Gateway4xxSurfacedVerbatim(t *testing.T) {
	srv := &mockJobServer{t: t, submitCodes: []int{403}}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_GATEWAY_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	_, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err == nil || !strings.Contains(err.Error(), "云端暂不可用") {
		t.Fatalf("网关 4xx 文案应原样透传且不重试，got %v（提交次数 %d）", err, len(srv.submits))
	}
	if len(srv.submits) != 1 {
		t.Fatalf("4xx 不应重试，got %d 次提交", len(srv.submits))
	}
}

func TestOcrClient_TransientPollErrorsTolerated(t *testing.T) {
	srv := &mockJobServer{t: t, states: []string{"running", "running", "done"}}
	srv.jsonl = jsonlFixture("被告：容错", nil, 1000, 1000)
	srv.pollHook = func(n int) (int, string) {
		if n == 1 {
			return 502, `{"error":"blip"}`
		}
		return 0, ""
	}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	if _, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil); err != nil {
		t.Fatalf("单次 5xx 轮询错误应被容忍: %v", err)
	}
}

func TestOcrClient_DefaultGatewayBaked(t *testing.T) {
	// 发布默认内置官方网关：纯默认配置下云端恒可用（用户零配置）
	withOcrEnv(t, map[string]string{})
	c := newTestClient()
	if !c.CloudAvailable() {
		t.Fatal("默认配置应内置网关，云端恒可用")
	}
	if got := config.GetOcr().GatewayUrl; !strings.Contains(got, "ocr-gw.bobochang.cn") {
		t.Fatalf("默认网关地址错误: %q", got)
	}
}

func TestOcrClient_UnconfiguredReturnsTokenMissing(t *testing.T) {
	// 配置未初始化（开发裸跑/异常路径）→ ErrTokenMissing 哨兵
	config.ResetForTest()
	t.Cleanup(func() { _ = config.Init("") })

	c := newTestClient()
	if c.CloudAvailable() {
		t.Fatal("配置未初始化时 CloudAvailable 应为 false")
	}
	_, err := c.ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err == nil || !strings.Contains(err.Error(), ErrTokenMissing.Error()) {
		t.Fatalf("应返回 ErrTokenMissing，got %v", err)
	}
}

func TestOcrClient_ImagePngDimsFallback(t *testing.T) {
	// 结果不带尺寸（pruned 无 width/height，dataInfo 无 width/height）→ PNG 本地读头兜底
	blocks := []map[string]any{{"block_label": "text", "block_content": "被告：王五", "block_bbox": []float64{40, 20, 240, 60}}}
	pruned := map[string]any{"parsing_res_list": blocks}
	line := map[string]any{"result": map[string]any{
		"layoutParsingResults": []map[string]any{{"markdown": map[string]any{"text": "被告：王五"}, "prunedResult": pruned}},
	}}
	b, _ := json.Marshal(line)

	srv := &mockJobServer{t: t}
	srv.jsonl = string(b)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	out, err := newTestClient().ParseDocument(context.Background(), buf.Bytes(), 1, false, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if len(out.OcrPages) != 1 {
		t.Fatalf("PNG 兜底应产出坐标行，got %+v", out.OcrPages)
	}
	l := out.OcrPages[0].Lines[0]
	if !approx(l.X, 0.1) || !approx(l.Y, 0.1) || !approx(l.W, 0.5) || !approx(l.H, 0.2) {
		t.Errorf("PNG 兜底归一化错误: %+v", l)
	}
}

func TestOcrClient_MalformedPrunedDegradesGracefully(t *testing.T) {
	// prunedResult 形状异常：markdown 主流程不受影响，坐标行为空
	line := `{"result":{"layoutParsingResults":[{"markdown":{"text":"被告：赵六"},"prunedResult":{"parsing_res_list":"not-an-array"}}]}}`
	srv := &mockJobServer{t: t}
	srv.jsonl = line
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	out, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 1, true, nil)
	if err != nil {
		t.Fatalf("形状异常不应击穿主流程: %v", err)
	}
	if len(out.Records) == 0 || out.Records[0]["defendant"] != "赵六" {
		t.Fatalf("markdown 记录应正常提取: %+v", out.Records)
	}
	if out.OcrPages != nil {
		t.Errorf("坐标行应退化为空，got %+v", out.OcrPages)
	}
}

func TestOcrClient_MultiLineJsonlAggregatesGlobalPages(t *testing.T) {
	// 两行 JSONL 各一页 → 全局页序 1、2
	l1 := jsonlFixture("第一页 被告：张三", []map[string]any{{"block_label": "text", "block_content": "被告：张三", "block_bbox": []float64{0, 0, 100, 20}}}, 1000, 1000)
	l2 := jsonlFixture("第二页 内容", []map[string]any{{"block_label": "text", "block_content": "第二页内容", "block_bbox": []float64{0, 0, 100, 20}}}, 800, 800)
	srv := &mockJobServer{t: t}
	srv.jsonl = l1 + "\n" + l2
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	withOcrEnv(t, map[string]string{
		"LEGAL_EXTRACTOR_OCR_TOKEN":   "tk",
		"LEGAL_EXTRACTOR_OCR_JOB_URL": ts.URL + "/api/v2/ocr/jobs",
	})

	out, err := newTestClient().ParseDocument(context.Background(), []byte("x"), 2, true, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if len(out.OcrPages) != 2 || out.OcrPages[0].Page != 1 || out.OcrPages[1].Page != 2 {
		t.Fatalf("跨行页序聚合错误: %+v", out.OcrPages)
	}
	// 第二页用自己的尺寸归一化（800×800）
	if !approx(out.OcrPages[1].Lines[0].W, 100.0/800) {
		t.Errorf("第二页归一化基准错误: %+v", out.OcrPages[1].Lines[0])
	}
}

// ─── buildOcrLines 守卫（纯函数，与传输层无关）───

func TestBuildOcrLines_Guards(t *testing.T) {
	// 数量不齐且无 polys → nil
	if got := buildOcrLines([]string{"a", "b"}, [][]float64{{0, 0, 1, 1}}, nil, 100, 100); got != nil {
		t.Errorf("数量不齐应返回 nil，got %+v", got)
	}
	// 页尺寸非法 → nil
	if got := buildOcrLines([]string{"a"}, [][]float64{{0, 0, 1, 1}}, nil, 0, 100); got != nil {
		t.Errorf("非法尺寸应返回 nil，got %+v", got)
	}
	// 坐标越界被钳制到 [0,1]
	got := buildOcrLines([]string{"a"}, [][]float64{{-50, 900, 1200, 1100}}, nil, 1000, 1000)
	if len(got) != 1 || !approx(got[0].X, 0) || !approx(got[0].W, 1) || !approx(got[0].Y, 0.9) || !approx(got[0].H, 0.1) {
		t.Errorf("越界坐标应被钳制: %+v", got)
	}
	// 全部行退化 → nil
	if got := buildOcrLines([]string{"a"}, [][]float64{{5, 5, 5, 5}}, nil, 100, 100); got != nil {
		t.Errorf("全退化框应返回 nil，got %+v", got)
	}
	// polys 兜底
	got = buildOcrLines([]string{"a"}, [][]float64{}, [][][]float64{{{100, 100}, {300, 100}, {300, 140}, {100, 140}}}, 1000, 1000)
	if len(got) != 1 || !approx(got[0].X, 0.1) || !approx(got[0].W, 0.2) {
		t.Errorf("polys 兜底错误: %+v", got)
	}
}
