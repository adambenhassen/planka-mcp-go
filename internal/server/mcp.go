package server

import (
	"context"
	"encoding/json"
)

// protocolVersion is the MCP protocol version advertised when a client does not
// request one.
const protocolVersion = "2024-11-05"

// rpcRequest is an incoming JSON-RPC 2.0 request or notification.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcError is a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// rpcResponse is an outgoing JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// contentItem is a single content block in a tools/call result.
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// callToolResult is the payload returned from a tools/call request.
type callToolResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// handleMessage processes a single JSON-RPC message and returns the marshaled
// response. isNotification is true for messages that carry no id and therefore
// receive no response.
func (s *Server) handleMessage(ctx context.Context, raw []byte) (response []byte, isNotification bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.marshalResponse(rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		}), false
	}

	// A message without an id is a notification: dispatch side effects (none
	// currently) and return no response.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil, true
	}

	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		resp.Result = s.initializeResult(req.Params)
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.tools}
	case "tools/call":
		result, rpcErr := s.callTool(ctx, req.Params)
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "Method not found: " + req.Method}
	}

	return s.marshalResponse(resp), false
}

// marshalResponse serializes a JSON-RPC response, logging on failure.
func (s *Server) marshalResponse(resp rpcResponse) []byte {
	out, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error("failed to marshal response", "error", err)
		return nil
	}
	return out
}

// initializeResult builds the initialize response, echoing the client's
// requested protocol version when present.
func (s *Server) initializeResult(params json.RawMessage) map[string]any {
	version := protocolVersion
	if len(params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(params, &p); err == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion
		}
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "planka-mcp", "version": serverVersion},
	}
}

// callTool dispatches a tools/call request to the Planka API and shapes the
// result content.
func (s *Server) callTool(ctx context.Context, params json.RawMessage) (*callToolResult, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params: " + err.Error()}
	}

	def, ok := s.toolMap[p.Name]
	if !ok {
		return &callToolResult{
			Content: []contentItem{{Type: "text", Text: "Unknown tool: " + p.Name}},
			IsError: true,
		}, nil
	}

	result := s.ExecuteGroupedAPICall(ctx, def, p.Arguments)
	if result.Success {
		return &callToolResult{
			Content: []contentItem{{Type: "text", Text: stringifyResult(result.Data)}},
		}, nil
	}

	text := result.Err
	if text == "" {
		text = "Unknown error"
	}
	return &callToolResult{
		Content: []contentItem{{Type: "text", Text: text}},
		IsError: true,
	}, nil
}

// stringifyResult renders successful response data: raw strings pass through,
// everything else is pretty-printed JSON with two-space indentation.
func stringifyResult(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}
