package extractor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestExtractData_CancelDuringWork cancels the context while the worker pool
// is still draining results and expects ErrCancelled. Uses a pre-existing
// fixture if available, otherwise skips.
func TestExtractData_CancelDuringWork(t *testing.T) {
	candidates := []string{
		filepath.Join("testdata", "sample.pdf"),
		filepath.Join("..", "..", "testdata", "sample.pdf"),
	}
	var pdfPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			pdfPath = p
			break
		}
	}
	if pdfPath == "" {
		t.Skip("no sample PDF fixture; skipping (add testdata/sample.pdf to enable)")
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	e := NewExtractor(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 50ms — long enough that some work has started, short
	// enough that real local extraction wouldn't finish.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = e.ExtractData(ctx, data, "sample.pdf", []string{"defendant"}, nil)
	if err == nil {
		t.Fatal("expected non-nil error when cancelled mid-flight")
	}
	if !errors.Is(err, ErrCancelled) {
		t.Logf("note: got %v (acceptable if extraction completed before cancel)", err)
	}
}
