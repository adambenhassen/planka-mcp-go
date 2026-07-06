package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// heartbeatInterval is how often an SSE comment is sent to keep proxies from
// timing out an idle connection.
const heartbeatInterval = 30 * time.Second

// sseSession is a single connected SSE client. Its ResponseWriter is written to
// both by its own heartbeat loop and by /messages POST handlers, so all writes
// are serialized through mu.
type sseSession struct {
	id      string
	writer  http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	closed  bool
}

// send writes an SSE event with the given name and data to the client,
// reporting whether the write succeeded.
func (sess *sseSession) send(event, data string) bool {
	return sess.write("event: " + event + "\ndata: " + data + "\n\n")
}

// comment writes an SSE comment (used as a heartbeat) to the client, reporting
// whether the write succeeded.
func (sess *sseSession) comment(text string) bool {
	return sess.write(": " + text + "\n\n")
}

// write emits a raw SSE frame under the session lock, reporting success.
func (sess *sseSession) write(frame string) bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.closed {
		return false
	}
	if _, err := io.WriteString(sess.writer, frame); err != nil {
		return false
	}
	sess.flusher.Flush()
	return true
}

// sseSessions is the concurrency-safe registry of active SSE sessions.
type sseSessions struct {
	mu sync.Mutex
	m  map[string]*sseSession
}

func (s *sseSessions) add(sess *sseSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[sess.id] = sess
}

func (s *sseSessions) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

func (s *sseSessions) get(id string) (*sseSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[id]
	return sess, ok
}

func (s *sseSessions) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// newSessionID returns a random hex session identifier.
func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read never fails on supported platforms; fall back to a
		// timestamp so the server keeps running rather than crashing.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// ServeSSE runs the MCP server over SSE on the given port, exposing /sse,
// /messages, and /health with permissive CORS.
func (s *Server) ServeSSE(ctx context.Context, port int) error {
	sessions := &sseSessions{m: make(map[string]*sseSession)}

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE(ctx, sessions))
	mux.HandleFunc("/messages", s.handleMessages(ctx, sessions))
	mux.HandleFunc("/health", s.handleHealth(sessions))

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 125 * time.Second,
	}

	//nolint:gosec // G118: the shutdown goroutine intentionally uses a fresh detached context because the server context is already cancelled when it runs.
	go func() {
		<-ctx.Done()
		// Detached from the (already-cancelled) server context on purpose: a
		// graceful shutdown needs its own live deadline.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("SSE server shutdown", "error", err)
		}
	}()

	s.logger.Info("Planka MCP server (SSE) listening",
		"port", port, "tools", len(s.tools), "heartbeatSeconds", int(heartbeatInterval/time.Second))
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// withCORS wraps a handler with permissive CORS headers and OPTIONS handling.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleSSE returns the handler for GET /sse, which opens an event stream and
// registers a session.
func (s *Server) handleSSE(serverCtx context.Context, sessions *sseSessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		sess := &sseSession{id: newSessionID(), writer: w, flusher: flusher}
		sessions.add(sess)
		s.logger.Info("client connected", "sessionId", sess.id, "activeClients", sessions.count())

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		cleanup := func() {
			sess.mu.Lock()
			sess.closed = true
			sess.mu.Unlock()
			sessions.remove(sess.id)
			s.logger.Info("client disconnected", "sessionId", sess.id, "activeClients", sessions.count())
		}
		defer cleanup()

		// Tell the client where to POST messages for this session; bail (cleaning
		// up) if the connection is already gone.
		if !sess.send("endpoint", "/messages?sessionId="+sess.id) {
			return
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case <-serverCtx.Done():
				return
			case <-ticker.C:
				if !sess.comment("heartbeat") {
					return
				}
			}
		}
	}
}

// handleMessages returns the handler for POST /messages, which routes a JSON-RPC
// message to its session's event stream.
func (s *Server) handleMessages(ctx context.Context, sessions *sseSessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID := r.URL.Query().Get("sessionId")
		if sessionID == "" {
			writeJSONError(w, http.StatusBadRequest, "Missing sessionId parameter")
			return
		}
		sess, ok := sessions.get(sessionID)
		if !ok {
			writeJSONError(w, http.StatusNotFound, "Session not found")
			return
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		resp, isNotification := s.handleMessage(ctx, payload)

		w.WriteHeader(http.StatusAccepted)
		if _, err := io.WriteString(w, "Accepted"); err != nil {
			return
		}

		if !isNotification && resp != nil {
			sess.send("message", string(resp))
		}
	}
}

// handleHealth returns the handler for GET /health.
func (s *Server) handleHealth(sessions *sseSessions) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		admin := map[string]int{"tools": 0, "operations": 0}
		if s.cfg.EnableAdmin || s.cfg.EnableAll {
			admin = map[string]int{"tools": s.counts.Admin, "operations": s.counts.AdminOperations}
		}
		optional := map[string]int{"tools": 0, "operations": 0}
		if s.cfg.EnableOptional || s.cfg.EnableAll {
			optional = map[string]int{"tools": s.counts.Optional, "operations": s.counts.OptionalOperations}
		}
		body := map[string]any{
			"status":         "ok",
			"activeClients":  sessions.count(),
			"toolsAvailable": len(s.tools),
			"toolCounts": map[string]any{
				"core":     map[string]int{"tools": s.counts.Core, "operations": s.counts.CoreOperations},
				"admin":    admin,
				"optional": optional,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			s.logger.Error("health encode failed", "error", err)
		}
	}
}

// writeJSONError writes a JSON {"error": msg} body with the given status.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		return
	}
}
