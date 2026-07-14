package extractor

import (
	"context"
	"os"
	"testing"

	"legal-extractor/internal/config"
)

// TestLive_DirectModeAgainstRealEndpoint 持真实凭据对生产端点做全链路回归：
// 提交 → 轮询 → JSONL 下载 → 记录与块级坐标聚合。默认跳过，按需执行：
//
//	直连：OCR_LIVE_TOKEN=xxx OCR_LIVE_FILE=/path/to/scan.pdf go test -run TestLive -v ./internal/extractor/
//	网关：OCR_LIVE_GATEWAY=https://gw.example.com/api/v2/ocr/jobs OCR_LIVE_FILE=... go test -run TestLive -v ./internal/extractor/
func TestLive_DirectModeAgainstRealEndpoint(t *testing.T) {
	token := os.Getenv("OCR_LIVE_TOKEN")
	gatewayURL := os.Getenv("OCR_LIVE_GATEWAY")
	if token == "" && gatewayURL == "" {
		t.Skip("未提供 OCR_LIVE_TOKEN 或 OCR_LIVE_GATEWAY，跳过真实端点回归")
	}
	file := os.Getenv("OCR_LIVE_FILE")
	if file == "" {
		t.Skip("未提供 OCR_LIVE_FILE，跳过真实端点回归")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("读取样张失败: %v", err)
	}

	if token != "" {
		t.Setenv("LEGAL_EXTRACTOR_OCR_TOKEN", token)
	} else {
		t.Setenv("LEGAL_EXTRACTOR_OCR_GATEWAY_URL", gatewayURL)
	}
	if err := config.Init(""); err != nil {
		t.Fatalf("config.Init: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("LEGAL_EXTRACTOR_OCR_TOKEN")
		_ = os.Unsetenv("LEGAL_EXTRACTOR_OCR_GATEWAY_URL")
		_ = config.Init("")
	})

	e := NewExtractor(nil)
	out, err := e.ExtractData(context.Background(), data, file, nil, func(cur, total int, msg string) {
		t.Logf("进度 %d/%d %s", cur, total, msg)
	})
	if err != nil {
		t.Fatalf("真实端点提取失败: %v", err)
	}

	t.Logf("记录数: %d", len(out.Records))
	for i, r := range out.Records {
		t.Logf("  REC%02d page=%s defendant=%q idNumber=%q", i+1, r["page"], r["defendant"], r["idNumber"])
	}
	t.Logf("坐标页数: %d", len(out.OcrPages))
	for _, p := range out.OcrPages {
		t.Logf("  第 %d 页 %d 行；首行 %q @ (%.3f, %.3f, %.3f, %.3f)",
			p.Page, len(p.Lines), p.Lines[0].Text, p.Lines[0].X, p.Lines[0].Y, p.Lines[0].W, p.Lines[0].H)
	}

	if len(out.Records) == 0 {
		t.Error("真实样张应提取到记录")
	}
	if len(out.OcrPages) == 0 {
		t.Error("真实样张应产出块级坐标（若官方 schema 变化需排查 parsing_res_list）")
	}
}
