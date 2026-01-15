package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

const (
	// MCPProtocolVersion is the supported MCP protocol version.
	MCPProtocolVersion = "2024-11-05"

	// JSONRPCVersion is the JSON-RPC version used.
	JSONRPCVersion = "2.0"
)

// JSON-RPC error codes
const (
	ErrorCodeParse          = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternal       = -32603
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCPInitializeParams represents MCP initialize request params.
type MCPInitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

// ClientInfo represents client information in MCP.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult represents MCP initialize response.
type MCPInitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
}

// ServerCapabilities describes what the server can do.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
}

// ToolsCapability describes tool capabilities.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability describes resource capabilities.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo represents server information.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolCallParams represents parameters for a tool call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult represents the result of a tool call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPHandler handles MCP protocol requests.
type MCPHandler struct {
	logger      *slog.Logger
	initialized bool
	mu          sync.RWMutex
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(logger *slog.Logger) *MCPHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MCPHandler{
		logger: logger,
	}
}

// HandleInitialize handles the MCP initialize request.
func (h *MCPHandler) HandleInitialize(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := h.parseRequest(r, &req); err != nil {
		h.writeError(w, nil, ErrorCodeParse, "failed to parse request", err)
		return
	}

	if req.Method != "initialize" {
		h.writeError(w, req.ID, ErrorCodeMethodNotFound, "method not found", nil)
		return
	}

	var params MCPInitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeError(w, req.ID, ErrorCodeInvalidParams, "invalid params", err)
		return
	}

	h.logger.Info("MCP initialize",
		slog.String("client", params.ClientInfo.Name),
		slog.String("client_version", params.ClientInfo.Version),
		slog.String("protocol_version", params.ProtocolVersion),
	)

	h.mu.Lock()
	h.initialized = true
	h.mu.Unlock()

	result := MCPInitializeResult{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "google-slides-mcp",
			Version: "0.1.0",
		},
	}

	h.writeResponse(w, req.ID, result)
}

// HandleToolCall handles MCP tool call requests.
func (h *MCPHandler) HandleToolCall(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	initialized := h.initialized
	h.mu.RUnlock()

	if !initialized {
		h.writeError(w, nil, ErrorCodeInvalidRequest, "server not initialized", nil)
		return
	}

	var req JSONRPCRequest
	if err := h.parseRequest(r, &req); err != nil {
		h.writeError(w, nil, ErrorCodeParse, "failed to parse request", err)
		return
	}

	switch req.Method {
	case "tools/call":
		h.handleToolsCall(w, req)
	case "tools/list":
		h.handleToolsList(w, req)
	default:
		h.writeError(w, req.ID, ErrorCodeMethodNotFound, "method not found", nil)
	}
}

// handleToolsList returns the list of available tools.
func (h *MCPHandler) handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
	// For now, return an empty list. Tools will be added in future stories.
	result := map[string]any{
		"tools": []any{},
	}
	h.writeResponse(w, req.ID, result)
}

// handleToolsCall handles a tool call request.
func (h *MCPHandler) handleToolsCall(w http.ResponseWriter, req JSONRPCRequest) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeError(w, req.ID, ErrorCodeInvalidParams, "invalid params", err)
		return
	}

	h.logger.Info("tool call",
		slog.String("tool", params.Name),
	)

	// For now, return an error for unknown tools. Tools will be added in future stories.
	result := ToolCallResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: fmt.Sprintf("Tool '%s' not found", params.Name),
			},
		},
		IsError: true,
	}

	h.writeResponse(w, req.ID, result)
}

// parseRequest reads and parses a JSON-RPC request from the HTTP body.
func (h *MCPHandler) parseRequest(r *http.Request, req *JSONRPCRequest) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, req); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if req.JSONRPC != JSONRPCVersion {
		return fmt.Errorf("invalid JSON-RPC version: %s", req.JSONRPC)
	}

	return nil
}

// writeResponse writes a JSON-RPC success response with chunked encoding.
func (h *MCPHandler) writeResponse(w http.ResponseWriter, id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}

	h.writeJSONResponse(w, http.StatusOK, resp)
}

// writeError writes a JSON-RPC error response.
func (h *MCPHandler) writeError(w http.ResponseWriter, id any, code int, message string, data any) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	status := http.StatusOK
	if code == ErrorCodeParse || code == ErrorCodeInvalidRequest {
		status = http.StatusBadRequest
	}

	h.writeJSONResponse(w, status, resp)
}

// writeJSONResponse writes a JSON response with chunked transfer encoding.
func (h *MCPHandler) writeJSONResponse(w http.ResponseWriter, status int, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(status)

	// Use chunked encoding by writing in chunks
	flusher, canFlush := w.(http.Flusher)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(resp); err != nil {
		h.logger.Error("failed to encode response", slog.Any("error", err))
	}

	if canFlush {
		flusher.Flush()
	}
}

// IsInitialized returns whether the handler has been initialized.
func (h *MCPHandler) IsInitialized() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.initialized
}

// Reset resets the handler state.
func (h *MCPHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.initialized = false
}
