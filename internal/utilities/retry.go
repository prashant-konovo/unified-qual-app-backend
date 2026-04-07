package utilities

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"
)

// RetryConfig controls the behaviour of WithRetry.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts (1 = no retry). Default: 3.
	MaxAttempts int
	// InitialDelay is the base delay before the first retry. Default: 200ms.
	InitialDelay time.Duration
	// MaxDelay caps the backoff delay. Default: 10s.
	MaxDelay time.Duration
	// Multiplier scales the delay after each attempt. Default: 2.0.
	Multiplier float64
	// Jitter adds ±percentage randomness to each delay (0.0–1.0). Default: 0.25 (25%).
	Jitter float64
	// RetryIf decides whether a failed attempt should be retried.
	// If nil every non-nil error is retried.
	RetryIf func(err error) bool
	// OnRetry is called before each retry with the attempt number and error.
	OnRetry func(attempt int, err error)
}

func (c *RetryConfig) defaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.InitialDelay <= 0 {
		c.InitialDelay = 200 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 10 * time.Second
	}
	if c.Multiplier <= 0 {
		c.Multiplier = 2.0
	}
	if c.Jitter < 0 || c.Jitter > 1 {
		c.Jitter = 0.25
	}
}

// WithRetry executes fn up to cfg.MaxAttempts times with exponential backoff
// and optional jitter. It respects context cancellation between attempts.
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	cfg.defaults()

	var zero T
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Check if we should retry this error
		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return zero, err
		}

		// Last attempt — don't sleep
		if attempt == cfg.MaxAttempts {
			break
		}

		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt, err)
		} else {
			slog.Warn("retrying external call",
				"attempt", attempt,
				"maxAttempts", cfg.MaxAttempts,
				"delay", delay,
				"error", err,
			)
		}

		// Apply jitter: delay ± jitter%
		jittered := delay
		if cfg.Jitter > 0 {
			delta := float64(delay) * cfg.Jitter
			jittered = time.Duration(float64(delay) + (rand.Float64()*2-1)*delta)
		}

		// Wait or bail on context cancellation
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(jittered):
		}

		// Exponential backoff capped at MaxDelay
		delay = time.Duration(math.Min(
			float64(delay)*cfg.Multiplier,
			float64(cfg.MaxDelay),
		))
	}

	return zero, fmt.Errorf("all %d attempts failed: %w", cfg.MaxAttempts, lastErr)
}
