// Package upload builds the multipart/form-data bodies Planka expects for image
// and file uploads, fetching file bytes from an operator-provided URL when
// needed. It mirrors the TypeScript upload.ts module 1:1.
package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strconv"
	"time"
)

// MaxURLDownloadBytes caps how many bytes we will download from a URL so a bad
// link cannot pull an unbounded blob into memory.
const MaxURLDownloadBytes = 10 * 1024 * 1024

// allowedProtocols is the scheme allowlist: only http(s). This does NOT block
// internal/private hosts — it is not full SSRF protection; the trusted source
// is our own object store.
var allowedProtocols = map[string]bool{"http": true, "https": true}

// UploadKind selects the multipart shape: "attachment" (type/name/url fields +
// optional file part) or "file" (bare file part).
type UploadKind string

// ResolvedBytes holds the fetched file content plus its derived filename and
// content type.
type ResolvedBytes struct {
	// Bytes is the downloaded file content.
	Bytes []byte
	// Filename is the name to use for the multipart file part.
	Filename string
	// ContentType is the MIME type reported by the source (or a fallback).
	ContentType string
}

// stringField reads a string value from a loose data map, reporting whether it
// was present and non-empty.
func stringField(data map[string]any, key string) (string, bool) {
	v, ok := data[key].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// ResolveBytes fetches the bytes for an upload from data["url"], enforcing the
// scheme allowlist and size cap, and deriving a filename and content type.
func ResolveBytes(ctx context.Context, data map[string]any) (ResolvedBytes, error) {
	rawURL, ok := stringField(data, "url")
	if !ok {
		return ResolvedBytes{}, errors.New("Image upload requires a 'url' in data.")
	}
	name, _ := stringField(data, "name")
	return fetchURLBytes(ctx, rawURL, name)
}

// fetchURLBytes downloads rawURL over http(s), enforcing the size cap, and
// derives a filename (explicit name, else URL basename, else "upload") and
// content type (response header, else application/octet-stream).
func fetchURLBytes(ctx context.Context, rawURL, name string) (out ResolvedBytes, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ResolvedBytes{}, fmt.Errorf("Invalid image url: %s", rawURL)
	}
	if !allowedProtocols[parsed.Scheme] {
		return ResolvedBytes{}, fmt.Errorf("Unsupported url protocol '%s:' — only http and https are allowed.", parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ResolvedBytes{}, fmt.Errorf("Failed to fetch image from url %s: %w", rawURL, err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return ResolvedBytes{}, fmt.Errorf("Failed to fetch image from url %s: %w", rawURL, err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ResolvedBytes{}, fmt.Errorf("Failed to fetch image from url %s: HTTP %d", rawURL, res.StatusCode)
	}

	// Cheap early-out for honest servers; the streaming guard below is the real cap.
	if declared, err := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64); err == nil && declared > MaxURLDownloadBytes {
		return ResolvedBytes{}, fmt.Errorf("Image at url too large (%d bytes > %d).", declared, MaxURLDownloadBytes)
	}

	// Read one byte past the cap so a missing/lying Content-Length can't buffer
	// an unbounded body; anything longer than the cap is rejected.
	body, err := io.ReadAll(io.LimitReader(res.Body, MaxURLDownloadBytes+1))
	if err != nil {
		return ResolvedBytes{}, fmt.Errorf("Failed to fetch image from url %s: %w", rawURL, err)
	}
	if len(body) > MaxURLDownloadBytes {
		return ResolvedBytes{}, fmt.Errorf("Image at url too large (exceeds %d bytes).", MaxURLDownloadBytes)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	filename := name
	if filename == "" {
		// EscapedPath keeps percent-encoding, matching Node's URL.pathname so the
		// derived multipart filename is identical to the TS server's.
		filename = urlBasename(parsed.EscapedPath())
	}
	if filename == "" {
		filename = "upload"
	}

	return ResolvedBytes{Bytes: body, Filename: filename, ContentType: contentType}, nil
}

// urlBasename returns the final path segment, mirroring Node's basename: a
// trailing-slash or empty path yields "" (so the caller falls back to "upload").
func urlBasename(p string) string {
	base := path.Base(p)
	if base == "/" || base == "." || base == "" {
		return ""
	}
	return base
}

// BuildUploadForm builds the multipart/form-data body Planka expects for an
// upload and returns the body bytes with the matching Content-Type header.
//
//   - kind "attachment" + type "link": type + url + name fields, no file part
//     (the url is stored, not fetched).
//   - kind "attachment" + type "file", or kind "file" (backgrounds, avatars):
//     fetch bytes from the url and send a file part.
func BuildUploadForm(ctx context.Context, kind UploadKind, data map[string]any) (body []byte, contentType string, err error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)

	if kind == "attachment" {
		attType, ok := stringField(data, "type")
		if !ok {
			attType = "file"
		}
		if err := form.WriteField("type", attType); err != nil {
			return nil, "", err
		}
		if name, ok := stringField(data, "name"); ok {
			if err := form.WriteField("name", name); err != nil {
				return nil, "", err
			}
		}
		if attType == "link" {
			linkURL, ok := stringField(data, "url")
			if !ok {
				return nil, "", errors.New("link attachment requires a 'url' in data.")
			}
			if err := form.WriteField("url", linkURL); err != nil {
				return nil, "", err
			}
			if err := form.Close(); err != nil {
				return nil, "", err
			}
			return buf.Bytes(), form.FormDataContentType(), nil
		}
	}

	resolved, err := ResolveBytes(ctx, data)
	if err != nil {
		return nil, "", err
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, resolved.Filename))
	header.Set("Content-Type", resolved.ContentType)
	part, err := form.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(resolved.Bytes); err != nil {
		return nil, "", err
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), form.FormDataContentType(), nil
}
