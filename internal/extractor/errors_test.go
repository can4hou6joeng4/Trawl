package extractor

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrPDFEncrypted_Is(t *testing.T) {
	wrapped := fmt.Errorf("处理失败: %w", ErrPDFEncrypted)
	if !errors.Is(wrapped, ErrPDFEncrypted) {
		t.Fatalf("errors.Is should match wrapped ErrPDFEncrypted")
	}
}

func TestErrUnsupportedFormat_Is(t *testing.T) {
	wrapped := fmt.Errorf("file foo.txt: %w", ErrUnsupportedFormat)
	if !errors.Is(wrapped, ErrUnsupportedFormat) {
		t.Fatalf("errors.Is should match wrapped ErrUnsupportedFormat")
	}
}

func TestErrTokenMissing_Is(t *testing.T) {
	if !errors.Is(ErrTokenMissing, ErrTokenMissing) {
		t.Fatalf("ErrTokenMissing should be Is-equal to itself")
	}
}

func TestSentinels_Distinct(t *testing.T) {
	if errors.Is(ErrPDFEncrypted, ErrUnsupportedFormat) {
		t.Fatal("distinct sentinels must not match each other")
	}
}
