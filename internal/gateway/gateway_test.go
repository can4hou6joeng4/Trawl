package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-license-secret"

// licenseFor 与应用端 config.GenerateLicense 同构，测试用
func licenseFor(machineID string) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(machineID))
	raw := fmt.Sprintf("%x", mac.Sum(nil))
	code := strings.ToUpper(raw[:16])
	return fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
}

// newTestGateway 起一个假上游 + 网关，返回 (网关 handler, 上游收到的请求记录)
type upstreamCall struct {
	method string
	path   string
	auth   string
	body   string
}

func newTestGateway(t *testing.T, cfg Config, upstreamHandler http.HandlerFunc) (*Gateway, *[]upstreamCall) {
	t.Helper()
	calls := &[]upstreamCall{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*calls = append(*calls, upstreamCall{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"), body: string(b)})
		if upstreamHandler != nil {
			upstreamHandler(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"jobId":"job-1"}}`))
	}))
	t.Cleanup(upstream.Close)

	cfg.UpstreamJobURL = upstream.URL + "/api/v2/ocr/jobs"
	if cfg.UpstreamToken == "" {
		cfg.UpstreamToken = "real-upstream-token"
	}
	g, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return g, calls
}

func doReq(g *Gateway, method, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

func TestGateway_SubmitForwardsWithRealToken(t *testing.T) {
	g, calls := newTestGateway(t, Config{LicenseSecret: testSecret}, nil)

	rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{
		HeaderMachineID: "ABCD1234",
		"Content-Type":  "multipart/form-data; boundary=xyz",
	}, "fake-multipart-body")

	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(*calls) != 1 {
		t.Fatalf("上游应收到 1 次请求，got %d", len(*calls))
	}
	c := (*calls)[0]
	if c.auth != "bearer real-upstream-token" {
		t.Errorf("上游鉴权头错误: %q", c.auth)
	}
	if c.body != "fake-multipart-body" {
		t.Errorf("请求体未透传: %q", c.body)
	}
	if !strings.Contains(rec.Body.String(), "job-1") {
		t.Errorf("响应未透传: %s", rec.Body.String())
	}
}

func TestGateway_RejectsInvalidMachineID(t *testing.T) {
	g, calls := newTestGateway(t, Config{}, nil)

	for _, id := range []string{"", "short", "GHIJKLMN", "ABCD1234EXTRA", "abcd123!"} {
		rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: id}, "x")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("机器码 %q 应 401，got %d", id, rec.Code)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("非法请求不应到达上游，got %d", len(*calls))
	}
}

func TestGateway_MachineIDCaseInsensitive(t *testing.T) {
	g, _ := newTestGateway(t, Config{}, nil)
	rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "abcd1234"}, "x")
	if rec.Code != 200 {
		t.Fatalf("小写机器码应被归一化接受，got %d", rec.Code)
	}
}

func TestGateway_TierQuotas(t *testing.T) {
	g, _ := newTestGateway(t, Config{LicenseSecret: testSecret, TrialDaily: 2, LicensedDaily: 4}, nil)

	// 试用档：第 3 次提交被拒
	for i := 0; i < 2; i++ {
		rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")
		if rec.Code != 200 {
			t.Fatalf("试用第 %d 次应通过，got %d", i+1, rec.Code)
		}
	}
	rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("试用超额应 403，got %d", rec.Code)
	}

	// 授权档：4 次全过，第 5 次拒
	lic := licenseFor("BBBB0002")
	for i := 0; i < 4; i++ {
		rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "BBBB0002", HeaderLicense: lic}, "x")
		if rec.Code != 200 {
			t.Fatalf("授权第 %d 次应通过，got %d", i+1, rec.Code)
		}
	}
	rec = doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "BBBB0002", HeaderLicense: lic}, "x")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("授权超额应 403，got %d", rec.Code)
	}

	// 无效授权码按试用档计
	rec = doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "CCCC0003", HeaderLicense: "FAKE-FAKE-FAKE-FAKE"}, "x")
	if rec.Code != 200 {
		t.Fatal("无效授权码应降级试用档而非拒绝")
	}
}

func TestGateway_QuotaResetsNextDay(t *testing.T) {
	g, _ := newTestGateway(t, Config{TrialDaily: 1}, nil)
	day1 := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return day1 }

	if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != 200 {
		t.Fatal("第一天首次应通过")
	}
	if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != http.StatusForbidden {
		t.Fatal("第一天第二次应拒")
	}
	g.now = func() time.Time { return day1.Add(24 * time.Hour) }
	if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != 200 {
		t.Fatal("次日配额应重置")
	}
}

func TestGateway_PollOwnership(t *testing.T) {
	g, calls := newTestGateway(t, Config{}, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":{"jobId":"job-owned"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"state":"running"}}`))
	})

	// 机器 A 提交
	doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")

	// 机器 A 轮询自己的作业 → 透传
	rec := doReq(g, "GET", "/api/v2/ocr/jobs/job-owned", map[string]string{HeaderMachineID: "AAAA0001"}, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "running") {
		t.Fatalf("归属轮询应透传，got %d %s", rec.Code, rec.Body.String())
	}

	// 机器 B 轮询别人的作业 → 404，且不触上游
	before := len(*calls)
	rec = doReq(g, "GET", "/api/v2/ocr/jobs/job-owned", map[string]string{HeaderMachineID: "BBBB0002"}, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("跨机器轮询应 404，got %d", rec.Code)
	}
	if len(*calls) != before {
		t.Error("跨机器轮询不应到达上游")
	}

	// 未知作业 → 404
	rec = doReq(g, "GET", "/api/v2/ocr/jobs/never-seen", map[string]string{HeaderMachineID: "AAAA0001"}, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("未知作业应 404，got %d", rec.Code)
	}
}

func TestGateway_RateLimitPerMinute(t *testing.T) {
	g, _ := newTestGateway(t, Config{PerMinute: 3, TrialDaily: 100}, nil)
	fixed := time.Date(2026, 7, 14, 10, 0, 30, 0, time.UTC)
	g.now = func() time.Time { return fixed }

	for i := 0; i < 3; i++ {
		if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != 200 {
			t.Fatalf("第 %d 次应通过，got %d", i+1, rec.Code)
		}
	}
	if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("超频应 429，got %d", rec.Code)
	}
	// 下一分钟恢复
	g.now = func() time.Time { return fixed.Add(time.Minute) }
	if rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x"); rec.Code != 200 {
		t.Fatal("下一分钟应恢复")
	}
}

func TestGateway_UpstreamErrorPassthrough(t *testing.T) {
	g, _ := newTestGateway(t, Config{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"msg":"upstream down"}`))
	})
	rec := doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("上游错误码应透传，got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "upstream down") {
		t.Errorf("上游错误体应透传: %s", rec.Body.String())
	}
}

func TestGateway_HealthAndUnknownPath(t *testing.T) {
	g, _ := newTestGateway(t, Config{}, nil)
	if rec := doReq(g, "GET", "/healthz", nil, ""); rec.Code != 200 {
		t.Fatal("healthz 应 200 且免鉴权")
	}
	if rec := doReq(g, "GET", "/other", map[string]string{HeaderMachineID: "AAAA0001"}, ""); rec.Code != http.StatusNotFound {
		t.Fatal("未知路径应 404")
	}
}

func TestGateway_RequiresUpstreamToken(t *testing.T) {
	if _, err := New(Config{}, nil); err == nil {
		t.Fatal("缺少上游 Token 应拒绝启动")
	}
}

func TestGateway_JobOwnerCleanup(t *testing.T) {
	var jobSeq int
	g, _ := newTestGateway(t, Config{}, func(w http.ResponseWriter, r *http.Request) {
		jobSeq++
		_, _ = fmt.Fprintf(w, `{"data":{"jobId":"job-%d"}}`, jobSeq)
	})
	base := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return base }
	doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")

	// 25 小时后再提交：旧条目应被清理
	g.now = func() time.Time { return base.Add(25 * time.Hour) }
	doReq(g, "POST", "/api/v2/ocr/jobs", map[string]string{HeaderMachineID: "AAAA0001"}, "x")

	g.mu.Lock()
	n := len(g.jobOwners)
	g.mu.Unlock()
	if n != 1 {
		t.Fatalf("过期作业归属应被清理，got %d 条", n)
	}
}
