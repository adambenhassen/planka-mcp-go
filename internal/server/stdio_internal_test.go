package server

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWriteLine(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := writeLine(w, []byte(`{"ok":1}`)); err != nil {
		t.Fatalf("writeLine: %v", err)
	}
	if buf.String() != `{"ok":1}`+"\n" {
		t.Errorf("writeLine wrote %q, want trailing newline", buf.String())
	}
}

// TestServeStdioRequestAndNotification feeds one request and one notification
// over a fake stdin, and confirms exactly the request gets a response line and
// that EOF ends the loop cleanly.
func TestServeStdioRequestAndNotification(t *testing.T) {
	origIn, origOut := os.Stdin, os.Stdout
	t.Cleanup(func() { os.Stdin, os.Stdout = origIn, origOut })

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin, os.Stdout = inR, outW

	s := internalTestServer("http://unused.example")
	done := make(chan error, 1)
	go func() { done <- s.ServeStdio(t.Context()) }()

	if _, err := io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	// A notification (no id) must produce no output line.
	if _, err := io.WriteString(inW, `{"jsonrpc":"2.0","method":"ping"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	if err := inW.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatalf("ServeStdio returned error on EOF: %v", err)
	}
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one response line, got %d: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], `"id":1`) {
		t.Errorf("response line = %q, want the ping request's id:1", lines[0])
	}
}

func TestServeStdioStopsOnCancelledContext(t *testing.T) {
	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })
	inR, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = inR

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // already cancelled: the loop must return before blocking on Read.

	s := internalTestServer("http://unused.example")
	if err := s.ServeStdio(ctx); err != nil {
		t.Errorf("ServeStdio on cancelled context = %v, want nil", err)
	}
}
