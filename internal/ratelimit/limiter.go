// Package ratelimit provides rate limiting middleware using a token bucket algorithm.
package ratelimit

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Config holds rate limiter configuration.
type Config struct {
	// RequestsPerSecond is the rate limit (tokens added per second).
	RequestsPerSecond float64
	// BurstSize is the maximum number of tokens (burst capacity).
	BurstSize int
	// Logger for rate limit events.
	Logger *slog.Logger
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		Logger:            slog.Default(),
	}
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// NewTokenBucket creates a new token bucket with the specified rate and burst size.
func NewTokenBucket(refillRate float64, burstSize int) *TokenBucket {
	return &TokenBucket{
		tokens:         float64(burstSize), // Start full
		maxTokens:      float64(burstSize),
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so.
// Returns whether the request is allowed, remaining tokens, and retry-after duration.
func (tb *TokenBucket) Allow() (allowed bool, remaining int, retryAfter time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime)
	tb.tokens = math.Min(tb.maxTokens, tb.tokens+tb.refillRate*elapsed.Seconds())
	tb.lastRefillTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true, int(tb.tokens), 0
	}

	// Calculate time until a token is available
	tokensNeeded := 1 - tb.tokens
	retryAfter = time.Duration(tokensNeeded/tb.refillRate*float64(time.Second)) + time.Millisecond
	return false, 0, retryAfter
}

// Remaining returns the current number of available tokens.
func (tb *TokenBucket) Remaining() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime)
	tb.tokens = math.Min(tb.maxTokens, tb.tokens+tb.refillRate*elapsed.Seconds())
	tb.lastRefillTime = now

	return int(tb.tokens)
}

// Limit returns the maximum burst size.
func (tb *TokenBucket) Limit() int {
	return int(tb.maxTokens)
}

// Rate returns the refill rate (tokens per second).
func (tb *TokenBucket) Rate() float64 {
	return tb.refillRate
}

// Limiter provides rate limiting middleware with per-endpoint support.
type Limiter struct {
	config         Config
	globalBucket   *TokenBucket
	endpointBuckets map[string]*TokenBucket
	mu             sync.RWMutex
}

// New creates a new rate limiter with the given configuration.
func New(config Config) *Limiter {
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = 10.0
	}
	if config.BurstSize <= 0 {
		config.BurstSize = 20
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Limiter{
		config:          config,
		globalBucket:    NewTokenBucket(config.RequestsPerSecond, config.BurstSize),
		endpointBuckets: make(map[string]*TokenBucket),
	}
}

// SetEndpointLimit sets a specific rate limit for an endpoint.
func (l *Limiter) SetEndpointLimit(endpoint string, requestsPerSecond float64, burstSize int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.endpointBuckets[endpoint] = NewTokenBucket(requestsPerSecond, burstSize)
}

// RemoveEndpointLimit removes the specific rate limit for an endpoint (falls back to global).
func (l *Limiter) RemoveEndpointLimit(endpoint string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.endpointBuckets, endpoint)
}

// GetEndpointLimits returns a copy of all configured endpoint limits.
func (l *Limiter) GetEndpointLimits() map[string]struct{ Rate float64; Burst int } {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]struct{ Rate float64; Burst int })
	for endpoint, bucket := range l.endpointBuckets {
		result[endpoint] = struct{ Rate float64; Burst int }{
			Rate:  bucket.Rate(),
			Burst: bucket.Limit(),
		}
	}
	return result
}

// getBucket returns the appropriate bucket for the endpoint.
func (l *Limiter) getBucket(endpoint string) *TokenBucket {
	l.mu.RLock()
	if bucket, ok := l.endpointBuckets[endpoint]; ok {
		l.mu.RUnlock()
		return bucket
	}
	l.mu.RUnlock()
	return l.globalBucket
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (l *Limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucket := l.getBucket(r.URL.Path)

		allowed, remaining, retryAfter := bucket.Allow()

		// Always set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(bucket.Limit()))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

		if !allowed {
			l.config.Logger.Warn("rate limit exceeded",
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Duration("retry_after", retryAfter),
			)

			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error":       "rate limit exceeded",
				"retry_after": int(math.Ceil(retryAfter.Seconds())),
			})
			return
		}

		next(w, r)
	}
}

// GlobalRemaining returns the remaining tokens in the global bucket.
func (l *Limiter) GlobalRemaining() int {
	return l.globalBucket.Remaining()
}

// GlobalLimit returns the global burst limit.
func (l *Limiter) GlobalLimit() int {
	return l.globalBucket.Limit()
}

// GlobalRate returns the global refill rate.
func (l *Limiter) GlobalRate() float64 {
	return l.globalBucket.Rate()
}
