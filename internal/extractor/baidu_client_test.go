package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"legal-extractor/internal/config"
)

// withMockBaidu 启动假百度服务并把全局配置指向它；handler 返回响应 JSON。
// 测试结束后恢复无 Token 状态，避免污染同包其他用例。
func withMockBaidu(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Setenv("LEGAL_EXTRACTOR_BAIDU_TOKEN", "test-token")
	t.Setenv("LEGAL_EXTRACTOR_BAIDU_API_URL", server.URL)
	if err := config.Init(""); err != nil {
		t.Fatalf("config.Init: %v", err)
	}
	t.Cleanup(func() {
		server.Close()
		_ = os.Unsetenv("LEGAL_EXTRACTOR_BAIDU_TOKEN")
		_ = os.Unsetenv("LEGAL_EXTRACTOR_BAIDU_API_URL")
		_ = config.Init("") // 还原为无 Token 配置
	})
	return server
}

// mockLayoutResponse 按 PaddleX 服务化契约拼一页响应
type mockPage struct {
	markdown string
	texts    []string
	boxes    [][]float64
	polys    [][][]float64
}

func mockResponseJSON(pages []mockPage, dataInfo map[string]any) []byte {
	results := make([]map[string]any, 0, len(pages))
	for _, p := range pages {
		ocr := map[string]any{"rec_texts": p.texts}
		if p.boxes != nil {
			ocr["rec_boxes"] = p.boxes
		}
		if p.polys != nil {
			ocr["rec_polys"] = p.polys
		}
		results = append(results, map[string]any{
			"markdown":     map[string]any{"text": p.markdown},
			"prunedResult": map[string]any{"overall_ocr_res": ocr},
		})
	}
	result := map[string]any{"layoutParsingResults": results}
	if dataInfo != nil {
		result["dataInfo"] = dataInfo
	}
	body, _ := json.Marshal(map[string]any{"error_code": 0, "result": result})
	return body
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestParseDocument_PdfWithCoordinates(t *testing.T) {
	pageMd := "# 民事起诉状\n被告：张三，身份证号码：110101199001011234"
	var gotPayload map[string]any
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		_, _ = w.Write(mockResponseJSON(
			[]mockPage{
				{
					markdown: pageMd,
					texts:    []string{"被告：张三", "", "身份证号码：110101199001011234"},
					boxes:    [][]float64{{100, 200, 500, 240}, {0, 0, 10, 10}, {100, 260, 700, 300}},
				},
				{
					markdown: "第二页无字段",
					texts:    []string{"到期后仍未偿还"},
					boxes:    [][]float64{{50, 100, 450, 140}},
				},
			},
			map[string]any{
				"numPages": 2,
				"pages": []map[string]any{
					{"width": 1000, "height": 1400},
					{"width": 800, "height": 1200},
				},
			},
		))
	})

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), []byte("fake-pdf"), 2, true, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	// 请求侧契约：不回传可视化图片
	if v, ok := gotPayload["visualize"].(bool); !ok || v {
		t.Errorf("payload 应包含 visualize=false，got %v", gotPayload["visualize"])
	}

	if len(out.Records) == 0 || out.Records[0]["defendant"] != "张三" {
		t.Fatalf("markdown 记录提取不应受坐标解析影响: %+v", out.Records)
	}

	if len(out.OcrPages) != 2 {
		t.Fatalf("应产出两页坐标行，got %d", len(out.OcrPages))
	}
	p1 := out.OcrPages[0]
	if p1.Page != 1 {
		t.Fatalf("第一页页码应为 1，got %d", p1.Page)
	}
	// 空文本行被跳过：3 行输入 → 2 行输出
	if len(p1.Lines) != 2 {
		t.Fatalf("空文本行应被跳过，got %d 行", len(p1.Lines))
	}
	l := p1.Lines[0]
	if l.Text != "被告：张三" || !approx(l.X, 0.1) || !approx(l.Y, 200.0/1400) || !approx(l.W, 0.4) || !approx(l.H, 40.0/1400) {
		t.Errorf("第一行归一化坐标错误: %+v", l)
	}
	// 第二页用自己的尺寸归一化
	l2 := out.OcrPages[1].Lines[0]
	if out.OcrPages[1].Page != 2 || !approx(l2.X, 50.0/800) || !approx(l2.H, 40.0/1200) {
		t.Errorf("第二页归一化坐标错误: page=%d %+v", out.OcrPages[1].Page, l2)
	}
}

func TestParseDocument_PolysFallbackAndMissingDims(t *testing.T) {
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(mockResponseJSON(
			[]mockPage{
				{ // rec_boxes 数量不齐 → rec_polys 兜底
					markdown: "被告：李四",
					texts:    []string{"被告：李四"},
					boxes:    [][]float64{},
					polys:    [][][]float64{{{100, 100}, {300, 100}, {300, 140}, {100, 140}}},
				},
				{ // dataInfo.pages 缺第二页尺寸 → 该页无坐标行
					markdown: "第二页",
					texts:    []string{"某行"},
					boxes:    [][]float64{{10, 10, 50, 30}},
				},
			},
			map[string]any{
				"numPages": 2,
				"pages":    []map[string]any{{"width": 1000, "height": 1000}},
			},
		))
	})

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), []byte("fake-pdf"), 2, true, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	if len(out.OcrPages) != 1 {
		t.Fatalf("只有第一页应产出坐标行，got %d 页", len(out.OcrPages))
	}
	l := out.OcrPages[0].Lines[0]
	if !approx(l.X, 0.1) || !approx(l.Y, 0.1) || !approx(l.W, 0.2) || !approx(l.H, 0.04) {
		t.Errorf("polys 包围盒归一化错误: %+v", l)
	}
}

func TestParseDocument_ImageDimsFallbackToLocalDecode(t *testing.T) {
	// 响应不带 dataInfo → 后端读图片头兜底（真实 PNG 400x200）
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(mockResponseJSON(
			[]mockPage{{
				markdown: "被告：王五",
				texts:    []string{"被告：王五"},
				boxes:    [][]float64{{40, 20, 240, 60}},
			}},
			nil,
		))
	})

	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), buf.Bytes(), 1, false, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if len(out.OcrPages) != 1 {
		t.Fatalf("图片应产出一页坐标行，got %d", len(out.OcrPages))
	}
	l := out.OcrPages[0].Lines[0]
	if !approx(l.X, 0.1) || !approx(l.Y, 0.1) || !approx(l.W, 0.5) || !approx(l.H, 0.2) {
		t.Errorf("图片尺寸兜底归一化错误: %+v", l)
	}
}

func TestParseDocument_CoordinatesUnavailable_RecordsStillWork(t *testing.T) {
	// 无 prunedResult（旧部署/字段缺失）→ 零坐标行，记录提取照常
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error_code":0,"result":{"layoutParsingResults":[{"markdown":{"text":"被告：赵六"}}]}}`))
	})

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), []byte("fake-pdf"), 1, true, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if len(out.Records) == 0 || out.Records[0]["defendant"] != "赵六" {
		t.Fatalf("记录提取应不受影响: %+v", out.Records)
	}
	if out.OcrPages != nil {
		t.Errorf("无坐标数据时 OcrPages 应为 nil，got %+v", out.OcrPages)
	}
}

func TestParseDocument_MalformedEnhancementFields_RecordsStillWork(t *testing.T) {
	// 增强字段形状异常（rec_boxes 呈 polys 形状、dataInfo 为字符串）：
	// 只降级坐标行，markdown 主流程必须照常成功——「宁缺毋错」契约
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error_code":0,"result":{
			"dataInfo": "unexpected-shape",
			"layoutParsingResults":[{
				"markdown":{"text":"被告：钱七"},
				"prunedResult":{"overall_ocr_res":{"rec_texts":["a"],"rec_boxes":[[[1,2],[3,4]]]}}
			}]}}`))
	})

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), []byte("fake-pdf"), 1, true, nil)
	if err != nil {
		t.Fatalf("形状异常不应击穿提取: %v", err)
	}
	if len(out.Records) == 0 || out.Records[0]["defendant"] != "钱七" {
		t.Fatalf("markdown 记录应照常提取: %+v", out.Records)
	}
	if out.OcrPages != nil {
		t.Errorf("形状异常时坐标行应退化为 nil，got %+v", out.OcrPages)
	}
}

func TestParseDocument_JpegWithoutDataInfo_NoLocalFallback(t *testing.T) {
	// JPEG 可能带 EXIF 旋转，本地尺寸兜底只对 PNG 开放：
	// 响应缺 dataInfo 的 JPEG → 无坐标行（宁缺毋错），记录照常
	withMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(mockResponseJSON(
			[]mockPage{{
				markdown: "被告：孙八",
				texts:    []string{"被告：孙八"},
				boxes:    [][]float64{{10, 10, 100, 40}},
			}},
			nil,
		))
	})

	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}

	c := NewBaiduClient(nil)
	out, err := c.ParseDocument(context.Background(), buf.Bytes(), 1, false, nil)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if len(out.Records) == 0 || out.Records[0]["defendant"] != "孙八" {
		t.Fatalf("记录提取应不受影响: %+v", out.Records)
	}
	if out.OcrPages != nil {
		t.Errorf("JPEG 无 dataInfo 时不应本地兜底，OcrPages 应为 nil，got %+v", out.OcrPages)
	}
}

func TestSanitizeChunkPages(t *testing.T) {
	mk := func(n int) []ocrPage {
		pages := make([]ocrPage, n)
		for i := range pages {
			pages[i] = ocrPage{markdown: "md", lines: []OcrLine{{Text: "x", X: 0.1, Y: 0.1, W: 0.2, H: 0.02}}}
		}
		return pages
	}

	// 页数吻合：原样返回
	ok := sanitizeChunkPages(mk(3), 3, testLogger())
	if len(ok) != 3 || ok[0].lines == nil {
		t.Fatal("页数吻合时不应改动坐标行")
	}

	// 页数不符（云端缺页）：markdown 保留、坐标行全部抹掉，防全局页序错位
	bad := sanitizeChunkPages(mk(2), 3, testLogger())
	if len(bad) != 2 {
		t.Fatal("页数不符时不应增删页")
	}
	for _, p := range bad {
		if p.lines != nil {
			t.Fatal("页数不符时坐标行应退化为 nil")
		}
		if p.markdown != "md" {
			t.Fatal("markdown 应保留")
		}
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBuildOcrLines_Guards(t *testing.T) {
	// 长度不齐且无 polys → nil
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
}
