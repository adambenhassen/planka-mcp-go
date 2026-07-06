package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adambenhassen/planka-mcp-go/internal/server"
	"github.com/adambenhassen/planka-mcp-go/internal/tools"
)

// findTool returns the named tool from AllTools.
func findTool(t *testing.T, name string) tools.GroupedToolDefinition {
	t.Helper()
	for _, tool := range tools.AllTools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return tools.GroupedToolDefinition{}
}

// newTestServer builds a Server whose BaseURL points at the given test HTTP
// server, using API-key auth and all tools enabled.
func newTestServer(baseURL string) *server.Server {
	return server.New(server.Config{
		BaseURL:        baseURL,
		APIKey:         "test-key",
		MaxRetries:     2,
		RetryBaseDelay: time.Millisecond,
		EnableAll:      true,
	})
}

// capture records the requests a test HTTP server receives.
type capture struct {
	mu      sync.Mutex
	paths   []string
	queries []string
	bodies  [][]byte
}

func (c *capture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		body = nil
	}
	c.paths = append(c.paths, r.URL.Path)
	c.queries = append(c.queries, r.URL.RawQuery)
	c.bodies = append(c.bodies, body)
}

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.paths)
}

func TestPathParamFromID(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"ok":true}`); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "projects"), map[string]any{
		"action": "get",
		"id":     "42",
	})
	if !res.Success {
		t.Fatalf("expected success, got %q", res.Err)
	}
	if rec.paths[0] != "/api/projects/42" {
		t.Errorf("path = %q, want /api/projects/42", rec.paths[0])
	}
}

func TestPathParamFromData(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "labels"), map[string]any{
		"action": "removeFromCard",
		"id":     "card1",
		"data":   map[string]any{"labelId": "lbl9"},
	})
	if !res.Success {
		t.Fatalf("expected success, got %q", res.Err)
	}
	if rec.paths[0] != "/api/cards/card1/card-labels/labelId:lbl9" {
		t.Errorf("path = %q, want /api/cards/card1/card-labels/labelId:lbl9", rec.paths[0])
	}
}

func TestMissingPathParam(t *testing.T) {
	s := newTestServer("http://unused.example")
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "cards"), map[string]any{
		"action": "get",
	})
	if res.Success {
		t.Fatal("expected failure for missing path param")
	}
	if !strings.Contains(res.Err, "{id}") {
		t.Errorf("error should name {id}, got %q", res.Err)
	}
}

func TestQueryFlattening(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "cards"), map[string]any{
		"action": "list",
		"id":     "list1",
		"query": map[string]any{
			"search":  "todo",
			"userIds": []any{"u1", "u2"},
			"before":  map[string]any{"id": "c9", "listChangedAt": "2020"},
		},
	})
	if !res.Success {
		t.Fatalf("expected success, got %q", res.Err)
	}
	q := rec.queries[0]
	for _, want := range []string{"search=todo", "userIds=u1", "userIds=u2", "before%5Bid%5D=c9", "before%5BlistChangedAt%5D=2020"} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q missing %q", q, want)
		}
	}
}

func TestJSONBodySent(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "cards"), map[string]any{
		"action": "create",
		"id":     "list1",
		"data":   map[string]any{"name": "New card", "type": "project"},
	})
	if !res.Success {
		t.Fatalf("expected success, got %q", res.Err)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.bodies[0], &got); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if got["name"] != "New card" || got["type"] != "project" {
		t.Errorf("body = %v, want name/type", got)
	}
}

func TestRetryOnServerError(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		if rec.count() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "projects"), map[string]any{
		"action": "get",
		"id":     "1",
	})
	if !res.Success {
		t.Fatalf("expected success after retry, got %q", res.Err)
	}
	if rec.count() != 2 {
		t.Errorf("expected 2 attempts, got %d", rec.count())
	}
}

func TestBearer401Refresh(t *testing.T) {
	var mu sync.Mutex
	tokenServes := 0
	apiCalls := 0
	var bearers []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/access-tokens" {
			mu.Lock()
			tokenServes++
			tok := "tok" + strconv.Itoa(tokenServes)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]string{"item": tok}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		mu.Lock()
		apiCalls++
		bearers = append(bearers, r.Header.Get("Authorization"))
		first := apiCalls == 1
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := server.New(server.Config{
		BaseURL:        srv.URL,
		Username:       "u",
		Password:       "p",
		MaxRetries:     2,
		RetryBaseDelay: time.Millisecond,
		EnableAll:      true,
	})
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "projects"), map[string]any{
		"action": "get",
		"id":     "1",
	})
	if !res.Success {
		t.Fatalf("expected success after 401 refresh, got %q", res.Err)
	}
	mu.Lock()
	defer mu.Unlock()
	if tokenServes != 2 {
		t.Errorf("expected 2 token fetches, got %d", tokenServes)
	}
	if len(bearers) != 2 || bearers[0] == bearers[1] {
		t.Errorf("expected two distinct bearer tokens, got %v", bearers)
	}
}

func TestCustomFieldDollarFallback(t *testing.T) {
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		if rec.count() == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "customFields"), map[string]any{
		"action": "setValue",
		"id":     "c1",
		"data":   map[string]any{"customFieldGroupId": "g1", "customFieldId": "f1", "content": "x"},
	})
	if !res.Success {
		t.Fatalf("expected success after fallback, got %q", res.Err)
	}
	if rec.count() != 2 {
		t.Fatalf("expected 2 attempts, got %d", rec.count())
	}
	if !strings.Contains(rec.paths[1], "customFieldId:$f1") {
		t.Errorf("fallback path = %q, want to contain customFieldId:$f1", rec.paths[1])
	}
}

func TestErrorTruncation(t *testing.T) {
	big := strings.Repeat("a", 3000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(w, big); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	s := newTestServer(srv.URL)
	res := s.ExecuteGroupedAPICall(t.Context(), findTool(t, "projects"), map[string]any{
		"action": "get",
		"id":     "1",
	})
	if res.Success {
		t.Fatal("expected failure")
	}
	if !strings.HasPrefix(res.Err, "HTTP 400: ") {
		t.Errorf("error prefix wrong: %q", res.Err[:min(20, len(res.Err))])
	}
	if !strings.HasSuffix(res.Err, "…") {
		t.Error("error should end with an ellipsis when truncated")
	}
	if got := strings.Count(res.Err, "a"); got != 2000 {
		t.Errorf("expected 2000 'a' chars after truncation, got %d", got)
	}
}
