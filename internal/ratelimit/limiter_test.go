package ratelimit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		bucket := NewTokenBucket(10.0, 5) // 10 req/s, burst of 5

		// Should allow 5 requests (burst size)
		for i := range 5 {
			allowed, remaining, retryAfter := bucket.Allow()
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
			if remaining != 4-i {
				t.Errorf("expected remaining %d, got %d", 4-i, remaining)
			}
			if retryAfter != 0 {
				t.Errorf("expected no retry delay, got %v", retryAfter)
			}
		}
	})

	t.Run("blocks requests when exhausted", func(t *testing.T) {
		bucket := NewTokenBucket(10.0, 2) // 10 req/s, burst of 2

		// Exhaust tokens
		bucket.Allow()
		bucket.Allow()

		// Next request should be blocked
		allowed, remaining, retryAfter := bucket.Allow()
		if allowed {
			t.Error("request should be blocked when tokens exhausted")
		}
		if remaining != 0 {
			t.Errorf("expected remaining 0, got %d", remaining)
		}
		if retryAfter <= 0 {
			t.Error("expected positive retry delay")
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		bucket := NewTokenBucket(100.0, 2) // 100 req/s (fast for testing)

		// Exhaust tokens
		bucket.Allow()
		bucket.Allow()

		// Wait for refill (at least 10ms for 1 token at 100/s)
		time.Sleep(15 * time.Millisecond)

		// Should have at least 1 token now
		allowed, _, _ := bucket.Allow()
		if !allowed {
			t.Error("request should be allowed after token refill")
		}
	})
}

func TestTokenBucket_Remaining(t *testing.T) {
	bucket := NewTokenBucket(10.0, 5)

	if remaining := bucket.Remaining(); remaining != 5 {
		t.Errorf("expected 5 remaining, got %d", remaining)
	}

	bucket.Allow()
	bucket.Allow()

	if remaining := bucket.Remaining(); remaining != 3 {
		t.Errorf("expected 3 remaining, got %d", remaining)
	}
}

func TestTokenBucket_Limit(t *testing.T) {
	bucket := NewTokenBucket(10.0, 15)
	if limit := bucket.Limit(); limit != 15 {
		t.Errorf("expected limit 15, got %d", limit)
	}
}

func TestTokenBucket_Rate(t *testing.T) {
	bucket := NewTokenBucket(25.5, 10)
	if rate := bucket.Rate(); rate != 25.5 {
		t.Errorf("expected rate 25.5, got %f", rate)
	}
}

func TestLimiter_New(t *testing.T) {
	t.Run("uses default values when config is empty", func(t *testing.T) {
		limiter := New(Config{})

		if limiter.GlobalRate() != 10.0 {
			t.Errorf("expected default rate 10.0, got %f", limiter.GlobalRate())
		}
		if limiter.GlobalLimit() != 20 {
			t.Errorf("expected default burst 20, got %d", limiter.GlobalLimit())
		}
	})

	t.Run("uses provided configuration", func(t *testing.T) {
		limiter := New(Config{
			RequestsPerSecond: 50.0,
			BurstSize:         100,
		})

		if limiter.GlobalRate() != 50.0 {
			t.Errorf("expected rate 50.0, got %f", limiter.GlobalRate())
		}
		if limiter.GlobalLimit() != 100 {
			t.Errorf("expected burst 100, got %d", limiter.GlobalLimit())
		}
	})
}

func TestLimiter_SetEndpointLimit(t *testing.T) {
	limiter := New(DefaultConfig())

	// Set endpoint-specific limit
	limiter.SetEndpointLimit("/api/heavy", 2.0, 5)

	limits := limiter.GetEndpointLimits()
	if len(limits) != 1 {
		t.Errorf("expected 1 endpoint limit, got %d", len(limits))
	}

	limit, ok := limits["/api/heavy"]
	if !ok {
		t.Error("endpoint limit not found")
	}
	if limit.Rate != 2.0 || limit.Burst != 5 {
		t.Errorf("expected rate 2.0 burst 5, got rate %f burst %d", limit.Rate, limit.Burst)
	}
}

func TestLimiter_RemoveEndpointLimit(t *testing.T) {
	limiter := New(DefaultConfig())

	limiter.SetEndpointLimit("/api/test", 5.0, 10)
	limiter.RemoveEndpointLimit("/api/test")

	limits := limiter.GetEndpointLimits()
	if len(limits) != 0 {
		t.Errorf("expected 0 endpoint limits after removal, got %d", len(limits))
	}
}

func TestLimiter_Middleware(t *testing.T) {
	t.Run("allows requests and sets headers", func(t *testing.T) {
		limiter := New(Config{
			RequestsPerSecond: 10.0,
			BurstSize:         5,
			Logger:            slog.Default(),
		})

		handler := limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		// Check rate limit headers
		if rec.Header().Get("X-RateLimit-Limit") != "5" {
			t.Errorf("expected X-RateLimit-Limit 5, got %s", rec.Header().Get("X-RateLimit-Limit"))
		}
		if rec.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("expected X-RateLimit-Remaining header")
		}
		if rec.Header().Get("X-RateLimit-Reset") == "" {
			t.Error("expected X-RateLimit-Reset header")
		}
	})

	t.Run("returns 429 when rate limit exceeded", func(t *testing.T) {
		limiter := New(Config{
			RequestsPerSecond: 10.0,
			BurstSize:         2,
			Logger:            slog.Default(),
		})

		handler := limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Exhaust tokens
		for range 2 {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			handler(rec, req)
		}

		// Next request should be rate limited
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}

		// Check Retry-After header
		retryAfter := rec.Header().Get("Retry-After")
		if retryAfter == "" {
			t.Error("expected Retry-After header")
		}
		if seconds, err := strconv.Atoi(retryAfter); err != nil || seconds < 1 {
			t.Errorf("expected Retry-After to be at least 1 second, got %s", retryAfter)
		}

		// Check response body
		var response map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response["error"] != "rate limit exceeded" {
			t.Errorf("expected error message 'rate limit exceeded', got %v", response["error"])
		}
	})

	t.Run("applies per-endpoint limits", func(t *testing.T) {
		limiter := New(Config{
			RequestsPerSecond: 100.0, // High global limit
			BurstSize:         100,
			Logger:            slog.Default(),
		})

		// Set strict limit for specific endpoint
		limiter.SetEndpointLimit("/api/limited", 10.0, 1)

		handler := limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// First request to limited endpoint should succeed
		req1 := httptest.NewRequest(http.MethodGet, "/api/limited", nil)
		rec1 := httptest.NewRecorder()
		handler(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Errorf("expected first request to succeed, got %d", rec1.Code)
		}

		// Second request to limited endpoint should be rate limited
		req2 := httptest.NewRequest(http.MethodGet, "/api/limited", nil)
		rec2 := httptest.NewRecorder()
		handler(rec2, req2)
		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("expected second request to be rate limited, got %d", rec2.Code)
		}

		// Request to other endpoint should still succeed (global limit)
		req3 := httptest.NewRequest(http.MethodGet, "/api/other", nil)
		rec3 := httptest.NewRecorder()
		handler(rec3, req3)
		if rec3.Code != http.StatusOK {
			t.Errorf("expected request to other endpoint to succeed, got %d", rec3.Code)
		}
	})
}

func TestLimiter_RateLimitHeaders(t *testing.T) {
	limiter := New(Config{
		RequestsPerSecond: 10.0,
		BurstSize:         10,
		Logger:            slog.Default(),
	})

	handler := limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Verify all rate limit headers are present
	headers := []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"}
	for _, header := range headers {
		if rec.Header().Get(header) == "" {
			t.Errorf("missing header: %s", header)
		}
	}

	// Verify X-RateLimit-Limit value
	if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "10" {
		t.Errorf("expected X-RateLimit-Limit 10, got %s", limit)
	}

	// Verify X-RateLimit-Remaining is decremented
	remaining, _ := strconv.Atoi(rec.Header().Get("X-RateLimit-Remaining"))
	if remaining != 9 {
		t.Errorf("expected X-RateLimit-Remaining 9, got %d", remaining)
	}

	// Verify X-RateLimit-Reset is a valid timestamp
	reset, err := strconv.ParseInt(rec.Header().Get("X-RateLimit-Reset"), 10, 64)
	if err != nil {
		t.Errorf("X-RateLimit-Reset is not a valid timestamp: %v", err)
	}
	if reset < time.Now().Unix() {
		t.Error("X-RateLimit-Reset should be in the future")
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := New(Config{
		RequestsPerSecond: 1000.0, // High rate to avoid blocking
		BurstSize:         100,
		Logger:            slog.Default(),
	})

	handler := limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Run concurrent requests
	var wg sync.WaitGroup
	numRequests := 50

	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			handler(rec, req)
		}()
	}

	wg.Wait()

	// Verify remaining tokens decreased
	remaining := limiter.GlobalRemaining()
	if remaining > 100-numRequests {
		t.Errorf("expected remaining <= %d, got %d", 100-numRequests, remaining)
	}
}

func TestLimiter_ConfigurationAdjustments(t *testing.T) {
	t.Run("adjust rate limit via endpoint configuration", func(t *testing.T) {
		limiter := New(Config{
			RequestsPerSecond: 10.0,
			BurstSize:         5,
			Logger:            slog.Default(),
		})

		// Initially no endpoint limits
		limits := limiter.GetEndpointLimits()
		if len(limits) != 0 {
			t.Errorf("expected no endpoint limits initially")
		}

		// Add multiple endpoint limits
		limiter.SetEndpointLimit("/api/v1/slow", 1.0, 2)
		limiter.SetEndpointLimit("/api/v1/fast", 100.0, 50)
		limiter.SetEndpointLimit("/api/v1/medium", 10.0, 10)

		limits = limiter.GetEndpointLimits()
		if len(limits) != 3 {
			t.Errorf("expected 3 endpoint limits, got %d", len(limits))
		}

		// Verify each limit
		if limits["/api/v1/slow"].Rate != 1.0 || limits["/api/v1/slow"].Burst != 2 {
			t.Error("slow endpoint limit incorrect")
		}
		if limits["/api/v1/fast"].Rate != 100.0 || limits["/api/v1/fast"].Burst != 50 {
			t.Error("fast endpoint limit incorrect")
		}
		if limits["/api/v1/medium"].Rate != 10.0 || limits["/api/v1/medium"].Burst != 10 {
			t.Error("medium endpoint limit incorrect")
		}

		// Remove one limit
		limiter.RemoveEndpointLimit("/api/v1/medium")
		limits = limiter.GetEndpointLimits()
		if len(limits) != 2 {
			t.Errorf("expected 2 endpoint limits after removal, got %d", len(limits))
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RequestsPerSecond != 10.0 {
		t.Errorf("expected default RequestsPerSecond 10.0, got %f", config.RequestsPerSecond)
	}
	if config.BurstSize != 20 {
		t.Errorf("expected default BurstSize 20, got %d", config.BurstSize)
	}
	if config.Logger == nil {
		t.Error("expected default Logger to be set")
	}
}
