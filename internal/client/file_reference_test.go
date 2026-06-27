package client

import (
	"errors"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
	"github.com/mtgo-labs/mtgo/tgerr"
)

func TestIsFileReferenceExpired(t *testing.T) {
	if !isFileReferenceExpired(tgerr.New(400, "FILE_REFERENCE_EXPIRED")) {
		t.Error("FILE_REFERENCE_EXPIRED should match")
	}
	if isFileReferenceExpired(tgerr.New(400, "FILE_ID_INVALID")) {
		t.Error("unrelated 400 should not match")
	}
	if isFileReferenceExpired(errors.New("network down")) {
		t.Error("non-tgerr error should not match")
	}
}

func TestExtractFreshReference(t *testing.T) {
	msgs := []tg.MessageClass{
		&tg.Message{
			ID: 100,
			Media: &tg.MessageMediaDocument{Document: &tg.Document{
				ID: 777, FileReference: []byte("fresh-doc-ref"),
			}},
		},
		&tg.Message{
			ID: 200,
			Media: &tg.MessageMediaPhoto{Photo: &tg.Photo{
				ID: 888, FileReference: []byte("fresh-photo-ref"),
			}},
		},
	}

	if ref := extractFreshReference(msgs, 100, 777); string(ref) != "fresh-doc-ref" {
		t.Errorf("doc ref = %q, want fresh-doc-ref", ref)
	}
	if ref := extractFreshReference(msgs, 200, 888); string(ref) != "fresh-photo-ref" {
		t.Errorf("photo ref = %q, want fresh-photo-ref", ref)
	}
	if ref := extractFreshReference(msgs, 100, 999); ref != nil {
		t.Errorf("mismatching media id should return nil, got %q", ref)
	}
	if ref := extractFreshReference(msgs, 999, 777); ref != nil {
		t.Errorf("missing message should return nil, got %q", ref)
	}
}

func TestMessagesClassToList(t *testing.T) {
	want := []tg.MessageClass{&tg.Message{ID: 5}}
	for name, res := range map[string]tg.MessagesClass{
		"messages":        &tg.MessagesMessages{Messages: want},
		"slice":           &tg.MessagesMessagesSlice{Messages: want},
		"channelMessages": &tg.MessagesChannelMessages{Messages: want},
	} {
		got := messagesClassToList(res)
		if len(got) != 1 {
			t.Errorf("%s: got %d messages, want 1", name, len(got))
			continue
		}
		m, ok := got[0].(*tg.Message)
		if !ok || m.ID != 5 {
			t.Errorf("%s: unexpected message %+v", name, got[0])
		}
	}
	if got := messagesClassToList(&tg.MessagesMessagesNotModified{}); got != nil {
		t.Errorf("NotModified should yield nil, got %v", got)
	}
}

func TestMediaIDsFromMessage(t *testing.T) {
	cases := []struct {
		name string
		m    *tg.Message
		want []int64
	}{
		{"nil", nil, nil},
		{"no media", &tg.Message{ID: 1}, nil},
		{"document", &tg.Message{ID: 2, Media: &tg.MessageMediaDocument{Document: &tg.Document{ID: 42}}}, []int64{42}},
		{"photo", &tg.Message{ID: 3, Media: &tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 7}}}, []int64{7}},
		{"non-file media", &tg.Message{ID: 4, Media: &tg.MessageMediaContact{PhoneNumber: "+1"}}, nil},
	}
	for _, tc := range cases {
		got := mediaIDsFromMessage(tc.m)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		}
	}
}
