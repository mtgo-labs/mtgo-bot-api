package fileid

import "testing"

// FuzzDecode fuzzes fileid.Decode with arbitrary strings. It must never panic;
// invalid input should return an error, not crash.
func FuzzDecode(f *testing.F) {
	f.Add("AQADBAADeP8GEg") // valid file_id
	f.Add("invalid")
	f.Add("")
	f.Add("AAAAAA") // valid base64url but too short to decode
	f.Add("___-")   // valid base64url padding chars
	f.Add("Dw")     // minimal valid base64
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Decode(s) // must not panic
	})
}
