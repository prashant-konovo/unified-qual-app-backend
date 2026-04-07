package utilities

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	result, err := WithRetry(context.Background(), RetryConfig{MaxAttempts: 3}, func(_ context.Context) (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	result, err := WithRetry(context.Background(), RetryConfig{
		MaxAttempts:  4,
		InitialDelay: time.Millisecond,
	}, func(_ context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_ExhaustsAllAttempts(t *testing.T) {
	calls := 0
	_, err := WithRetry(context.Background(), RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
	}, func(_ context.Context) (string, error) {
		calls++
		return "", errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_RespectsRetryIf(t *testing.T) {
	permanent := errors.New("do not retry")
	calls := 0
	_, err := WithRetry(context.Background(), RetryConfig{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
		RetryIf:      func(err error) bool { return !errors.Is(err, permanent) },
	}, func(_ context.Context) (string, error) {
		calls++
		return "", permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", calls)
	}
}

func TestWithRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := WithRetry(ctx, RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 50 * time.Millisecond,
	}, func(_ context.Context) (string, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return "", errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls > 3 {
		t.Fatalf("expected ≤3 calls before cancellation, got %d", calls)
	}
}

func TestWithRetry_DefaultConfig(t *testing.T) {
	cfg := RetryConfig{}
	cfg.defaults()
	if cfg.MaxAttempts != 3 {
		t.Fatalf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 200*time.Millisecond {
		t.Fatalf("expected InitialDelay=200ms, got %v", cfg.InitialDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Fatalf("expected Multiplier=2.0, got %f", cfg.Multiplier)
	}
}

func TestWithRetry_OnRetryCallback(t *testing.T) {
	retried := 0
	_, _ = WithRetry(context.Background(), RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		OnRetry:      func(attempt int, _ error) { retried = attempt },
	}, func(_ context.Context) (string, error) {
		return "", errors.New("fail")
	})
	if retried != 2 {
		t.Fatalf("expected OnRetry called with attempt=2, got %d", retried)
	}
}
