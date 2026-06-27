package types

import (
	"bytes"
	"encoding/json"
)

// ForceFloat is a float64 that always serializes to JSON with a decimal point,
// matching the reference (TDLib) double formatting: whole numbers render as
// "500.0" rather than Go's default "500". Non-whole values serialize exactly as
// Go's default (which already matches the reference). Use for Bot API double
// fields that can be whole numbers (coordinates, horizontal_accuracy, …).
type ForceFloat float64

// MarshalJSON renders the value with Go's default float formatting, appending
// ".0" when the result would otherwise be a bare integer.
func (f ForceFloat) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(float64(f))
	if err != nil {
		return nil, err
	}
	if !bytes.ContainsAny(b, ".eE") {
		b = append(b, '.', '0')
	}
	return b, nil
}
