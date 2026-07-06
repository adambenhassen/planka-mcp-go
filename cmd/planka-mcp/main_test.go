package main

import (
	"log/slog"
	"os"
	"testing"
)

// TestRunStdioReturnsOnEOF exercises the stdio wiring in run(): it loads config,
// builds the server, logs the banner, and serves stdio until stdin EOFs.
func TestRunStdioReturnsOnEOF(t *testing.T) {
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("PLANKA_API_KEY", "test-key")
	t.Setenv("ENABLE_ALL_TOOLS", "true")

	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil { // immediate EOF on stdin
		t.Fatal(err)
	}
	os.Stdin = r

	logger := slog.New(slog.DiscardHandler)
	if err := run(logger); err != nil {
		t.Fatalf("run returned %v, want nil on stdin EOF", err)
	}
}
