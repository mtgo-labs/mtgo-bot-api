package server

import "testing"

func TestNewQuery(t *testing.T) {
	q := NewQuery()
	if q.Args == nil || q.Headers == nil || q.Files == nil {
		t.Error("NewQuery must initialize all maps")
	}
}

func TestQuery_HasArg(t *testing.T) {
	q := NewQuery()
	q.Args["chat_id"] = "123"
	if !q.HasArg("chat_id") {
		t.Error("HasArg should return true for present key")
	}
	if q.HasArg("missing") {
		t.Error("HasArg should return false for absent key")
	}
}

func TestQuery_Arg(t *testing.T) {
	q := NewQuery()
	q.Args["text"] = "hello"
	if q.Arg("text") != "hello" {
		t.Errorf("Arg = %q, want 'hello'", q.Arg("text"))
	}
	if q.Arg("missing") != "" {
		t.Error("absent key should return empty string")
	}
}

func TestQuery_ArgInt64(t *testing.T) {
	q := NewQuery()
	q.Args["id"] = "42"
	n, err := q.ArgInt64("id")
	if err != nil || n != 42 {
		t.Errorf("ArgInt64 = %d, err=%v; want 42, nil", n, err)
	}
	// Missing.
	_, err = q.ArgInt64("absent")
	if err == nil {
		t.Error("missing key should return error")
	}
	// Invalid.
	q.Args["bad"] = "not_a_number"
	_, err = q.ArgInt64("bad")
	if err == nil {
		t.Error("non-integer should return error")
	}
	// Negative.
	q.Args["neg"] = "-100"
	n, err = q.ArgInt64("neg")
	if err != nil || n != -100 {
		t.Errorf("negative: got %d, err=%v; want -100", n, err)
	}
}

func TestQuery_ArgBool(t *testing.T) {
	q := NewQuery()
	tests := []struct {
		val  string
		want bool
	}{
		{"true", true}, {"1", true},
		{"false", false}, {"0", false}, {"", false}, {"yes", false},
	}
	for _, tt := range tests {
		q.Args["flag"] = tt.val
		if got := q.ArgBool("flag"); got != tt.want {
			t.Errorf("ArgBool(%q) = %v, want %v", tt.val, got, tt.want)
		}
	}
}

func TestQuery_Header(t *testing.T) {
	q := NewQuery()
	q.Headers["X-Custom"] = "value"
	if q.Header("X-Custom") != "value" {
		t.Errorf("Header = %q", q.Header("X-Custom"))
	}
	if q.Header("Missing") != "" {
		t.Error("absent header should return empty")
	}
}

func TestQuery_File(t *testing.T) {
	q := NewQuery()
	q.Files["photo"] = File{FileName: "test.jpg", Size: 1024}
	f, ok := q.File("photo")
	if !ok || f.FileName != "test.jpg" || f.Size != 1024 {
		t.Errorf("File = %+v, ok=%v", f, ok)
	}
	_, ok = q.File("absent")
	if ok {
		t.Error("absent file should return ok=false")
	}
}
