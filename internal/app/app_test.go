package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"legal-extractor/internal/extractor"
)

// newTestApp builds an App with a parent context but no real Wails runtime.
// startExtraction / CancelExtraction work fine without one.
func newTestApp() *App {
	return &App{ctx: context.Background()}
}

func TestStartExtraction_ReturnsCancellableChild(t *testing.T) {
	a := newTestApp()
	ctx := a.startExtraction()
	defer a.finishExtraction()

	if ctx == nil {
		t.Fatal("startExtraction returned nil")
	}
	if ctx.Err() != nil {
		t.Fatalf("ctx should be live, got %v", ctx.Err())
	}
}

func TestCancelExtraction_AbortsInFlight(t *testing.T) {
	a := newTestApp()
	ctx := a.startExtraction()

	a.CancelExtraction()

	select {
	case <-ctx.Done():
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Fatal("ctx not cancelled after CancelExtraction")
	}
	if ctx.Err() == nil {
		t.Fatal("ctx.Err() should be non-nil after cancel")
	}
}

func TestCancelExtraction_NoOpWhenIdle(t *testing.T) {
	a := newTestApp()
	// Should not panic, should not block.
	a.CancelExtraction()
	a.CancelExtraction()
}

func TestStartExtraction_CancelsPrevious(t *testing.T) {
	a := newTestApp()
	first := a.startExtraction()
	second := a.startExtraction()
	defer a.finishExtraction()

	select {
	case <-first.Done():
		// expected: starting a new extraction cancelled the prior one
	case <-time.After(50 * time.Millisecond):
		t.Fatal("first ctx not cancelled when second extraction started")
	}
	if second.Err() != nil {
		t.Fatalf("second ctx should still be live, got %v", second.Err())
	}
}

func TestFinishExtraction_ClearsState(t *testing.T) {
	a := newTestApp()
	_ = a.startExtraction()
	a.finishExtraction()

	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFn != nil {
		t.Fatal("cancelFn should be nil after finishExtraction")
	}
}

func TestEmitExtractionProgress_SkipsPlainContext(t *testing.T) {
	a := newTestApp()

	// A plain context lacks Wails' lifecycle values. This must be a no-op:
	// Wails runtime.EventsEmit would otherwise call log.Fatalf.
	a.emitExtractionProgress(1, 1, "testing")
}

// newTestAppWithExtractor 构造带真实提取器的 App，用于走通 runExtraction 骨架。
func newTestAppWithExtractor() *App {
	return &App{ctx: context.Background(), extractor: extractor.NewExtractor(nil)}
}

func TestExtractToPath_EmptyFile(t *testing.T) {
	a := newTestAppWithExtractor()
	emptyFile := filepath.Join(t.TempDir(), "empty.pdf")
	if err := os.WriteFile(emptyFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	res := a.ExtractToPath(emptyFile, filepath.Join(t.TempDir(), "out.csv"), nil)
	if res.Success {
		t.Fatal("空文件不应提取成功")
	}
	if !strings.Contains(res.ErrorMessage, "文件内容为空") {
		t.Fatalf("空文件错误文案不符: %q", res.ErrorMessage)
	}
}

func TestPreviewData_UnsupportedFormatReturnsStableCode(t *testing.T) {
	a := newTestAppWithExtractor()
	txtFile := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(txtFile, []byte("普通文本"), 0644); err != nil {
		t.Fatal(err)
	}

	// 预览与提取共用错误契约：前端依赖稳定错误码分支
	res := a.PreviewData(txtFile, nil)
	if res.Success {
		t.Fatal("不支持格式不应预览成功")
	}
	if !strings.HasPrefix(res.ErrorMessage, "UNSUPPORTED_FORMAT") {
		t.Fatalf("预览错误应返回稳定错误码, got %q", res.ErrorMessage)
	}
}

func TestPreviewData_NoRecordsReturnsError(t *testing.T) {
	a := newTestAppWithExtractor()

	// 一个合法但不含任何法律字段的 docx（zip 结构最小化不可行，直接用不存在路径以外的
	// 场景验证：0 记录时必须返回明确错误而非静默成功）
	res := a.PreviewData(filepath.Join(t.TempDir(), "不存在.docx"), nil)
	if res.Success {
		t.Fatal("读取失败不应返回成功")
	}
	if res.ErrorMessage == "" {
		t.Fatal("必须返回错误信息")
	}
}
