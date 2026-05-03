package app

import (
	"context"
	"testing"
	"time"
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
