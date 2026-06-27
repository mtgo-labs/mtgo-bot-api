package client

import (
	"testing"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestChatTypeFromID(t *testing.T) {
	tests := []struct {
		id   int64
		want apitypes.ChatType
	}{
		{123, apitypes.ChatTypePrivate},
		{-999, apitypes.ChatTypeGroup},
		{-1000000000000, apitypes.ChatTypeSupergroup},
		{-1001234567890, apitypes.ChatTypeSupergroup},
	}
	for _, tt := range tests {
		got := chatTypeFromID(tt.id)
		if got != tt.want {
			t.Errorf("chatTypeFromID(%d) = %s, want %s", tt.id, got, tt.want)
		}
	}
}

func TestNewError(t *testing.T) {
	e := NewError(400, "Bad Request: test error")
	if e.Code != 400 {
		t.Errorf("Code = %d, want 400", e.Code)
	}
	if e.Description != "Bad Request: test error" {
		t.Errorf("Description = %s", e.Description)
	}
}

func TestNewError_InternalError(t *testing.T) {
	e := NewError(500, "internal failure")
	if e.Code != 500 {
		t.Errorf("Code = %d, want 500", e.Code)
	}
}

func TestUserIDFromUser(t *testing.T) {
	// nil → 0
	if got := userIDFromUser(nil); got != 0 {
		t.Errorf("nil user should return 0, got %d", got)
	}
}

func TestPollOptionIDs(t *testing.T) {
	// Empty → empty
	if ids := pollOptionIDs(nil); len(ids) != 0 {
		t.Error("nil options should return empty")
	}
	// Single byte options
	opts := [][]byte{{0x00}, {0x01}, {0x02}}
	ids := pollOptionIDs(opts)
	if len(ids) != 3 || ids[0] != 0 || ids[1] != 1 || ids[2] != 2 {
		t.Errorf("pollOptionIDs = %v, want [0 1 2]", ids)
	}
}
