package server

import (
	"fmt"
	"strconv"
)

// File represents an uploaded multipart file, mirroring td::HttpFile.
type File struct {
	FieldName string // form field name
	FileName  string // original file name
	TempPath  string // path to the temp file on disk
	MimeType  string
	Size      int64
}

// Query is a parsed Bot API HTTP request, mirroring telegram-bot-api/Query.
// Method/Token are derived from the path; Args/Headers/Files from the body.
type Query struct {
	Token   string
	Method  string
	TestDC  bool
	Args    map[string]string
	Headers map[string]string
	Files   map[string]File
	PeerIP  string
}

// NewQuery returns an empty Query with initialised maps.
func NewQuery() *Query {
	return &Query{
		Args:    make(map[string]string),
		Headers: make(map[string]string),
		Files:   make(map[string]File),
	}
}

// HasArg reports whether key is present.
func (q *Query) HasArg(key string) bool { _, ok := q.Args[key]; return ok }

// Arg returns the string value of a parameter, or "" if absent.
func (q *Query) Arg(key string) string { return q.Args[key] }

// ArgInt64 parses an integer parameter.
func (q *Query) ArgInt64(key string) (int64, error) {
	v := q.Args[key]
	if v == "" {
		return 0, fmt.Errorf("missing parameter %q", key)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parameter %q must be an integer", key)
	}
	return n, nil
}

// ArgBool parses a boolean parameter ("true"/"false"/"1"/"0").
func (q *Query) ArgBool(key string) bool {
	v := q.Args[key]
	return v == "true" || v == "1"
}

// File returns the uploaded file for key, if any.
func (q *Query) File(key string) (File, bool) {
	f, ok := q.Files[key]
	return f, ok
}

// Header returns a header value by key (case-insensitive lookup expected by caller).
func (q *Query) Header(key string) string { return q.Headers[key] }
