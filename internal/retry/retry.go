// Package retry provides automatic retry logic with exponential backoff and jitter
// for handling transient failures when calling external APIs.
package retry

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// Sentinel errors for retry conditions.
var (
	// ErrMaxRetriesExceeded is returned when all retry attempts have been exhausted.
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	// ErrNonRetryable is returned when the error is not retryable.
	ErrNonRetryable = errors.New("non-retryable error")
)

// Config holds retry configuration.
type Config struct {
	// MaxRetries is the maximum number of retry attempts (default: 5).
	MaxRetries int
	// InitialDelay is the initial delay before first retry (default: 1s).
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries (default: 16s).
	MaxDelay time.Duration
	// Multiplier is the backoff multiplier (default: 2.0).
	Multiplier float64
	// JitterFactor is the jitter factor (0.0 to 1.0) for randomization (default: 0.2).
	JitterFactor float64
	// RetryableStatusCodes are HTTP status codes that should trigger a retry.
	RetryableStatusCodes []int
	// Logger for retry events.
	Logger *slog.Logger
}

// DefaultConfig returns the default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxRetries:   5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     16 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.2,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
		Logger: slog.Default(),
	}
}

// Retryer provides retry functionality with exponential backoff.
type Retryer struct {
	config           Config
	retryableStatus  map[int]bool
}

// New creates a new Retryer with the given configuration.
func New(config Config) *Retryer {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 5
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 16 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.JitterFactor <= 0 || config.JitterFactor > 1 {
		config.JitterFactor = 0.2
	}
	if len(config.RetryableStatusCodes) == 0 {
		config.RetryableStatusCodes = []int{429, 500, 502, 503, 504}
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Build lookup map for retryable status codes
	statusMap := make(map[int]bool, len(config.RetryableStatusCodes))
	for _, code := range config.RetryableStatusCodes {
		statusMap[code] = true
	}

	return &Retryer{
		config:          config,
		retryableStatus: statusMap,
	}
}

// RetryableError represents an error that can be retried.
type RetryableError struct {
	StatusCode int
	Err        error
	Attempt    int
}

// Error returns the error message.
func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an HTTP status code is retryable.
func (r *Retryer) IsRetryable(statusCode int) bool {
	return r.retryableStatus[statusCode]
}

// CalculateDelay calculates the delay for a given attempt using exponential backoff with jitter.
// Attempt is 1-based (first retry is attempt 1).
func (r *Retryer) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate base delay with exponential backoff: initialDelay * multiplier^(attempt-1)
	delay := float64(r.config.InitialDelay)
	for i := 1; i < attempt; i++ {
		delay *= r.config.Multiplier
	}

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Apply jitter: delay * (1 - jitterFactor + random(0, 2*jitterFactor))
	// This gives a range of [delay*(1-jitterFactor), delay*(1+jitterFactor)]
	jitterRange := delay * r.config.JitterFactor
	jitter := rand.Float64()*2*jitterRange - jitterRange
	delay += jitter

	// Ensure delay is at least 1ms
	if delay < float64(time.Millisecond) {
		delay = float64(time.Millisecond)
	}

	return time.Duration(delay)
}

// Operation is a function that can be retried. It returns an HTTP status code and an error.
// If the status code is retryable and error is not nil, the operation will be retried.
type Operation func(ctx context.Context) (statusCode int, err error)

// Do executes the operation with retry logic.
// Returns the last error encountered if all retries are exhausted.
func (r *Retryer) Do(ctx context.Context, op Operation) error {
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return ctx.Err()
		}

		statusCode, err := op(ctx)
		if err == nil {
			// Success
			if attempt > 0 {
				r.config.Logger.Info("operation succeeded after retry",
					slog.Int("attempts", attempt+1),
				)
			}
			return nil
		}

		lastErr = err
		lastStatusCode = statusCode

		// Check if the error is retryable
		if !r.IsRetryable(statusCode) {
			r.config.Logger.Debug("non-retryable error",
				slog.Int("status_code", statusCode),
				slog.String("error", err.Error()),
			)
			return &RetryableError{
				StatusCode: statusCode,
				Err:        err,
				Attempt:    attempt + 1,
			}
		}

		// Don't wait after the last attempt
		if attempt >= r.config.MaxRetries {
			break
		}

		// Calculate delay and wait
		delay := r.CalculateDelay(attempt + 1)
		r.config.Logger.Warn("retrying operation",
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", r.config.MaxRetries),
			slog.Int("status_code", statusCode),
			slog.Duration("delay", delay),
			slog.String("error", err.Error()),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	r.config.Logger.Error("max retries exceeded",
		slog.Int("max_retries", r.config.MaxRetries),
		slog.Int("last_status_code", lastStatusCode),
		slog.String("last_error", lastErr.Error()),
	)

	return &RetryableError{
		StatusCode: lastStatusCode,
		Err:        errors.Join(ErrMaxRetriesExceeded, lastErr),
		Attempt:    r.config.MaxRetries + 1,
	}
}

// DoWithResult executes the operation with retry logic and returns a result.
// The operation function returns a result, status code, and error.
func DoWithResult[T any](ctx context.Context, r *Retryer, op func(ctx context.Context) (T, int, error)) (T, error) {
	var result T
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		res, statusCode, err := op(ctx)
		if err == nil {
			// Success
			if attempt > 0 {
				r.config.Logger.Info("operation succeeded after retry",
					slog.Int("attempts", attempt+1),
				)
			}
			return res, nil
		}

		lastErr = err
		lastStatusCode = statusCode
		result = res // Keep last result

		// Check if the error is retryable
		if !r.IsRetryable(statusCode) {
			r.config.Logger.Debug("non-retryable error",
				slog.Int("status_code", statusCode),
				slog.String("error", err.Error()),
			)
			return result, &RetryableError{
				StatusCode: statusCode,
				Err:        err,
				Attempt:    attempt + 1,
			}
		}

		// Don't wait after the last attempt
		if attempt >= r.config.MaxRetries {
			break
		}

		// Calculate delay and wait
		delay := r.CalculateDelay(attempt + 1)
		r.config.Logger.Warn("retrying operation",
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", r.config.MaxRetries),
			slog.Int("status_code", statusCode),
			slog.Duration("delay", delay),
			slog.String("error", err.Error()),
		)

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	r.config.Logger.Error("max retries exceeded",
		slog.Int("max_retries", r.config.MaxRetries),
		slog.Int("last_status_code", lastStatusCode),
		slog.String("last_error", lastErr.Error()),
	)

	return result, &RetryableError{
		StatusCode: lastStatusCode,
		Err:        errors.Join(ErrMaxRetriesExceeded, lastErr),
		Attempt:    r.config.MaxRetries + 1,
	}
}

// Config getters for inspection.

// MaxRetries returns the maximum number of retries.
func (r *Retryer) MaxRetries() int {
	return r.config.MaxRetries
}

// InitialDelay returns the initial delay.
func (r *Retryer) InitialDelay() time.Duration {
	return r.config.InitialDelay
}

// MaxDelay returns the maximum delay.
func (r *Retryer) MaxDelay() time.Duration {
	return r.config.MaxDelay
}

// Multiplier returns the backoff multiplier.
func (r *Retryer) Multiplier() float64 {
	return r.config.Multiplier
}

// JitterFactor returns the jitter factor.
func (r *Retryer) JitterFactor() float64 {
	return r.config.JitterFactor
}

// RetryableStatusCodes returns the list of retryable status codes.
func (r *Retryer) RetryableStatusCodes() []int {
	return r.config.RetryableStatusCodes
}
