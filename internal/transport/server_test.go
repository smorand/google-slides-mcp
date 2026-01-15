package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name   string
		config ServerConfig
		want   ServerConfig
	}{
		{
			name:   "default values applied",
			config: ServerConfig{},
			want: ServerConfig{
				Port:            defaultPort,
				ReadTimeout:     defaultReadTimeout,
				WriteTimeout:    defaultWriteTimeout,
				IdleTimeout:     defaultIdleTimeout,
				ShutdownTimeout: defaultShutdownTimeout,
			},
		},
		{
			name: "custom port preserved",
			config: ServerConfig{
				Port: 9000,
			},
			want: ServerConfig{
				Port:            9000,
				ReadTimeout:     defaultReadTimeout,
				WriteTimeout:    defaultWriteTimeout,
				IdleTimeout:     defaultIdleTimeout,
				ShutdownTimeout: defaultShutdownTimeout,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(tt.config)
			if s.config.Port != tt.want.Port {
				t.Errorf("Port = %d, want %d", s.config.Port, tt.want.Port)
			}
			if s.config.ReadTimeout != tt.want.ReadTimeout {
				t.Errorf("ReadTimeout = %v, want %v", s.config.ReadTimeout, tt.want.ReadTimeout)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := NewServer(ServerConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("status = %s, want healthy", resp["status"])
	}
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		wantOrigin     string
	}{
		{
			name:           "wildcard allows any origin",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			wantOrigin:     "https://example.com",
		},
		{
			name:           "specific origin allowed",
			allowedOrigins: []string{"https://allowed.com"},
			requestOrigin:  "https://allowed.com",
			wantOrigin:     "https://allowed.com",
		},
		{
			name:           "origin not allowed",
			allowedOrigins: []string{"https://allowed.com"},
			requestOrigin:  "https://notallowed.com",
			wantOrigin:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(ServerConfig{
				AllowedOrigins: tt.allowedOrigins,
				Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
			})

			// Need to initialize first
			initReq := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
			}
			body, _ := json.Marshal(initReq)
			req := httptest.NewRequest(http.MethodPost, "/mcp/initialize", bytes.NewReader(body))
			req.Header.Set("Origin", tt.requestOrigin)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			s.mux.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
		})
	}
}

func TestPreflightRequest(t *testing.T) {
	s := NewServer(ServerConfig{
		AllowedOrigins: []string{"*"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	// Initialize first
	s.handler.mu.Lock()
	s.handler.initialized = true
	s.handler.mu.Unlock()

	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods header not set")
	}
}

func TestGracefulShutdown(t *testing.T) {
	s := NewServer(ServerConfig{
		Port:            0, // Use any available port
		ShutdownTimeout: 1 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should return after context is cancelled
	err := s.Start(ctx)
	if err != nil {
		t.Errorf("Start returned error: %v", err)
	}

	if s.IsRunning() {
		t.Error("server should not be running after shutdown")
	}
}

func TestServerIsRunning(t *testing.T) {
	s := NewServer(ServerConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if s.IsRunning() {
		t.Error("new server should not be running")
	}
}

func TestServerPort(t *testing.T) {
	s := NewServer(ServerConfig{
		Port:   9000,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if s.Port() != 9000 {
		t.Errorf("Port() = %d, want 9000", s.Port())
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Port != defaultPort {
		t.Errorf("Port = %d, want %d", config.Port, defaultPort)
	}
	if config.ReadTimeout != defaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", config.ReadTimeout, defaultReadTimeout)
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Errorf("AllowedOrigins = %v, want [*]", config.AllowedOrigins)
	}
}

func TestResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusCreated)
	}
}
