package upload_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adambenhassen/planka-mcp-go/internal/tools"
	"github.com/adambenhassen/planka-mcp-go/internal/upload"
)

// findTool returns the named tool from the given group, failing the test if
// absent.
func findTool(t *testing.T, group []tools.GroupedToolDefinition, name string) tools.GroupedToolDefinition {
	t.Helper()
	idx := slices.IndexFunc(group, func(x tools.GroupedToolDefinition) bool { return x.Name == name })
	if idx < 0 {
		t.Fatalf("tool %q not found", name)
	}
	return group[idx]
}

func TestUploadOperationFlags(t *testing.T) {
	attachments := findTool(t, tools.OptionalTools, "attachments")
	if attachments.Operations["create"].Upload != "attachment" {
		t.Errorf("attachments.create.Upload = %q, want attachment", attachments.Operations["create"].Upload)
	}
	backgrounds := findTool(t, tools.OptionalTools, "backgroundImages")
	if backgrounds.Operations["upload"].Upload != "file" {
		t.Errorf("backgroundImages.upload.Upload = %q, want file", backgrounds.Operations["upload"].Upload)
	}
	users := findTool(t, tools.AdminTools, "users")
	if users.Operations["updateAvatar"].Upload != "file" {
		t.Errorf("users.updateAvatar.Upload = %q, want file", users.Operations["updateAvatar"].Upload)
	}
}

// newServer starts a test HTTP server for the given handler and registers its
// cleanup.
func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	// Silence expected "wrote fewer bytes than declared Content-Length" noise.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveBytesRejectsMissingURL(t *testing.T) {
	_, err := upload.ResolveBytes(t.Context(), map[string]any{})
	if err == nil || !regexp.MustCompile(`(?i)url`).MatchString(err.Error()) {
		t.Errorf("expected url error, got %v", err)
	}
}

func TestResolveBytesFetchesAndDerives(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/img/cat.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte("PNGDATA")); err != nil {
			t.Errorf("write: %v", err)
		}
	})

	out, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/img/cat.png"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Bytes) != "PNGDATA" {
		t.Errorf("bytes = %q, want PNGDATA", out.Bytes)
	}
	if out.Filename != "cat.png" {
		t.Errorf("filename = %q, want cat.png", out.Filename)
	}
	if out.ContentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", out.ContentType)
	}
}

func TestResolveBytesExplicitNameOverride(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte("X")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	out, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/img/cat.png", "name": "renamed.png"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Filename != "renamed.png" {
		t.Errorf("filename = %q, want renamed.png", out.Filename)
	}
}

func TestResolveBytesFallbackFilename(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte("X")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	out, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Filename != "upload" {
		t.Errorf("filename = %q, want upload", out.Filename)
	}
}

func TestResolveBytesOctetStreamFallback(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		// No Content-Type set and none sniffed.
		w.Header()["Content-Type"] = nil
		if _, err := w.Write([]byte("X")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	out, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/raw"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "application/octet-stream" {
		t.Errorf("contentType = %q, want application/octet-stream", out.ContentType)
	}
}

func TestResolveBytesRejectsNon2xx(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	_, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/missing"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestResolveBytesRejectsDeclaredOversize(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(upload.MaxURLDownloadBytes+1))
		if _, err := w.Write([]byte("x")); err != nil {
			t.Logf("server write ended early (expected): %v", err)
		}
	})
	_, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/big"})
	if err == nil || !regexp.MustCompile(`(?i)too large`).MatchString(err.Error()) {
		t.Errorf("expected too-large error, got %v", err)
	}
}

func TestResolveBytesRejectsOversizeBody(t *testing.T) {
	big := bytes.Repeat([]byte{0x61}, upload.MaxURLDownloadBytes+16)
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(big); err != nil {
			t.Logf("server write ended early (expected): %v", err)
		}
	})
	_, err := upload.ResolveBytes(t.Context(), map[string]any{"url": srv.URL + "/stream"})
	if err == nil || !regexp.MustCompile(`(?i)too large`).MatchString(err.Error()) {
		t.Errorf("expected too-large error, got %v", err)
	}
}

func TestResolveBytesRejectsNonHTTPScheme(t *testing.T) {
	_, err := upload.ResolveBytes(t.Context(), map[string]any{"url": "file:///etc/passwd"})
	if err == nil || !regexp.MustCompile(`(?i)protocol`).MatchString(err.Error()) {
		t.Errorf("expected protocol error, got %v", err)
	}
}

// filePart is a parsed multipart file part.
type filePart struct {
	Filename string
	Content  string
}

// parseForm parses a multipart body into its scalar fields and file parts.
func parseForm(t *testing.T, body []byte, contentType string) (map[string]string, map[string]filePart) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := map[string]string{}
	files := map[string]filePart{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() != "" {
			files[part.FormName()] = filePart{Filename: part.FileName(), Content: string(content)}
		} else {
			fields[part.FormName()] = string(content)
		}
	}
	return fields, files
}

func TestBuildUploadFormFile(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte("BG")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	body, ct, err := upload.BuildUploadForm(t.Context(), "file", map[string]any{"url": srv.URL + "/bg.png", "name": "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	fields, files := parseForm(t, body, ct)
	if files["file"].Content != "BG" {
		t.Errorf("file content = %q, want BG", files["file"].Content)
	}
	if _, ok := fields["name"]; ok {
		t.Error("backgrounds should have no name field")
	}
}

func TestBuildUploadFormFileAttachment(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte("IMG")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	body, ct, err := upload.BuildUploadForm(t.Context(), "attachment", map[string]any{"type": "file", "url": srv.URL + "/img.png", "name": "pic.png"})
	if err != nil {
		t.Fatal(err)
	}
	fields, files := parseForm(t, body, ct)
	if fields["type"] != "file" {
		t.Errorf("type = %q, want file", fields["type"])
	}
	if fields["name"] != "pic.png" {
		t.Errorf("name = %q, want pic.png", fields["name"])
	}
	if files["file"].Content != "IMG" {
		t.Errorf("file content = %q, want IMG", files["file"].Content)
	}
	if files["file"].Filename != "pic.png" {
		t.Errorf("file filename = %q, want pic.png", files["file"].Filename)
	}
}

func TestBuildUploadFormLinkAttachment(t *testing.T) {
	body, ct, err := upload.BuildUploadForm(t.Context(), "attachment", map[string]any{
		"type": "link",
		"url":  "https://example.com/page",
		"name": "A link",
	})
	if err != nil {
		t.Fatal(err)
	}
	fields, files := parseForm(t, body, ct)
	if fields["type"] != "link" {
		t.Errorf("type = %q, want link", fields["type"])
	}
	if fields["url"] != "https://example.com/page" {
		t.Errorf("url = %q, want https://example.com/page", fields["url"])
	}
	if fields["name"] != "A link" {
		t.Errorf("name = %q, want A link", fields["name"])
	}
	if _, ok := files["file"]; ok {
		t.Error("link attachments should have no file part")
	}
}

func TestBuildUploadFormLinkWithoutURL(t *testing.T) {
	_, _, err := upload.BuildUploadForm(t.Context(), "attachment", map[string]any{"type": "link", "name": "x"})
	if err == nil || !regexp.MustCompile(`(?i)link attachment requires`).MatchString(err.Error()) {
		t.Errorf("expected link-attachment error, got %v", err)
	}
}

func dataDescription(t *testing.T, tool tools.GroupedToolDefinition) string {
	t.Helper()
	data, ok := tool.InputSchema.Properties["data"].(map[string]any)
	if !ok {
		t.Fatalf("tool %s has no data property", tool.Name)
	}
	desc, ok := data["description"].(string)
	if !ok {
		t.Fatalf("tool %s data has no description", tool.Name)
	}
	return desc
}

func TestSchemaDocumentsAttachmentsURL(t *testing.T) {
	desc := dataDescription(t, findTool(t, tools.OptionalTools, "attachments"))
	if !strings.Contains(desc, "url") {
		t.Error("attachments data should mention url")
	}
	if strings.Contains(desc, "base64") {
		t.Error("attachments data should not mention base64")
	}
	if !strings.Contains(desc, "file") || !strings.Contains(desc, "link") {
		t.Error("attachments data should mention file and link")
	}
}

func TestSchemaDocumentsBackgroundImagesURL(t *testing.T) {
	desc := dataDescription(t, findTool(t, tools.OptionalTools, "backgroundImages"))
	if !strings.Contains(desc, "url") {
		t.Error("backgroundImages data should mention url")
	}
	if strings.Contains(desc, "base64") {
		t.Error("backgroundImages data should not mention base64")
	}
}

func TestSchemaDocumentsCoverAttachmentID(t *testing.T) {
	desc := dataDescription(t, findTool(t, tools.CoreTools, "cards"))
	if !strings.Contains(desc, "coverAttachmentId") {
		t.Error("cards data should mention coverAttachmentId")
	}
}

func TestSchemaDocumentsBackgroundImageID(t *testing.T) {
	desc := dataDescription(t, findTool(t, tools.CoreTools, "projects"))
	if !strings.Contains(desc, "backgroundImageId") {
		t.Error("projects data should mention backgroundImageId")
	}
}
