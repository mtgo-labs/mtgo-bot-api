package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

// Regression: cashtag/phone entities are emitted (not dropped) and the
// blockquote "collapsed" flag maps to "expandable_blockquote" (Client.cpp:532-616).
func TestMessageEntityCases(t *testing.T) {
	tests := []struct {
		name string
		in   tg.MessageEntityClass
		typ  string
	}{
		{"cashtag", &tg.MessageEntityCashtag{Offset: 0, Length: 5}, "cashtag"},
		{"phone", &tg.MessageEntityPhone{Offset: 0, Length: 11}, "phone_number"},
		{"blockquote", &tg.MessageEntityBlockquote{Offset: 0, Length: 1}, "blockquote"},
		{"expandable_blockquote", &tg.MessageEntityBlockquote{Collapsed: true, Offset: 0, Length: 1}, "expandable_blockquote"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := messageEntity(tc.in)
			if e == nil {
				t.Fatalf("entity dropped (nil) for %s", tc.name)
			}
			if e.Type != tc.typ {
				t.Errorf("type = %q, want %q", e.Type, tc.typ)
			}
		})
	}
}
