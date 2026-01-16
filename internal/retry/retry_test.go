package retry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", config.MaxRetries)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay 1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 16*time.Second {
		t.Errorf("expected MaxDelay 16s, got %v", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", config.Multiplier)
	}
	if config.JitterFactor != 0.2 {
		t.Errorf("expected JitterFactor 0.2, got %f", config.JitterFactor)
	}
	if len(config.RetryableStatusCodes) != 5 {
		t.Errorf("expected 5 retryable status codes, got %d", len(config.RetryableStatusCodes))
	}
	if config.Logger == nil {
		t.Error("expected Logger to be set")
	}
}

func TestNew(t *testing.T) {
	t.Run("uses default values when config is empty", func(t *testing.T) {
		retryer := New(Config{})

		if retryer.MaxRetries() != 5 {
			t.Errorf("expected MaxRetries 5, got %d", retryer.MaxRetries())
		}
		if retryer.InitialDelay() != 1*time.Second {
			t.Errorf("expected InitialDelay 1s, got %v", retryer.InitialDelay())
		}
		if retryer.MaxDelay() != 16*time.Second {
			t.Errorf("expected MaxDelay 16s, got %v", retryer.MaxDelay())
		}
		if retryer.Multiplier() != 2.0 {
			t.Errorf("expected Multiplier 2.0, got %f", retryer.Multiplier())
		}
		if retryer.JitterFactor() != 0.2 {
			t.Errorf("expected JitterFactor 0.2, got %f", retryer.JitterFactor())
		}
	})

	t.Run("uses provided configuration", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         500 * time.Millisecond,
			MaxDelay:             8 * time.Second,
			Multiplier:           3.0,
			JitterFactor:         0.1,
			RetryableStatusCodes: []int{503},
			Logger:               slog.Default(),
		})

		if retryer.MaxRetries() != 3 {
			t.Errorf("expected MaxRetries 3, got %d", retryer.MaxRetries())
		}
		if retryer.InitialDelay() != 500*time.Millisecond {
			t.Errorf("expected InitialDelay 500ms, got %v", retryer.InitialDelay())
		}
		if retryer.MaxDelay() != 8*time.Second {
			t.Errorf("expected MaxDelay 8s, got %v", retryer.MaxDelay())
		}
		if retryer.Multiplier() != 3.0 {
			t.Errorf("expected Multiplier 3.0, got %f", retryer.Multiplier())
		}
		if retryer.JitterFactor() != 0.1 {
			t.Errorf("expected JitterFactor 0.1, got %f", retryer.JitterFactor())
		}
	})

	t.Run("handles invalid jitter factor", func(t *testing.T) {
		retryer := New(Config{
			JitterFactor: 1.5, // Invalid, > 1
		})
		if retryer.JitterFactor() != 0.2 {
			t.Errorf("expected JitterFactor to fall back to 0.2, got %f", retryer.JitterFactor())
		}

		retryer2 := New(Config{
			JitterFactor: -0.5, // Invalid, < 0
		})
		if retryer2.JitterFactor() != 0.2 {
			t.Errorf("expected JitterFactor to fall back to 0.2, got %f", retryer2.JitterFactor())
		}
	})
}

func TestRetryer_IsRetryable(t *testing.T) {
	retryer := New(DefaultConfig())

	testCases := []struct {
		statusCode int
		expected   bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, tc := range testCases {
		if got := retryer.IsRetryable(tc.statusCode); got != tc.expected {
			t.Errorf("IsRetryable(%d): expected %v, got %v", tc.statusCode, tc.expected, got)
		}
	}
}

func TestRetryer_CalculateDelay(t *testing.T) {
	// Use a very small jitter factor for more predictable tests
	// 0.001 = 0.1% jitter, negligible but not triggering defaults
	retryer := New(Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     16 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.001, // Minimal jitter for nearly predictable tests
		Logger:       slog.Default(),
	})

	t.Run("returns 0 for attempt 0 or negative", func(t *testing.T) {
		if delay := retryer.CalculateDelay(0); delay != 0 {
			t.Errorf("expected 0 delay for attempt 0, got %v", delay)
		}
		if delay := retryer.CalculateDelay(-1); delay != 0 {
			t.Errorf("expected 0 delay for attempt -1, got %v", delay)
		}
	})

	t.Run("calculates exponential backoff", func(t *testing.T) {
		// Expected base delays (with minimal jitter, should be within 0.2% of expected)
		expectedDelays := []time.Duration{
			1 * time.Second,  // Attempt 1: 1s
			2 * time.Second,  // Attempt 2: 1s * 2 = 2s
			4 * time.Second,  // Attempt 3: 2s * 2 = 4s
			8 * time.Second,  // Attempt 4: 4s * 2 = 8s
			16 * time.Second, // Attempt 5: 8s * 2 = 16s (capped at max)
			16 * time.Second, // Attempt 6: would be 32s but capped at 16s
		}

		tolerance := 0.01 // 1% tolerance for timing
		for i, expected := range expectedDelays {
			delay := retryer.CalculateDelay(i + 1)
			minExpected := time.Duration(float64(expected) * (1 - tolerance))
			maxExpected := time.Duration(float64(expected) * (1 + tolerance))
			if delay < minExpected || delay > maxExpected {
				t.Errorf("attempt %d: expected %v (±1%%), got %v", i+1, expected, delay)
			}
		}
	})

	t.Run("applies jitter within expected range", func(t *testing.T) {
		retryerWithJitter := New(Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     16 * time.Second,
			Multiplier:   2.0,
			JitterFactor: 0.2, // 20% jitter
			Logger:       slog.Default(),
		})

		// Run multiple times to check jitter is applied
		baseDelay := 1 * time.Second
		minDelay := float64(baseDelay) * 0.8 // 800ms
		maxDelay := float64(baseDelay) * 1.2 // 1200ms

		for range 100 {
			delay := retryerWithJitter.CalculateDelay(1)
			delayFloat := float64(delay)

			if delayFloat < minDelay || delayFloat > maxDelay {
				t.Errorf("delay %v is outside expected range [%v, %v]",
					delay, time.Duration(minDelay), time.Duration(maxDelay))
			}
		}
	})
}

func TestRetryer_Do(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:   3,
			InitialDelay: 10 * time.Millisecond,
			Logger:       slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			return http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries on 429 and succeeds", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{429},
			Logger:               slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			if attempts < 3 {
				return http.StatusTooManyRequests, errors.New("rate limited")
			}
			return http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("retries on 500 errors", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{500},
			Logger:               slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			if attempts < 2 {
				return http.StatusInternalServerError, errors.New("internal error")
			}
			return http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retries on 503 errors", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{503},
			Logger:               slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			if attempts < 2 {
				return http.StatusServiceUnavailable, errors.New("service unavailable")
			}
			return http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("stops after max retries", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{429},
			Logger:               slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			return http.StatusTooManyRequests, errors.New("rate limited")
		})

		if err == nil {
			t.Error("expected error after max retries")
		}
		if attempts != 4 { // 1 initial + 3 retries
			t.Errorf("expected 4 attempts (1 + 3 retries), got %d", attempts)
		}

		var retryableErr *RetryableError
		if !errors.As(err, &retryableErr) {
			t.Errorf("expected RetryableError, got %T", err)
		}
		if !errors.Is(retryableErr.Err, ErrMaxRetriesExceeded) {
			t.Error("expected ErrMaxRetriesExceeded to be wrapped")
		}
	})

	t.Run("does not retry non-retryable errors", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{429, 500, 503},
			Logger:               slog.Default(),
		})

		var attempts int
		err := retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			attempts++
			return http.StatusBadRequest, errors.New("bad request")
		})

		if err == nil {
			t.Error("expected error for non-retryable status")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry for 400), got %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           5,
			InitialDelay:         100 * time.Millisecond,
			RetryableStatusCodes: []int{429},
			Logger:               slog.Default(),
		})

		ctx, cancel := context.WithCancel(context.Background())

		var attempts int32
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := retryer.Do(ctx, func(ctx context.Context) (int, error) {
			atomic.AddInt32(&attempts, 1)
			return http.StatusTooManyRequests, errors.New("rate limited")
		})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if atomic.LoadInt32(&attempts) > 2 {
			t.Errorf("expected at most 2 attempts before cancellation, got %d", attempts)
		}
	})

	t.Run("applies exponential backoff intervals", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         50 * time.Millisecond,
			Multiplier:           2.0,
			JitterFactor:         0.0, // No jitter for predictable timing
			RetryableStatusCodes: []int{429},
			Logger:               slog.Default(),
		})

		var timestamps []time.Time
		_ = retryer.Do(context.Background(), func(ctx context.Context) (int, error) {
			timestamps = append(timestamps, time.Now())
			return http.StatusTooManyRequests, errors.New("rate limited")
		})

		if len(timestamps) < 4 {
			t.Fatalf("expected 4 attempts, got %d", len(timestamps))
		}

		// Check intervals are approximately exponential
		// Attempt 1 -> 2: ~50ms
		// Attempt 2 -> 3: ~100ms
		// Attempt 3 -> 4: ~200ms

		expectedIntervals := []time.Duration{
			50 * time.Millisecond,
			100 * time.Millisecond,
			200 * time.Millisecond,
		}

		for i := 0; i < len(timestamps)-1; i++ {
			interval := timestamps[i+1].Sub(timestamps[i])
			expected := expectedIntervals[i]
			tolerance := expected / 2 // 50% tolerance for timing variations

			if interval < expected-tolerance || interval > expected+tolerance {
				t.Errorf("interval %d: expected ~%v, got %v", i+1, expected, interval)
			}
		}
	})
}

func TestDoWithResult(t *testing.T) {
	t.Run("returns result on success", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:   3,
			InitialDelay: 10 * time.Millisecond,
			Logger:       slog.Default(),
		})

		result, err := DoWithResult(context.Background(), retryer, func(ctx context.Context) (string, int, error) {
			return "success", http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got '%s'", result)
		}
	})

	t.Run("returns result after retries", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           3,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{429},
			Logger:               slog.Default(),
		})

		var attempts int
		result, err := DoWithResult(context.Background(), retryer, func(ctx context.Context) (int, int, error) {
			attempts++
			if attempts < 3 {
				return 0, http.StatusTooManyRequests, errors.New("rate limited")
			}
			return 42, http.StatusOK, nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected result 42, got %d", result)
		}
	})

	t.Run("returns error after max retries", func(t *testing.T) {
		retryer := New(Config{
			MaxRetries:           2,
			InitialDelay:         10 * time.Millisecond,
			RetryableStatusCodes: []int{500},
			Logger:               slog.Default(),
		})

		result, err := DoWithResult(context.Background(), retryer, func(ctx context.Context) (string, int, error) {
			return "partial", http.StatusInternalServerError, errors.New("server error")
		})

		if err == nil {
			t.Error("expected error after max retries")
		}
		if result != "partial" {
			t.Errorf("expected last result 'partial', got '%s'", result)
		}
	})
}

func TestRetryableError(t *testing.T) {
	originalErr := errors.New("original error")
	retryErr := &RetryableError{
		StatusCode: 429,
		Err:        originalErr,
		Attempt:    3,
	}

	t.Run("Error returns message", func(t *testing.T) {
		if retryErr.Error() != "original error" {
			t.Errorf("expected 'original error', got '%s'", retryErr.Error())
		}
	})

	t.Run("Unwrap returns original error", func(t *testing.T) {
		if retryErr.Unwrap() != originalErr {
			t.Error("Unwrap should return original error")
		}
	})

	t.Run("errors.Is works with wrapped errors", func(t *testing.T) {
		if !errors.Is(retryErr, originalErr) {
			t.Error("errors.Is should find original error")
		}
	})
}

func TestRetryer_RetryableStatusCodes(t *testing.T) {
	t.Run("returns configured status codes", func(t *testing.T) {
		retryer := New(Config{
			RetryableStatusCodes: []int{429, 503},
		})

		codes := retryer.RetryableStatusCodes()
		if len(codes) != 2 {
			t.Errorf("expected 2 status codes, got %d", len(codes))
		}

		hasExpected := make(map[int]bool)
		for _, code := range codes {
			hasExpected[code] = true
		}

		if !hasExpected[429] || !hasExpected[503] {
			t.Errorf("expected [429, 503], got %v", codes)
		}
	})
}
