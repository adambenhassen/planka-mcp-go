// Package server forwards MCP tool calls to a Planka REST API and speaks the
// MCP protocol over stdio or SSE. It is a 1:1 port of the TypeScript
// @adambenhassen/planka-mcp server.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/adambenhassen/planka-mcp-go/internal/tools"
	"github.com/adambenhassen/planka-mcp-go/internal/upload"
)

// serverVersion is the MCP server version reported to clients.
const serverVersion = "2.2.1"

// tokenExpiryBuffer is how long before the cached token's expiry it is
// considered stale (matching the TS 5-minute buffer).
const tokenExpiryBuffer = 5 * time.Minute

// tokenLifetime is how long a freshly fetched token is treated as valid.
const tokenLifetime = 24 * time.Hour

// knownIDParams are the multi-segment path parameters that fall back to the
// top-level id field when not supplied in data.
var knownIDParams = []string{
	"projectId", "boardId", "listId", "cardId", "userId",
	"taskListId", "baseCustomFieldGroupId", "customFieldGroupId", "customFieldId",
}

// Config holds the runtime configuration read from the environment.
type Config struct {
	// BaseURL is the Planka base URL (without the /api suffix).
	BaseURL string
	// Username is the Planka login username/email (password auth).
	Username string
	// Password is the Planka login password (password auth).
	Password string
	// APIKey, when set, is sent as X-Api-Key and takes precedence over password auth.
	APIKey string
	// Port is the SSE HTTP port.
	Port int
	// Transport selects "stdio" or "sse".
	Transport string
	// MaxRetries is the maximum number of HTTP retries for transient failures.
	MaxRetries int
	// RetryBaseDelay is the base delay for exponential backoff between retries.
	RetryBaseDelay time.Duration
	// EnableAll enables every tool category.
	EnableAll bool
	// EnableAdmin enables the admin tool category.
	EnableAdmin bool
	// EnableOptional enables the optional tool category.
	EnableOptional bool
}

// envInt reads an integer environment variable, returning def when unset or
// unparseable, mirroring the TS parseInt(..., 10) with a numeric default.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// LoadConfig reads the server configuration from environment variables, applying
// the same defaults as the TypeScript server.
func LoadConfig() Config {
	baseURL := os.Getenv("PLANKA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}
	return Config{
		BaseURL:        baseURL,
		Username:       os.Getenv("PLANKA_USERNAME"),
		Password:       os.Getenv("PLANKA_PASSWORD"),
		APIKey:         strings.TrimSpace(os.Getenv("PLANKA_API_KEY")),
		Port:           envInt("MCP_PORT", 3001),
		Transport:      transport,
		MaxRetries:     envInt("PLANKA_HTTP_MAX_RETRIES", 2),
		RetryBaseDelay: time.Duration(envInt("PLANKA_HTTP_RETRY_BASE_DELAY_MS", 250)) * time.Millisecond,
		EnableAll:      os.Getenv("ENABLE_ALL_TOOLS") == "true",
		EnableAdmin:    os.Getenv("ENABLE_ADMIN_TOOLS") == "true",
		EnableOptional: os.Getenv("ENABLE_OPTIONAL_TOOLS") == "true",
	}
}

// Server forwards MCP tool calls to Planka and manages the auth token cache.
type Server struct {
	cfg        Config
	httpClient *http.Client
	logger     *log.Logger
	tools      []tools.GroupedToolDefinition
	toolMap    map[string]tools.GroupedToolDefinition
	counts     tools.Counts

	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New builds a Server for cfg, selecting the enabled tools.
func New(cfg Config) *Server {
	enabled := tools.GetEnabledTools(tools.ToolConfig{
		EnableAllTools:      cfg.EnableAll,
		EnableAdminTools:    cfg.EnableAdmin,
		EnableOptionalTools: cfg.EnableOptional,
	})
	toolMap := make(map[string]tools.GroupedToolDefinition, len(enabled))
	for _, t := range enabled {
		toolMap[t.Name] = t
	}
	return &Server{
		cfg:        cfg,
		httpClient: &http.Client{},
		logger:     log.New(os.Stderr, "", 0),
		tools:      enabled,
		toolMap:    toolMap,
		counts:     tools.ToolCounts(),
	}
}

// EnabledTools returns the tools currently exposed by the server.
func (s *Server) EnabledTools() []tools.GroupedToolDefinition { return s.tools }

// Counts returns the tool and operation counts.
func (s *Server) Counts() tools.Counts { return s.counts }

// hasCachedToken reports whether a bearer token is currently cached.
func (s *Server) hasCachedToken() bool {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	return s.cachedToken != ""
}

// clearToken drops the cached bearer token so the next call re-authenticates.
func (s *Server) clearToken() {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.cachedToken = ""
	s.tokenExpiry = time.Time{}
}

// getAccessToken returns a valid bearer token, using the cache when fresh and
// authenticating against Planka otherwise.
func (s *Server) getAccessToken(ctx context.Context) (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry.Add(-tokenExpiryBuffer)) {
		return s.cachedToken, nil
	}

	if s.cfg.Username == "" || s.cfg.Password == "" {
		return "", errors.New("Authentication is not configured. Set PLANKA_API_KEY or PLANKA_USERNAME and PLANKA_PASSWORD.")
	}

	payload, err := json.Marshal(map[string]string{
		"emailOrUsername": s.cfg.Username,
		"password":        s.cfg.Password,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/api/access-tokens", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	text, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("Authentication failed: " + strconv.Itoa(res.StatusCode) + " - " + string(text))
	}

	var parsed struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(text, &parsed); err != nil {
		return "", err
	}
	s.cachedToken = parsed.Item
	s.tokenExpiry = time.Now().Add(tokenLifetime)
	return s.cachedToken, nil
}

// APIResult is the outcome of a forwarded Planka API call.
type APIResult struct {
	// Success reports whether the call returned a 2xx status.
	Success bool
	// Data is the parsed response body (a decoded value or a raw string).
	Data any
	// Err is a human-readable error message when Success is false.
	Err string
}

// callState carries the recursion parameters for retries and fallbacks.
type callState struct {
	overridePath                   string
	hasOverride                    bool
	allowCustomFieldDollarFallback bool
	retryAttempt                   int
	unauthorizedRetried            bool
	prebuiltUpload                 []byte
	prebuiltUploadCT               string
	hasPrebuilt                    bool
}

// ExecuteGroupedAPICall forwards a grouped tool action to the Planka API.
func (s *Server) ExecuteGroupedAPICall(ctx context.Context, def tools.GroupedToolDefinition, input map[string]any) APIResult {
	return s.execute(ctx, def, input, callState{allowCustomFieldDollarFallback: true})
}

// asString returns v as a string, or "" when v is not a string.
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// asMap returns v as a map[string]any, or nil when v is not one.
func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

//nolint:gocyclo // Faithful 1:1 port of the TS executeGroupedApiCall control flow (path resolution, auth, retries, and compat fallbacks) kept in one function for parity.
func (s *Server) execute(ctx context.Context, def tools.GroupedToolDefinition, input map[string]any, st callState) APIResult {
	action := asString(input["action"])
	if action == "" {
		return APIResult{Err: "Missing required 'action' parameter"}
	}
	op, ok := def.Operations[action]
	if !ok {
		valid := make([]string, 0, len(def.Operations))
		for name := range def.Operations {
			valid = append(valid, name)
		}
		slices.Sort(valid)
		return APIResult{Err: "Invalid action '" + action + "'. Valid actions: " + strings.Join(valid, ", ")}
	}

	actualPath := op.Path
	if st.hasOverride {
		actualPath = st.overridePath
	}

	id := asString(input["id"])
	data := asMap(input["data"])

	for _, param := range pathParamPattern.FindAllString(actualPath, -1) {
		name := param[1 : len(param)-1]
		value, has := "", false
		switch {
		case name == "id" && id != "":
			value, has = id, true
		case dataString(data, name) != "":
			value, has = dataString(data, name), true
		case id != "" && slices.Contains(knownIDParams, name):
			value, has = id, true
		}
		if has {
			actualPath = strings.Replace(actualPath, param, encodeURIComponent(value), 1)
		}
	}

	if unresolved := pathParamPattern.FindAllString(actualPath, -1); len(unresolved) > 0 {
		return APIResult{Err: "Missing required path parameters: " + strings.Join(unresolved, ", ") + ". Provide them via 'id' or 'data'."}
	}

	reqURL, err := url.Parse(s.cfg.BaseURL + "/api" + actualPath)
	if err != nil {
		return APIResult{Err: err.Error()}
	}
	if query, ok := input["query"].(map[string]any); ok {
		values := reqURL.Query()
		for k, v := range query {
			switch vv := v.(type) {
			case []any:
				for _, item := range vv {
					values.Add(k, jsString(item))
				}
			case map[string]any:
				for sub, val := range vv {
					if val != nil {
						values.Set(k+"["+sub+"]", jsString(val))
					}
				}
			default:
				if v != nil {
					values.Set(k, jsString(v))
				}
			}
		}
		reqURL.RawQuery = values.Encode()
	}

	header := http.Header{}
	header.Set("Accept", "application/json")

	requiresAuth := !op.NoAuth
	if requiresAuth {
		if s.cfg.APIKey != "" {
			header.Set("X-Api-Key", s.cfg.APIKey)
		} else {
			token, err := s.getAccessToken(ctx)
			if err != nil {
				return APIResult{Err: err.Error()}
			}
			header.Set("Authorization", "Bearer "+token)
		}
	}

	var body []byte
	if slices.Contains([]string{"POST", "PUT", "PATCH"}, op.Method) && data != nil {
		if op.Upload != "" {
			if st.hasPrebuilt {
				body = st.prebuiltUpload
				header.Set("Content-Type", st.prebuiltUploadCT)
			} else {
				built, ct, err := upload.BuildUploadForm(ctx, upload.UploadKind(op.Upload), data)
				if err != nil {
					return APIResult{Err: err.Error()}
				}
				body = built
				header.Set("Content-Type", ct)
				st.prebuiltUpload, st.prebuiltUploadCT, st.hasPrebuilt = built, ct, true
			}
		} else {
			header.Set("Content-Type", "application/json")
			marshaled, err := json.Marshal(data)
			if err != nil {
				return APIResult{Err: err.Error()}
			}
			body = marshaled
		}
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, op.Method, reqURL.String(), bodyReader)
	if err != nil {
		return APIResult{Err: err.Error()}
	}
	req.Header = header

	res, err := s.httpClient.Do(req)
	if err != nil {
		if st.retryAttempt < s.cfg.MaxRetries {
			delay := s.cfg.RetryBaseDelay << st.retryAttempt
			s.logger.Printf("[retry] network error for %s.%s (%s %s), attempt %s failed, retrying in %dms: %v",
				def.Name, action, op.Method, actualPath, s.formatAttempt(st.retryAttempt), delay.Milliseconds(), err)
			sleepCtx(ctx, delay)
			st.retryAttempt++
			return s.execute(ctx, def, input, st)
		}
		return APIResult{Err: err.Error()}
	}

	contentType := res.Header.Get("Content-Type")
	text, readErr := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil && readErr == nil {
		readErr = cerr
	}
	if readErr != nil {
		return APIResult{Err: readErr.Error()}
	}

	var respData any = string(text)
	if strings.Contains(contentType, "application/json") && len(text) > 0 {
		var parsed any
		if json.Unmarshal(text, &parsed) == nil {
			respData = parsed
		}
	}

	status := res.StatusCode
	if status >= 200 && status < 300 {
		return APIResult{Success: true, Data: respData}
	}

	if status == http.StatusUnauthorized && requiresAuth && s.cfg.APIKey == "" && s.hasCachedToken() && !st.unauthorizedRetried {
		s.clearToken()
		s.logger.Printf("[auth] received 401 for %s.%s (%s %s); clearing cached token and retrying once",
			def.Name, action, op.Method, actualPath)
		st.unauthorizedRetried = true
		return s.execute(ctx, def, input, st)
	}

	if status == http.StatusNotFound && st.allowCustomFieldDollarFallback && def.Name == "customFields" &&
		(action == "setValue" || action == "clearValue") {
		templatePath := op.Path
		if st.hasOverride {
			templatePath = st.overridePath
		}
		fallbackPath := strings.Replace(templatePath, "customFieldId:{customFieldId}", "customFieldId:${customFieldId}", 1)
		if fallbackPath != templatePath {
			s.logger.Printf("[compat] 404 for %s.%s using %s; retrying with legacy custom field path variant",
				def.Name, action, actualPath)
			st.overridePath, st.hasOverride = fallbackPath, true
			st.allowCustomFieldDollarFallback = false
			return s.execute(ctx, def, input, st)
		}
	}

	if shouldRetryStatus(status) && st.retryAttempt < s.cfg.MaxRetries {
		delay := s.cfg.RetryBaseDelay << st.retryAttempt
		s.logger.Printf("[retry] transient HTTP %d for %s.%s (%s %s), attempt %s failed, retrying in %dms",
			status, def.Name, action, op.Method, actualPath, s.formatAttempt(st.retryAttempt), delay.Milliseconds())
		sleepCtx(ctx, delay)
		st.retryAttempt++
		return s.execute(ctx, def, input, st)
	}

	s.logger.Printf("[error] API call failed for %s.%s (%s %s) with HTTP %d",
		def.Name, action, op.Method, actualPath, status)
	return APIResult{Err: "HTTP " + strconv.Itoa(status) + ": " + truncate(stringifyData(respData), 2000)}
}

// formatAttempt renders the "n/total" attempt label used in retry logs.
func (s *Server) formatAttempt(attempt int) string {
	return strconv.Itoa(attempt+1) + "/" + strconv.Itoa(s.cfg.MaxRetries+1)
}

// dataString returns data[key] as a non-empty string, or "" when absent, empty,
// or not a string.
func dataString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	return jsTruthyString(data[key])
}

// jsTruthyString stringifies v for path substitution, returning "" for
// JavaScript-falsy values (nil, "", false, 0).
func jsTruthyString(v any) string {
	switch vv := v.(type) {
	case nil:
		return ""
	case string:
		return vv
	case bool:
		if vv {
			return "true"
		}
		return ""
	case float64:
		if vv == 0 {
			return ""
		}
		return jsNumber(vv)
	default:
		return jsString(v)
	}
}

// jsString stringifies a JSON-decoded value the way JavaScript's String() would
// for the scalars we encounter in query parameters.
func jsString(v any) string {
	switch vv := v.(type) {
	case nil:
		return ""
	case string:
		return vv
	case bool:
		if vv {
			return "true"
		}
		return "false"
	case float64:
		return jsNumber(vv)
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(marshaled)
	}
}

// jsNumber formats a float64 like JavaScript: integral values without a decimal
// point.
func jsNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// stringifyData renders a response body for an error message: raw strings pass
// through, everything else is compact JSON.
func stringifyData(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	marshaled, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(marshaled)
}

// truncate shortens s to at most n bytes, appending an ellipsis when cut, and
// trims any partial trailing UTF-8 rune.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}

// shouldRetryStatus reports whether an HTTP status is transient and retryable.
func shouldRetryStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
