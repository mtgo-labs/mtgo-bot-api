package types

import "testing"

func TestForceFloatFormatting(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{500, "500.0"},          // whole number gets ".0" (TDLib style)
		{0, "0.0"},              // zero
		{40.712787, "40.712787"}, // fractional unchanged
		{-74.00603, "-74.00603"},
		{-5, "-5.0"},
	}
	for _, c := range cases {
		b, err := ForceFloat(c.in).MarshalJSON()
		if err != nil {
			t.Fatalf("marshal %v: %v", c.in, err)
		}
		if string(b) != c.want {
			t.Errorf("ForceFloat(%v) = %s, want %s", c.in, b, c.want)
		}
	}
}
