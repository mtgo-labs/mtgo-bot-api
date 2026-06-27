package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHandler records the last query and returns a canned response.
type fakeHandler struct {
	last *Query
}

func (f *fakeHandler) Handle(_ context.Context, q *Query) (int, []byte) {
	f.last = q
	return 200, []byte(`{"ok":true,"result":1}`)
}

func TestHealth(t *testing.T) {
	s := New(Config{Addr: "", TempDir: t.TempDir()}, &fakeHandler{})
	rec := doRequest(t, s, "GET", "/", nil, "")
	if rec.Code != 200 || rec.Body.String() != "OK" {
		t.Fatalf("health = %d %q, want 200 OK", rec.Code, rec.Body.String())
	}
}

func TestParsePathTokenAndMethod(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)
	doRequest(t, s, "POST", "/bot123:ABC/getMe", nil, "")
	if h.last == nil {
		t.Fatal("handler not invoked")
	}
	if h.last.Token != "123:ABC" {
		t.Errorf("token = %q, want 123:ABC", h.last.Token)
	}
	if h.last.Method != "getme" {
		t.Errorf("method = %q, want getme (lowercased)", h.last.Method)
	}
	if h.last.TestDC {
		t.Error("TestDC should be false")
	}
}

func TestParsePathTestDC(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)
	doRequest(t, s, "POST", "/bot123:ABC/test/sendMessage", nil, "")
	if h.last == nil || !h.last.TestDC || h.last.Method != "sendmessage" {
		t.Fatalf("test-DC parse wrong: %+v", h.last)
	}
}

func TestInvalidPath(t *testing.T) {
	s := New(Config{TempDir: t.TempDir()}, &fakeHandler{})
	rec := doRequest(t, s, "GET", "/notabotpath", nil, "")
	if rec.Code != 404 {
		t.Fatalf("invalid path status = %d, want 404", rec.Code)
	}
	var got struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid error envelope: %v: %s", err, rec.Body.String())
	}
	if got.Ok || got.ErrorCode != 404 || got.Description != "Not Found" {
		t.Fatalf("invalid path envelope = %+v, want 404 Not Found", got)
	}
}

func TestQueryAndFormParams(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)
	rec := doRequest(t, s, "POST", "/bot1:t/getUpdates?offset=5&limit=10",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "timeout=30")
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if h.last.Args["offset"] != "5" || h.last.Args["limit"] != "10" || h.last.Args["timeout"] != "30" {
		t.Errorf("args = %+v", h.last.Args)
	}
}

func TestJSONBodyParams(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)
	body := `{"chat_id":42,"text":"hi"}`
	rec := doRequest(t, s, "POST", "/bot1:t/sendMessage",
		map[string]string{"Content-Type": "application/json"}, body)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if h.last.Args["chat_id"] != "42" || h.last.Args["text"] != "hi" {
		t.Errorf("json args = %+v", h.last.Args)
	}
}

func TestMultipartUpload(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", "99")
	file, _ := mw.CreateFormFile("photo", "pic.jpg")
	_, _ = file.Write([]byte("binarydata"))
	_ = mw.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/bot1:t/sendPhoto", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	s.handle(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if h.last.Args["chat_id"] != "99" {
		t.Errorf("chat_id arg = %q", h.last.Args["chat_id"])
	}
	f, ok := h.last.File("photo")
	if !ok {
		t.Fatal("photo file not parsed")
	}
	if f.FileName != "pic.jpg" || f.TempPath == "" {
		t.Errorf("file = %+v", f)
	}
}

// TestMultipartStreamedToDisk asserts that file parts are streamed straight to
// a temp file (F5): the parsed File has the right size and the temp file on disk
// holds the uploaded bytes verbatim — the body was never buffered whole in RAM.
func TestMultipartStreamedToDisk(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)

	payload := bytes.Repeat([]byte("x"), 4096)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("caption", "hi")
	fw, _ := mw.CreateFormFile("document", "data.bin")
	_, _ = fw.Write(payload)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/bot1:t/sendDocument", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	s.handle(httptest.NewRecorder(), req)

	f, ok := h.last.File("document")
	if !ok {
		t.Fatal("document file not parsed")
	}
	if f.Size != int64(len(payload)) {
		t.Errorf("Size = %d, want %d", f.Size, len(payload))
	}
	got, err := os.ReadFile(f.TempPath)
	if err != nil {
		t.Fatalf("temp file unreadable: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("temp file content mismatch: got %d bytes, want %d", len(got), len(payload))
	}
	if h.last.Args["caption"] != "hi" {
		t.Errorf("caption = %q", h.last.Args["caption"])
	}
}

// TestMultipartFirstFilePerField verifies only the first file under a repeated
// field name is kept (matches FileHeader[0]).
func TestMultipartFirstFilePerField(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir()}, h)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, name := range []string{"a.png", "b.png"} {
		fw, _ := mw.CreateFormFile("photo", name)
		_, _ = fw.Write([]byte(name))
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/bot1:t/sendPhoto", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	s.handle(httptest.NewRecorder(), req)

	f := h.last.Files["photo"]
	if f.FileName != "a.png" {
		t.Errorf("kept file = %q, want a.png", f.FileName)
	}
	if len(h.last.Files) != 1 {
		t.Errorf("file fields = %d, want 1", len(h.last.Files))
	}
}

// TestMultipartMaxFileCount verifies the file-count cap rejects with 400 and
// cleans up any temp files already written (no disk leak).
func TestMultipartMaxFileCount(t *testing.T) {
	dir := t.TempDir()
	h := &fakeHandler{}
	s := New(Config{TempDir: dir, MaxFileCount: 2}, h)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, name := range []string{"f1", "f2", "f3"} {
		fw, _ := mw.CreateFormFile(name, name+".bin")
		_, _ = fw.Write([]byte("data-" + name))
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/bot1:t/sendMediaGroup", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.handle(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 (too many files)", rec.Code)
	}
	// parseMultipart rejects at parse time, so the handler is never invoked.
	if h.last != nil {
		t.Errorf("handler should not be called on rejection, got %+v", h.last)
	}
	// No temp files should remain in TempDir after the rejection (cleanup ran).
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("temp files leaked after rejection: %d entries", len(entries))
	}
}

func TestMaxBodyBytesReturns413(t *testing.T) {
	h := &fakeHandler{}
	s := New(Config{TempDir: t.TempDir(), MaxBodyBytes: 4}, h)
	rec := doRequest(t, s, "POST", "/bot1:t/sendMessage",
		map[string]string{"Content-Type": "application/json"}, `{"x":"too long"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
	if h.last != nil {
		t.Fatalf("handler should not be called for oversized body, got %+v", h.last)
	}
}

func TestHandleFileRejectsCrossBotPath(t *testing.T) {
	dir := t.TempDir()
	botADir := filepath.Join(dir, "111")
	botBDir := filepath.Join(dir, "222")
	if err := os.MkdirAll(botADir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(botBDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(botADir, "own.txt"), []byte("own"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(botBDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(Config{TempDir: dir}, &fakeHandler{})
	own := doRequest(t, s, "GET", "/file/bot111:token/own.txt", nil, "")
	if own.Code != http.StatusOK || own.Body.String() != "own" {
		t.Fatalf("own file = %d %q, want 200 own", own.Code, own.Body.String())
	}

	cross := doRequest(t, s, "GET", "/file/bot111:token/222/secret.txt", nil, "")
	if cross.Code != http.StatusNotFound {
		t.Fatalf("cross-bot status = %d, want 404; body=%s", cross.Code, cross.Body.String())
	}
}

func TestHandleFileRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	botDir := filepath.Join(dir, "111")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(botDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(botDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	s := New(Config{TempDir: dir}, &fakeHandler{})
	rec := doRequest(t, s, "GET", "/file/bot111:token/link.txt", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("symlink escape status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

// doRequest invokes the server's handler directly via httptest.
func doRequest(t *testing.T, s *Server, method, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	var req *http.Request
	if rdr != nil {
		req = httptest.NewRequest(method, path, rdr)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handle(rec, req)
	return rec
}
