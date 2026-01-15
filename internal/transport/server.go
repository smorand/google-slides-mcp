package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	defaultPort            = 8080
	defaultReadTimeout     = 30 * time.Second
	defaultWriteTimeout    = 60 * time.Second
	defaultIdleTimeout     = 120 * time.Second
	defaultShutdownTimeout = 30 * time.Second
)

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
	Logger          *slog.Logger
}

// DefaultServerConfig returns configuration with default values.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:            defaultPort,
		ReadTimeout:     defaultReadTimeout,
		WriteTimeout:    defaultWriteTimeout,
		IdleTimeout:     defaultIdleTimeout,
		ShutdownTimeout: defaultShutdownTimeout,
		AllowedOrigins:  []string{"*"},
		Logger:          slog.Default(),
	}
}

// AuthHandler is the interface for OAuth authentication handlers.
type AuthHandler interface {
	HandleAuth(w http.ResponseWriter, r *http.Request)
	HandleCallback(w http.ResponseWriter, r *http.Request)
}

// APIKeyMiddleware is the interface for API key validation middleware.
type APIKeyMiddleware interface {
	Middleware(next http.HandlerFunc) http.HandlerFunc
}

// RateLimitMiddleware is the interface for rate limiting middleware.
type RateLimitMiddleware interface {
	Middleware(next http.HandlerFunc) http.HandlerFunc
}

// Server represents the HTTP streamable MCP server.
type Server struct {
	config              ServerConfig
	httpServer          *http.Server
	mux                 *http.ServeMux
	handler             *MCPHandler
	authHandler         AuthHandler
	apiKeyMiddleware    APIKeyMiddleware
	rateLimitMiddleware RateLimitMiddleware
	logger              *slog.Logger
	mu                  sync.RWMutex
	running             bool
}

// NewServer creates a new MCP HTTP server.
func NewServer(config ServerConfig) *Server {
	if config.Port == 0 {
		config.Port = defaultPort
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = defaultReadTimeout
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = defaultWriteTimeout
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = defaultIdleTimeout
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = defaultShutdownTimeout
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}

	s := &Server{
		config:  config,
		mux:     http.NewServeMux(),
		handler: NewMCPHandler(config.Logger),
		logger:  config.Logger,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Health check endpoint (no auth required)
	s.mux.HandleFunc("/health", s.handleHealth)

	// MCP protocol endpoint (POST for tool calls) - requires API key
	s.mux.HandleFunc("/mcp", s.withMiddleware(s.withAPIKeyAuth(s.handleMCP)))

	// MCP initialize endpoint - requires API key
	s.mux.HandleFunc("/mcp/initialize", s.withMiddleware(s.withAPIKeyAuth(s.handleMCPInitialize)))

	// OAuth2 authentication endpoints (only if auth handler is set) - no API key required
	if s.authHandler != nil {
		s.mux.HandleFunc("/auth", s.withMiddleware(s.handleAuth))
		s.mux.HandleFunc("/auth/callback", s.withMiddleware(s.handleAuthCallback))
	}
}

// withAPIKeyAuth wraps a handler with API key authentication.
func (s *Server) withAPIKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apiKeyMiddleware != nil {
			s.apiKeyMiddleware.Middleware(next)(w, r)
		} else {
			// No API key middleware configured, proceed without auth
			next(w, r)
		}
	}
}

// SetAuthHandler sets the OAuth authentication handler.
func (s *Server) SetAuthHandler(handler AuthHandler) {
	s.authHandler = handler
	// Re-register auth routes
	if handler != nil {
		s.mux.HandleFunc("/auth", s.withMiddleware(s.handleAuth))
		s.mux.HandleFunc("/auth/callback", s.withMiddleware(s.handleAuthCallback))
	}
}

// SetAPIKeyMiddleware sets the API key validation middleware.
func (s *Server) SetAPIKeyMiddleware(middleware APIKeyMiddleware) {
	s.apiKeyMiddleware = middleware
}

// SetRateLimitMiddleware sets the rate limiting middleware.
func (s *Server) SetRateLimitMiddleware(middleware RateLimitMiddleware) {
	s.rateLimitMiddleware = middleware
}

// handleAuth handles the /auth endpoint.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if s.authHandler == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "authentication not configured",
		})
		return
	}
	s.authHandler.HandleAuth(w, r)
}

// handleAuthCallback handles the /auth/callback endpoint.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.authHandler == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "authentication not configured",
		})
		return
	}
	s.authHandler.HandleCallback(w, r)
}

// withMiddleware wraps a handler with logging, CORS, and rate limiting middleware.
func (s *Server) withMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Apply CORS
		s.applyCORS(w, r)

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Create response wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Apply rate limiting if configured
		if s.rateLimitMiddleware != nil {
			rateLimitedHandler := s.rateLimitMiddleware.Middleware(func(w http.ResponseWriter, r *http.Request) {
				next(w, r)
			})
			rateLimitedHandler(rw, r)
		} else {
			// No rate limiting, call handler directly
			next(rw, r)
		}

		// Log the request
		s.logger.Info("request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote_addr", r.RemoteAddr),
		)
	}
}

// applyCORS applies CORS headers to the response.
func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	allowed := false
	for _, o := range s.config.AllowedOrigins {
		if o == "*" || o == origin {
			allowed = true
			break
		}
	}

	if allowed {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
}

// handleHealth handles the /health endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// handleMCP handles MCP tool call requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	s.handler.HandleToolCall(w, r)
}

// handleMCPInitialize handles the MCP initialize handshake.
func (s *Server) handleMCPInitialize(w http.ResponseWriter, r *http.Request) {
	s.handler.HandleInitialize(w, r)
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	s.logger.Info("starting MCP server",
		slog.Int("port", s.config.Port),
	)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("server failed to start: %w", err)
	case <-ctx.Done():
		return s.Shutdown()
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	s.logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.logger.Info("server shutdown complete")
	return nil
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Port returns the configured port.
func (s *Server) Port() int {
	return s.config.Port
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before writing.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
