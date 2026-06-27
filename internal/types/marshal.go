package types

import "encoding/json"

// jsonField is one ordered key/value pair for marshalOrdered.
type jsonField struct {
	key string
	val any
}

// marshalOrdered emits a JSON object with the given fields in slice order,
// bypassing Go struct field order. Each val is marshaled with encoding/json
// (bools → true/false, *User → its own JSON, etc.). Used by types whose official
// Client.cpp emission order or field set is status/variant-dependent and cannot
// be expressed by a single Go struct.
func marshalOrdered(fields []jsonField) ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')
	for i, f := range fields {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, err := json.Marshal(f.key)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		vb, err := json.Marshal(f.val)
		if err != nil {
			return nil, err
		}
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return buf, nil
}
