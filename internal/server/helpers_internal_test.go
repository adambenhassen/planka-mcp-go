package server

import (
	"context"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range []string{
		"PLANKA_BASE_URL", "MCP_TRANSPORT", "PLANKA_USERNAME", "PLANKA_PASSWORD",
		"PLANKA_API_KEY", "MCP_PORT", "PLANKA_HTTP_MAX_RETRIES",
		"PLANKA_HTTP_RETRY_BASE_DELAY_MS", "ENABLE_ALL_TOOLS", "ENABLE_ADMIN_TOOLS", "ENABLE_OPTIONAL_TOOLS",
	} {
		t.Setenv(k, "")
	}
	cfg := LoadConfig()
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL default = %q", cfg.BaseURL)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport default = %q", cfg.Transport)
	}
	if cfg.Port != 3001 {
		t.Errorf("Port default = %d", cfg.Port)
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries default = %d", cfg.MaxRetries)
	}
	if cfg.RetryBaseDelay != 250*time.Millisecond {
		t.Errorf("RetryBaseDelay default = %v", cfg.RetryBaseDelay)
	}
	if cfg.EnableAll || cfg.EnableAdmin || cfg.EnableOptional {
		t.Error("tool categories should default to disabled")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PLANKA_BASE_URL", "https://planka.example")
	t.Setenv("MCP_TRANSPORT", "sse")
	t.Setenv("PLANKA_API_KEY", "  key123  ") // trimmed
	t.Setenv("MCP_PORT", "9000")
	t.Setenv("PLANKA_HTTP_MAX_RETRIES", "5")
	t.Setenv("PLANKA_HTTP_RETRY_BASE_DELAY_MS", "100")
	t.Setenv("ENABLE_ALL_TOOLS", "true")
	// A non-numeric int env falls back to the default.
	t.Setenv("PLANKA_HTTP_MAX_RETRIES", "notanumber")

	cfg := LoadConfig()
	if cfg.BaseURL != "https://planka.example" || cfg.Transport != "sse" || cfg.Port != 9000 {
		t.Errorf("env not applied: %+v", cfg)
	}
	if cfg.APIKey != "key123" {
		t.Errorf("APIKey = %q, want trimmed key123", cfg.APIKey)
	}
	if cfg.RetryBaseDelay != 100*time.Millisecond {
		t.Errorf("RetryBaseDelay = %v", cfg.RetryBaseDelay)
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("unparseable MaxRetries should fall back to 2, got %d", cfg.MaxRetries)
	}
	if !cfg.EnableAll {
		t.Error("ENABLE_ALL_TOOLS=true not applied")
	}
}

func TestJSStringVariants(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hi", "hi"},
		{true, "true"},
		{false, "false"},
		{float64(3), "3"},
		{float64(1.5), "1.5"},
		{[]any{"a", 1.0}, `["a",1]`},
	}
	for _, c := range cases {
		if got := jsString(c.in); got != c.want {
			t.Errorf("jsString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSTruthyString(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"x", "x"},
		{true, "true"},
		{false, ""},
		{float64(0), ""},
		{float64(7), "7"},
		{[]any{"a"}, `["a"]`},
	}
	for _, c := range cases {
		if got := jsTruthyString(c.in); got != c.want {
			t.Errorf("jsTruthyString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStringifyData(t *testing.T) {
	if got := stringifyData("raw"); got != "raw" {
		t.Errorf("string passthrough = %q", got)
	}
	if got := stringifyData(map[string]any{"a": float64(1)}); got != `{"a":1}` {
		t.Errorf("map stringify = %q", got)
	}
	// Unmarshalable value yields an empty string rather than panicking.
	if got := stringifyData(make(chan int)); got != "" {
		t.Errorf("unmarshalable stringify = %q, want empty", got)
	}
}

func TestEncodeURIComponent(t *testing.T) {
	if got := encodeURIComponent("a b/c?"); got != "a%20b%2Fc%3F" {
		t.Errorf("encodeURIComponent = %q", got)
	}
	// Unreserved characters pass through unescaped.
	if got := encodeURIComponent("A-z0_9.!~*'()"); got != "A-z0_9.!~*'()" {
		t.Errorf("unreserved chars should not be escaped, got %q", got)
	}
}

// TestServeSSELifecycle starts the SSE HTTP server on an ephemeral port and shuts
// it down via context cancellation, exercising the bootstrap and shutdown paths.
func TestServeSSELifecycle(t *testing.T) {
	s := internalTestServer("http://unused.example")
	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() { errCh <- s.ServeSSE(ctx, 0) }() // port 0 → OS picks a free port

	// Give ListenAndServe a moment to bind, then trigger graceful shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ServeSSE returned %v, want nil after shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServeSSE did not return after context cancellation")
	}
}
