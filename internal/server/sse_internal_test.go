package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSessionID(t *testing.T) {
	a, b := newSessionID(), newSessionID()
	if len(a) != 32 {
		t.Errorf("session id length = %d, want 32 hex chars", len(a))
	}
	if a == b {
		t.Error("session ids must be unique")
	}
}

func TestSSESessionsRegistry(t *testing.T) {
	reg := &sseSessions{m: make(map[string]*sseSession)}
	if reg.count() != 0 {
		t.Fatalf("new registry count = %d, want 0", reg.count())
	}
	sess := &sseSession{id: "a"}
	reg.add(sess)
	if got, ok := reg.get("a"); !ok || got != sess {
		t.Fatal("get should return the added session")
	}
	if _, ok := reg.get("missing"); ok {
		t.Error("get of unknown id should report not found")
	}
	if reg.count() != 1 {
		t.Errorf("count after add = %d, want 1", reg.count())
	}
	reg.remove("a")
	if reg.count() != 0 {
		t.Errorf("count after remove = %d, want 0", reg.count())
	}
}

func TestSessionWriteAfterCloseReturnsFalse(t *testing.T) {
	rec := httptest.NewRecorder()
	sess := &sseSession{id: "s", writer: rec, flusher: rec}
	if !sess.send("message", "hi") {
		t.Fatal("send on open session should succeed")
	}
	sess.mu.Lock()
	sess.closed = true
	sess.mu.Unlock()
	if sess.send("message", "hi") {
		t.Error("send after close should return false")
	}
	if sess.comment("beat") {
		t.Error("comment after close should return false")
	}
}

func TestWithCORS(t *testing.T) {
	called := false
	h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// OPTIONS preflight is short-circuited with 204 and never reaches next.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/sse", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", rec.Code)
	}
	if called {
		t.Error("OPTIONS must not invoke the wrapped handler")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS origin header not set")
	}

	// Non-preflight requests pass through.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sse", nil))
	if !called || rec.Code != http.StatusOK {
		t.Error("non-OPTIONS request should reach the wrapped handler")
	}
}

func TestHandleHealthAllEnabled(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	s.handleHealth(reg)(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}
	var body struct {
		Status         string `json:"status"`
		ToolsAvailable int    `json:"toolsAvailable"`
		ToolCounts     struct {
			Admin    map[string]int `json:"admin"`
			Optional map[string]int `json:"optional"`
		} `json:"toolCounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body not JSON: %v", err)
	}
	if body.Status != "ok" || body.ToolsAvailable != len(s.tools) {
		t.Errorf("health body = %+v", body)
	}
	if body.ToolCounts.Admin["tools"] == 0 || body.ToolCounts.Optional["tools"] == 0 {
		t.Error("admin/optional counts should be non-zero when all tools enabled")
	}
}

func TestHandleHealthGatesDisabledCategories(t *testing.T) {
	// Core only: admin/optional counts must be reported as zero.
	s := New(Config{BaseURL: "http://unused.example", APIKey: "k"})
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	s.handleHealth(reg)(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	var body struct {
		ToolCounts struct {
			Admin    map[string]int `json:"admin"`
			Optional map[string]int `json:"optional"`
		} `json:"toolCounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body not JSON: %v", err)
	}
	if body.ToolCounts.Admin["tools"] != 0 || body.ToolCounts.Optional["tools"] != 0 {
		t.Errorf("disabled categories should report 0, got admin=%v optional=%v",
			body.ToolCounts.Admin, body.ToolCounts.Optional)
	}
}

func TestHandleMessagesRejectsWrongMethod(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	s.handleMessages(t.Context(), reg)(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/messages", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /messages = %d, want 405", rec.Code)
	}
}

func TestHandleMessagesMissingSessionID(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/messages", strings.NewReader("{}"))
	s.handleMessages(t.Context(), reg)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing sessionId = %d, want 400", rec.Code)
	}
}

func TestHandleMessagesSessionNotFound(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/messages?sessionId=ghost", strings.NewReader("{}"))
	s.handleMessages(t.Context(), reg)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown session = %d, want 404", rec.Code)
	}
}

func TestHandleMessagesRoutesResponseToSession(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	// A fake session whose stream is a buffer we can inspect.
	stream := httptest.NewRecorder()
	sess := &sseSession{id: "s1", writer: stream, flusher: stream}
	reg.add(sess)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/messages?sessionId=s1",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	s.handleMessages(t.Context(), reg)(rec, req)

	if rec.Code != http.StatusAccepted || rec.Body.String() != "Accepted" {
		t.Errorf("POST ack = %d %q, want 202 Accepted", rec.Code, rec.Body.String())
	}
	if !strings.Contains(stream.Body.String(), "event: message") {
		t.Errorf("session stream missing routed response: %q", stream.Body.String())
	}
}

func TestHandleMessagesNotificationSendsNothing(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	stream := httptest.NewRecorder()
	reg.add(&sseSession{id: "s1", writer: stream, flusher: stream})

	rec := httptest.NewRecorder()
	// No id → notification → 202 ack but no message event on the stream.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/messages?sessionId=s1",
		strings.NewReader(`{"jsonrpc":"2.0","method":"ping"}`))
	s.handleMessages(t.Context(), reg)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("notification ack = %d, want 202", rec.Code)
	}
	if stream.Body.Len() != 0 {
		t.Errorf("notification must not write to the stream, got %q", stream.Body.String())
	}
}

func TestHandleMessagesOversizeBody(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	stream := httptest.NewRecorder()
	reg.add(&sseSession{id: "s1", writer: stream, flusher: stream})

	big := strings.Repeat("a", maxMessageBytes+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/messages?sessionId=s1", strings.NewReader(big))
	s.handleMessages(t.Context(), reg)(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize body = %d, want 413", rec.Code)
	}
}

func TestHandleSSERejectsWrongMethod(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	rec := httptest.NewRecorder()
	s.handleSSE(t.Context(), reg)(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sse", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /sse = %d, want 405", rec.Code)
	}
}

// nonFlushWriter is an http.ResponseWriter that does not implement http.Flusher.
type nonFlushWriter struct{ h http.Header }

func (n nonFlushWriter) Header() http.Header         { return n.h }
func (n nonFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (nonFlushWriter) WriteHeader(int)               {}

func TestHandleSSEStreamingUnsupported(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	w := nonFlushWriter{h: make(http.Header)}
	// Should return without blocking or registering a session when the writer is
	// not a Flusher.
	s.handleSSE(t.Context(), reg)(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/sse", nil))
	if reg.count() != 0 {
		t.Error("no session should be registered when streaming is unsupported")
	}
}

// TestSSEEndToEnd drives the full SSE transport over real HTTP: connect, receive
// the endpoint event, POST a request, and read the routed response.
func TestSSEEndToEnd(t *testing.T) {
	s := internalTestServer("http://unused.example")
	reg := &sseSessions{m: make(map[string]*sseSession)}
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE(t.Context(), reg))
	mux.HandleFunc("/messages", s.handleMessages(t.Context(), reg))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("closing SSE stream: %v", cerr)
		}
	}()

	reader := bufio.NewReader(resp.Body)
	endpoint := readSSEEvent(t, reader)
	if endpoint.event != "endpoint" || !strings.HasPrefix(endpoint.data, "/messages?sessionId=") {
		t.Fatalf("first event = %+v, want endpoint announcement", endpoint)
	}
	sessionID := strings.TrimPrefix(endpoint.data, "/messages?sessionId=")

	postReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/messages?sessionId="+sessionID, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := postResp.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST status = %d, want 202", postResp.StatusCode)
	}

	msg := readSSEEvent(t, reader)
	if msg.event != "message" || !strings.Contains(msg.data, `"id":1`) {
		t.Fatalf("routed event = %+v, want ping response", msg)
	}
}

type sseEvent struct{ event, data string }

// readSSEEvent reads one "event:/data:" frame, failing on timeout.
func readSSEEvent(t *testing.T, r *bufio.Reader) sseEvent {
	t.Helper()
	type line struct {
		s   string
		err error
	}
	var ev sseEvent
	deadline := time.After(3 * time.Second)
	for {
		lineCh := make(chan line, 1)
		go func() {
			s, err := r.ReadString('\n')
			lineCh <- line{s, err}
		}()
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE event")
		case l := <-lineCh:
			if l.err != nil {
				t.Fatalf("reading SSE stream: %v", l.err)
			}
			text := strings.TrimRight(l.s, "\n")
			switch {
			case strings.HasPrefix(text, "event: "):
				ev.event = strings.TrimPrefix(text, "event: ")
			case strings.HasPrefix(text, "data: "):
				ev.data = strings.TrimPrefix(text, "data: ")
			case text == "" && ev.event != "":
				return ev
			}
		}
	}
}
