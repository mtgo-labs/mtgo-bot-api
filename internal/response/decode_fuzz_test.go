package response

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"ok":true,"result":{"id":1}}`))
	f.Add([]byte(`{"ok":false,"error_code":400,"description":"Bad Request"}`))
	f.Add([]byte(`not-json`))
	f.Add([]byte(`{"ok":true,"result":[1,2,3],"description":"done"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _, _, _ = Decode(data)
	})
}
