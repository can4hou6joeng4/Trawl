package extractor

import (
	"context"
	"errors"
	"testing"
)

// TestExtractFromDocx_PreCancelledCtx 验证 DOCX 路径在 ctx 已取消时立即返回 ErrCancelled，
// 不进行任何 zip / xml 解析工作。该测试是 Iteration 4b 把 context 传播补齐到 DOCX 路径的回归网。
func TestExtractFromDocx_PreCancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 在调用之前就取消

	e := NewExtractor(nil)
	// fileData 故意是非法 docx —— 如果 ctx 检查没有提前生效，zip.NewReader 会报"非 zip"错误，
	// 我们就会拿到那个错误而不是 ErrCancelled，测试失败。
	_, err := e.extractFromDocx(ctx, []byte("not-a-real-docx"), []string{"defendant"})
	if err == nil {
		t.Fatal("expected error from pre-cancelled ctx, got nil")
	}
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled, got %v", err)
	}
}
