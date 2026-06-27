// Package server implements the raw net/http Bot API server: URL routing,
// multipart/query parsing, and JSON response envelope construction. It
// delegates to a Handler interface (implemented by manager.Manager) so the
// server package never imports the client or manager packages.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
)

// Handler routes a parsed Query to the per-bot client layer and returns the HTTP
// status plus the JSON envelope body to write. Implemented by manager.Manager;
// kept as an interface here so the server package does not import manager (no cycle).
type Handler interface {
	Handle(ctx context.Context, q *Query) (status int, body []byte)
}

// Config configures the HTTP server.
type Config struct {
	Addr            string        // listen "host:port"
	TempDir         string        // temp dir for multipart uploads
	MaxFileCount    int           // default 50 (ref HttpServer.h MAX_FILE_COUNT)
	MaxInMemorySize int64         // reserved: file parts are always streamed to disk (parseMultipart)
	ReadTimeout     time.Duration // max duration reading entire request (0 = no timeout)
	WriteTimeout    time.Duration // max duration writing response (0 = no timeout)
	IdleTimeout     time.Duration // max idle keep-alive period (0 = no timeout)
	MaxBodyBytes    int64         // max request body size, 0 = use default (100 MiB)
}

// Server is the raw net/http Bot API server.
type Server struct {
	cfg     Config
	handler Handler
	mu      sync.Mutex
	srv     *http.Server
	stats   http.Handler // optional /stats handler
}

// SetStatsHandler sets the /stats endpoint handler.
func (s *Server) SetStatsHandler(h http.Handler) {
	s.stats = h
}

// New creates a Server bound to the given Handler.
func New(cfg Config, h Handler) *Server {
	if cfg.MaxFileCount == 0 {
		cfg.MaxFileCount = 50
	}
	if cfg.TempDir == "" {
		cfg.TempDir = os.TempDir()
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 60 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 * time.Second
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 100 << 20 // 100 MiB
	}
	return &Server{cfg: cfg, handler: h}
}

// Start serves HTTP until ctx is cancelled / Shutdown is called.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           mux,
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()
	return srv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// handle is the single dispatch entry point for all paths.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	// Recover from handler panics so a bug (e.g. a nil required TL field at
	// encode time) yields a clean 500 JSON response instead of killing the
	// connection (ECONNRESET on the client). Mirrors the official server's
	// behavior of never dropping a connection without a response.
	defer func() {
		if rec := recover(); rec != nil {
			botlog.Error("handler panic: %v\n%s", rec, debug.Stack())
			writeBody(w, http.StatusInternalServerError,
				response.Fail(500, "Internal Server Error", nil))
		}
	}()

	// Health endpoint.
	if r.URL.Path == "/" || r.URL.Path == "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "OK")
		return
	}

	// Stats endpoint.
	if r.URL.Path == "/stats" && s.stats != nil {
		s.stats.ServeHTTP(w, r)
		return
	}

	// File-download endpoint: /file/bot<token>/<file_path>. The file_path comes
	// from getFile's response; the file is served from the temp dir (written by
	// do_get_file). Mirrors the official local server's file serving.
	if strings.HasPrefix(r.URL.Path, "/file/") {
		s.handleFile(w, r)
		return
	}

	q, ok := parsePath(r)
	if !ok {
		writeBody(w, http.StatusNotFound,
			response.Fail(404, "Not Found", nil))
		return
	}
	// Parse body into args/files.
	// Cap the request body to prevent memory-exhaustion DoS. MaxBytesReader
	// causes the next io.ReadAll/ParseForm to return an error when the limit
	// is exceeded, which parseBody translates into a 400 response.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	if err := parseBody(r, q, s.cfg); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeBody(w, http.StatusRequestEntityTooLarge,
				response.Fail(http.StatusRequestEntityTooLarge, "Request Entity Too Large", nil))
			return
		}
		writeBody(w, http.StatusBadRequest,
			response.Fail(400, "Bad Request: "+err.Error(), nil))
		return
	}

	status, body := s.handler.Handle(r.Context(), q)
	writeBody(w, status, body)
}

// handleFile serves a downloaded file at /file/bot<token>/<file_path>.
// The token is validated and the file_path is scoped to the requesting bot's
// subdirectory under TempDir (<TempDir>/<botID>/). Symlinks are resolved to
// prevent escape via planted links on a shared TempDir.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/file/")
	idx := strings.IndexByte(rest, '/')
	if idx <= 0 || !strings.HasPrefix(rest, "bot") {
		writeBody(w, http.StatusNotFound, response.Fail(404, "Not Found", nil))
		return
	}
	token := rest[3:idx] // strip "bot" prefix
	relPath := rest[idx+1:]

	// Validate the token and extract the bot user ID. This prevents
	// cross-tenant file reads: only the bot that downloaded the file may
	// fetch it via /file/.
	botID, ok := fileBotID(token)
	if !ok {
		writeBody(w, http.StatusNotFound, response.Fail(404, "Not Found", nil))
		return
	}

	// Resolve TempDir to an absolute path for reliable containment checks.
	absTemp, err := filepath.Abs(s.cfg.TempDir)
	if err != nil {
		absTemp = s.cfg.TempDir
	}

	// Scope every candidate to <TempDir>/<botID>/ and verify via
	// EvalSymlinks + Rel so symlinks can't escape the bot's directory.
	botDir := filepath.Join(absTemp, botID)
	p := filepath.Join(botDir, relPath)
	if isPathContained(p, botDir) {
		if info, statErr := os.Stat(p); statErr == nil && !info.IsDir() {
			http.ServeFile(w, r, p)
			return
		}
	}
	writeBody(w, http.StatusNotFound, response.Fail(404, "Not Found", nil))
}

// fileBotID validates a bot token and returns its numeric user ID prefix.
// Returns ok=false for any token that is not <digits>:<secret>.
func fileBotID(token string) (string, bool) {
	prefix, _, hasColon := strings.Cut(token, ":")
	if !hasColon || prefix == "" || prefix[0] == '0' {
		return "", false
	}
	for _, c := range prefix {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return prefix, true
}

// isPathContained reports whether resolved is inside root, using symlink
// evaluation to defeat planted links. Both paths are cleaned first.
func isPathContained(resolved, root string) bool {
	resolved = filepath.Clean(resolved)
	root = filepath.Clean(root)
	// Resolve symlinks in the candidate path. If the file doesn't exist yet
	// (shouldn't happen here since os.Stat already verified it), fall back to
	// cleaning the directory portion.
	if real, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = real
	}
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	// rel must not start with ".." and must not be absolute.
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// parsePath extracts token, method and test-DC flag from the URL path.
// Accepts "/bot<token>/<method>" and "/bot<token>/test/<method>".
func parsePath(r *http.Request) (*Query, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/bot") {
		return nil, false
	}
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// First segment must start with "bot".
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "bot") {
		return nil, false
	}
	q := NewQuery()
	q.Token = strings.TrimPrefix(parts[0], "bot")
	q.PeerIP = peerIP(r)

	rest := parts[1:]
	if len(rest) >= 2 && rest[0] == "test" {
		q.TestDC = true
		q.Method = strings.ToLower(rest[1])
	} else {
		q.Method = strings.ToLower(rest[0])
	}
	if q.Method == "" {
		return nil, false
	}
	return q, true
}

// parseBody populates Args and Files from the request body. It handles query
// string (already in r.URL.Query()), application/x-www-form-urlencoded,
// multipart/form-data, and a single application/json object.
func parseBody(r *http.Request, q *Query, cfg Config) error {
	// Query string first (GET and POST forms).
	for key, vals := range r.URL.Query() {
		if len(vals) > 0 {
			q.Args[key] = vals[0]
		}
	}

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		return parseJSONBody(r, q)
	case strings.HasPrefix(ct, "multipart/form-data"):
		return parseMultipart(r, q, cfg)
	case ct == "application/x-www-form-urlencoded" || r.Method == http.MethodPost:
		// Standard form: let net/http parse, then copy into Args.
		if err := r.ParseForm(); err != nil {
			return err
		}
		for key, vals := range r.PostForm {
			if len(vals) > 0 {
				q.Args[key] = vals[0]
			}
		}
	}
	return nil
}

// parseJSONBody decodes a single JSON object into flat string args. Mirrors the
// official server's acceptance of an application/json request body.
//
// Key difference from json.Unmarshal: string values are decoded, but non-string
// values (numbers, booleans, arrays, objects) are preserved as raw JSON text,
// matching the official td::HttpReader::parse_json_parameters behavior.
func parseJSONBody(r *http.Request, q *Query) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	// Trim whitespace and check for string content (the official parser
	// handles this as a special "content" parameter).
	i := 0
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r') {
		i++
	}
	if i < len(raw) && raw[i] == '"' {
		// Single JSON string — store as "content" parameter.
		var s string
		if err := json.Unmarshal(raw[i:], &s); err != nil {
			return fmt.Errorf("can't parse string content: %w", err)
		}
		q.Args["content"] = s
		return nil
	}
	// Parse as JSON object, preserving raw JSON for non-string values.
	return parseJSONObject(raw[i:], q)
}

// parseJSONObject parses a JSON object, storing string values as decoded
// strings and non-string values as raw JSON text. Matches the official
// td::HttpReader::parse_json_parameters behavior.
func parseJSONObject(data []byte, q *Query) error {
	// Manual parser that preserves raw JSON for non-string values.
	i := 0
	// Skip whitespace.
	i = skipWS(data, i)
	if i >= len(data) || data[i] != '{' {
		return errors.New("JSON object expected")
	}
	i++ // skip '{'
	i = skipWS(data, i)

	for i < len(data) && data[i] != '}' {
		// Read key (must be a string).
		key, ni, err := parseJSONString(data, i)
		if err != nil {
			return fmt.Errorf("can't parse parameter name: %w", err)
		}
		i = ni
		i = skipWS(data, i)
		if i >= len(data) || data[i] != ':' {
			return fmt.Errorf("expected ':' after key %q", key)
		}
		i++ // skip ':'
		i = skipWS(data, i)

		// Read value.
		if i >= len(data) {
			return fmt.Errorf("expected value for key %q", key)
		}
		if data[i] == '"' {
			// String value — decode it.
			val, ni, err := parseJSONString(data, i)
			if err != nil {
				return fmt.Errorf("can't parse value for key %q: %w", key, err)
			}
			q.Args[key] = val
			i = ni
		} else {
			// Non-string value — preserve raw JSON.
			ni, err := skipJSONValue(data, i)
			if err != nil {
				return fmt.Errorf("can't parse value for key %q: %w", key, err)
			}
			q.Args[key] = string(data[i:ni])
			i = ni
		}
		i = skipWS(data, i)
		// Skip comma.
		if i < len(data) && data[i] == ',' {
			i++
			i = skipWS(data, i)
		}
	}
	return nil
}

func skipWS(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	return i
}

func parseJSONString(data []byte, i int) (string, int, error) {
	if i >= len(data) || data[i] != '"' {
		return "", i, errors.New("expected string")
	}
	i++ // skip opening quote
	var buf []byte
	for i < len(data) {
		if data[i] == '"' {
			return string(buf), i + 1, nil
		}
		if data[i] == '\\' && i+1 < len(data) {
			switch data[i+1] {
			case '"':
				buf = append(buf, '"')
			case '\\':
				buf = append(buf, '\\')
			case '/':
				buf = append(buf, '/')
			case 'n':
				buf = append(buf, '\n')
			case 'r':
				buf = append(buf, '\r')
			case 't':
				buf = append(buf, '\t')
			case 'b':
				buf = append(buf, '\b')
			case 'f':
				buf = append(buf, '\f')
			case 'u':
				// Unicode escape — simplified: just pass through the raw bytes.
				end := min(i+6, len(data))
				buf = append(buf, data[i:end]...)
				i = end
				continue
			default:
				buf = append(buf, data[i], data[i+1])
			}
			i += 2
			continue
		}
		buf = append(buf, data[i])
		i++
	}
	return "", i, errors.New("unterminated string")
}

// skipJSONValue skips over a JSON value (string, number, boolean, null, array, object)
// and returns the index after the value.
func skipJSONValue(data []byte, i int) (int, error) {
	if i >= len(data) {
		return i, errors.New("unexpected end of JSON")
	}
	switch data[i] {
	case '"':
		// String.
		_, ni, err := parseJSONString(data, i)
		return ni, err
	case '{':
		// Object.
		i++ // skip '{'
		i = skipWS(data, i)
		for i < len(data) && data[i] != '}' {
			// Skip key.
			_, ni, err := parseJSONString(data, i)
			if err != nil {
				return i, err
			}
			i = skipWS(data, ni)
			if i >= len(data) || data[i] != ':' {
				return i, errors.New("expected ':'")
			}
			i++ // skip ':'
			i = skipWS(data, i)
			// Skip value.
			ni, err = skipJSONValue(data, i)
			if err != nil {
				return i, err
			}
			i = skipWS(data, ni)
			if i < len(data) && data[i] == ',' {
				i++
				i = skipWS(data, i)
			}
		}
		if i >= len(data) {
			return i, errors.New("unterminated object")
		}
		return i + 1, nil // skip '}'
	case '[':
		// Array.
		i++ // skip '['
		i = skipWS(data, i)
		for i < len(data) && data[i] != ']' {
			ni, err := skipJSONValue(data, i)
			if err != nil {
				return i, err
			}
			i = skipWS(data, ni)
			if i < len(data) && data[i] == ',' {
				i++
				i = skipWS(data, i)
			}
		}
		if i >= len(data) {
			return i, errors.New("unterminated array")
		}
		return i + 1, nil // skip ']'
	default:
		// Number, boolean, null — skip until delimiter.
		for i < len(data) && data[i] != ',' && data[i] != '}' && data[i] != ']' &&
			data[i] != ' ' && data[i] != '\t' && data[i] != '\n' && data[i] != '\r' {
			i++
		}
		return i, nil
	}
}

// parseMultipart parses multipart/form-data by streaming each part directly,
// never buffering the whole body in memory: non-file fields are read into Args
// and every file part is written straight to a temp file in cfg.TempDir. This
// mirrors the official server, which streams every part into a TmpFile
// (HttpServer.cpp) instead of holding the body — and small files — in RAM. It
// replaces the prior r.ParseMultipartForm path, which buffered files <32 MB in
// memory and then re-copied them to disk. First value / first file per field
// name wins (matches FileHeader[0]); MaxFileCount caps the number of file
// fields. Temp files created before a parse error are removed so nothing leaks.
func parseMultipart(r *http.Request, q *Query, cfg Config) error {
	reader, err := r.MultipartReader()
	if err != nil {
		return err
	}

	var written []string
	cleanup := func() {
		for _, p := range written {
			_ = os.Remove(p)
		}
		for k := range q.Files {
			delete(q.Files, k)
		}
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return err
		}
		field := part.FormName()
		if field == "" {
			continue
		}
		if part.FileName() == "" {
			// Non-file form value: keep the first value for the field.
			if _, ok := q.Args[field]; ok {
				continue
			}
			val, err := io.ReadAll(part)
			if err != nil {
				cleanup()
				return err
			}
			q.Args[field] = string(val)
			continue
		}

		// File part: keep the first file per field name (matches FileHeader[0]).
		if _, ok := q.Files[field]; ok {
			continue
		}
		if len(q.Files) >= cfg.MaxFileCount {
			cleanup()
			return errTooManyFiles
		}
		name := part.FileName()
		path, n, err := persistMultipartPart(part, cfg.TempDir, name)
		if err != nil {
			cleanup()
			return err
		}
		written = append(written, path)
		q.Files[field] = File{
			FieldName: field,
			FileName:  name,
			TempPath:  path,
			MimeType:  part.Header.Get("Content-Type"),
			Size:      n,
		}
	}
	return nil
}

// persistMultipartPart copies a multipart part to a fresh temp file and returns
// its path and size.
func persistMultipartPart(src io.Reader, dir, name string) (string, int64, error) {
	tmp, err := os.CreateTemp(dir, "mgo-*"+sanitizeExt(name))
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(tmp, src)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", 0, err
	}
	return tmp.Name(), n, nil
}

func sanitizeExt(name string) string {
	ext := filepath.Ext(name)
	// Only keep a short, safe extension.
	return ext[:min(8, len(ext))]
}

// peerIP extracts the client IP (honouring X-Forwarded-For if present).
func peerIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(xff)
	}
	// Trim port.
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

var errTooManyFiles = &multipartError{"too many files uploaded"}

type multipartError struct{ msg string }

func (e *multipartError) Error() string { return e.msg }
