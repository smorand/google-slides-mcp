package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPInitialize(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp/initialize", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleInitialize(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	if resp.ID != float64(1) { // JSON numbers are float64
		t.Errorf("ID = %v, want 1", resp.ID)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}

	if result["protocolVersion"] != MCPProtocolVersion {
		t.Errorf("protocolVersion = %v, want %s", result["protocolVersion"], MCPProtocolVersion)
	}

	if !h.IsInitialized() {
		t.Error("handler should be initialized")
	}
}

func TestMCPInitializeWrongMethod(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "wrong_method",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp/initialize", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleInitialize(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != ErrorCodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeMethodNotFound)
	}
}

func TestToolCallWithoutInitialize(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != ErrorCodeInvalidRequest {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeInvalidRequest)
	}
}

func TestToolsList(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize first
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools is not an array: %T", result["tools"])
	}

	// Currently empty, but should be an array
	if len(tools) != 0 {
		t.Errorf("tools length = %d, want 0", len(tools))
	}
}

func TestToolsCall(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize first
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "unknown_tool", "arguments": {}}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}

	if result["isError"] != true {
		t.Error("expected isError to be true for unknown tool")
	}
}

func TestUnknownMethod(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize first
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != ErrorCodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeMethodNotFound)
	}
}

func TestInvalidJSON(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp/initialize", strings.NewReader("invalid json"))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleInitialize(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != ErrorCodeParse {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeParse)
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := JSONRPCRequest{
		JSONRPC: "1.0", // Wrong version
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp/initialize", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleInitialize(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChunkedTransferEncoding(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize first
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	if w.Header().Get("Transfer-Encoding") != "chunked" {
		t.Errorf("Transfer-Encoding = %q, want chunked", w.Header().Get("Transfer-Encoding"))
	}
}

func TestHandlerReset(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	if !h.IsInitialized() {
		t.Error("handler should be initialized")
	}

	// Reset
	h.Reset()

	if h.IsInitialized() {
		t.Error("handler should not be initialized after reset")
	}
}

func TestNewMCPHandlerWithNilLogger(t *testing.T) {
	h := NewMCPHandler(nil)
	if h.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestInvalidInitializeParams(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`"invalid"`), // Should be object, not string
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp/initialize", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleInitialize(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for invalid params, got nil")
	}

	if resp.Error.Code != ErrorCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeInvalidParams)
	}
}

func TestToolsCallInvalidParams(t *testing.T) {
	h := NewMCPHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Initialize first
	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`"invalid"`), // Should be object, not string
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleToolCall(w, httpReq)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for invalid params, got nil")
	}

	if resp.Error.Code != ErrorCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrorCodeInvalidParams)
	}
}
