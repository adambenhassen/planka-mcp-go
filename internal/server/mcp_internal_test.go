package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// internalTestServer builds a Server pointed at baseURL with API-key auth and
// all tools enabled, for exercising unexported protocol handlers.
func internalTestServer(baseURL string) *Server {
	return New(Config{
		BaseURL:        baseURL,
		APIKey:         "test-key",
		MaxRetries:     0,
		RetryBaseDelay: time.Millisecond,
		EnableAll:      true,
	})
}

// decodeResponse unmarshals a marshaled JSON-RPC response for assertions.
func decodeResponse(t *testing.T, raw []byte) rpcResponse {
	t.Helper()
	var resp rpcResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("response not valid JSON: %v (%s)", err, raw)
	}
	return resp
}

// asMapT asserts that v is a JSON object.
func asMapT(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("value %v is not an object", v)
	}
	return m
}

// asSliceT asserts that v is a JSON array.
func asSliceT(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("value %v is not an array", v)
	}
	return s
}

// asStrT asserts that v is a JSON string.
func asStrT(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("value %v is not a string", v)
	}
	return s
}

func TestHandleMessageParseError(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, isNotification := s.handleMessage(t.Context(), []byte("{not json"))
	if isNotification {
		t.Fatal("parse error must not be treated as a notification")
	}
	resp := decodeResponse(t, raw)
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("want parse error -32700, got %+v", resp.Error)
	}
	if string(resp.ID) != "null" {
		t.Errorf("parse error id = %s, want null", resp.ID)
	}
}

func TestHandleMessageNotification(t *testing.T) {
	s := internalTestServer("http://unused.example")
	// No id member → notification, no response.
	raw, isNotification := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","method":"ping"}`))
	if !isNotification {
		t.Fatal("message without id must be a notification")
	}
	if raw != nil {
		t.Errorf("notification must produce no response, got %s", raw)
	}
}

func TestHandleMessageIDNullIsRequest(t *testing.T) {
	s := internalTestServer("http://unused.example")
	// id:null is a request (present id) and must get a response.
	raw, isNotification := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`))
	if isNotification {
		t.Fatal("id:null is a request, not a notification")
	}
	resp := decodeResponse(t, raw)
	if string(resp.ID) != "null" {
		t.Errorf("id = %s, want null", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("ping should not error: %+v", resp.Error)
	}
}

func TestHandleMessagePing(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	resp := decodeResponse(t, raw)
	if resp.Error != nil {
		t.Fatalf("ping errored: %+v", resp.Error)
	}
	if m := asMapT(t, resp.Result); len(m) != 0 {
		t.Errorf("ping result = %v, want empty object", resp.Result)
	}
}

func TestHandleMessageInitializeDefaultVersion(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	resp := decodeResponse(t, raw)
	m := asMapT(t, resp.Result)
	if m["protocolVersion"] != protocolVersion {
		t.Errorf("protocolVersion = %v, want %s", m["protocolVersion"], protocolVersion)
	}
	info := asMapT(t, m["serverInfo"])
	if info["name"] != "planka-mcp" || info["version"] != serverVersion {
		t.Errorf("serverInfo = %v, want planka-mcp/%s", info, serverVersion)
	}
}

func TestHandleMessageInitializeEchoesClientVersion(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	resp := decodeResponse(t, raw)
	m := asMapT(t, resp.Result)
	if m["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want echoed 2025-06-18", m["protocolVersion"])
	}
}

func TestHandleMessageToolsList(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	resp := decodeResponse(t, raw)
	m := asMapT(t, resp.Result)
	if list := asSliceT(t, m["tools"]); len(list) != len(s.tools) {
		t.Errorf("tools/list returned %d tools, want %d", len(list), len(s.tools))
	}
}

func TestHandleMessageMethodNotFound(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":1,"method":"nope"}`))
	resp := decodeResponse(t, raw)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("want method-not-found -32601, got %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "nope") {
		t.Errorf("error should name the method, got %q", resp.Error.Message)
	}
}

func TestCallToolInvalidParams(t *testing.T) {
	s := internalTestServer("http://unused.example")
	// arguments is a string, not the expected object → unmarshal error.
	raw, _ := s.handleMessage(t.Context(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"oops"}`))
	resp := decodeResponse(t, raw)
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("want invalid-params -32602, got %+v", resp.Error)
	}
}

func TestCallToolUnknownTool(t *testing.T) {
	s := internalTestServer("http://unused.example")
	raw, _ := s.handleMessage(t.Context(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ghost","arguments":{}}}`))
	resp := decodeResponse(t, raw)
	if resp.Error != nil {
		t.Fatalf("unknown tool should be a result with isError, not a protocol error: %+v", resp.Error)
	}
	res := asMapT(t, resp.Result)
	if res["isError"] != true {
		t.Errorf("unknown tool result should set isError, got %v", res)
	}
	content := asMapT(t, asSliceT(t, res["content"])[0])
	if !strings.Contains(asStrT(t, content["text"]), "ghost") {
		t.Errorf("content should name the tool, got %v", content)
	}
}

func TestCallToolSuccess(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"item":{"id":"7"}}`); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	t.Cleanup(backend.Close)

	s := internalTestServer(backend.URL)
	raw, _ := s.handleMessage(t.Context(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects","arguments":{"action":"get","id":"7"}}}`))
	resp := decodeResponse(t, raw)
	if resp.Error != nil {
		t.Fatalf("call errored: %+v", resp.Error)
	}
	res := asMapT(t, resp.Result)
	if res["isError"] == true {
		t.Fatalf("expected success result, got %v", res)
	}
	text := asStrT(t, asMapT(t, asSliceT(t, res["content"])[0])["text"])
	if !strings.Contains(text, `"id": "7"`) {
		t.Errorf("result text should be pretty-printed JSON, got %q", text)
	}
}

func TestCallToolAPIError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(backend.Close)

	s := internalTestServer(backend.URL)
	raw, _ := s.handleMessage(t.Context(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"projects","arguments":{"action":"get","id":"1"}}}`))
	resp := decodeResponse(t, raw)
	res := asMapT(t, resp.Result)
	if res["isError"] != true {
		t.Errorf("API failure should set isError, got %v", res)
	}
}

func TestStringifyResult(t *testing.T) {
	if got := stringifyResult("raw"); got != "raw" {
		t.Errorf("string should pass through, got %q", got)
	}
	got := stringifyResult(map[string]any{"a": 1})
	if !strings.Contains(got, "\n  \"a\"") {
		t.Errorf("non-string should be indented JSON, got %q", got)
	}
}

func TestMarshalResponseUnmarshalable(t *testing.T) {
	s := internalTestServer("http://unused.example")
	// A channel cannot be marshaled; marshalResponse must return nil, not panic.
	if out := s.marshalResponse(rpcResponse{Result: make(chan int)}); out != nil {
		t.Errorf("unmarshalable response should yield nil, got %s", out)
	}
}
