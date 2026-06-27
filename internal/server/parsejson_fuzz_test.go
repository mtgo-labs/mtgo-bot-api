package server

import "testing"

// FuzzParseJSONObject fuzzes the hand-written JSON object parser with arbitrary
// input. It must never panic on malformed or adversarial JSON-like data.
func FuzzParseJSONObject(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`bad`))
	f.Add([]byte(`{"nested":{"a":1},"arr":[1,2,3],"str":"hi","num":42,"bool":true,"null":null}`))
	f.Add([]byte(`{"empty_arr":[],"empty_obj":{}}`))
	f.Add([]byte(`{"unicode":"\u00e9"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		q := NewQuery()
		_ = parseJSONObject(data, q) // must not panic
	})
}
