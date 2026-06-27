package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestPermissionsAreFullyOpen_AllTrue(t *testing.T) {
	p := &apitypes.ChatPermissions{
		CanSendMessages: true, CanSendAudios: true, CanSendDocuments: true,
		CanSendPhotos: true, CanSendVideos: true, CanSendVideoNotes: true,
		CanSendVoiceNotes: true, CanSendPolls: true, CanSendOtherMessages: true,
		CanAddWebPagePreviews: true, CanReactToMessages: true, CanEditTag: true,
		CanChangeInfo: true, CanInviteUsers: true, CanPinMessages: true,
	}
	if !permissionsAreFullyOpen(p) {
		t.Error("all-true permissions should be fully open")
	}
}

func TestPermissionsAreFullyOpen_Partial(t *testing.T) {
	p := &apitypes.ChatPermissions{
		CanSendMessages: true, CanSendAudios: true, CanSendDocuments: true,
		CanSendPhotos: true, CanSendVideos: true, CanSendVideoNotes: true,
		CanSendVoiceNotes: true, CanSendPolls: true, CanSendOtherMessages: true,
		CanAddWebPagePreviews: true, CanReactToMessages: true, CanEditTag: true,
		CanChangeInfo: true, CanInviteUsers: true,
		// CanPinMessages is false → not fully open.
	}
	if permissionsAreFullyOpen(p) {
		t.Error("missing CanPinMessages should not be fully open")
	}
}

func TestPermissionsAreFullyOpen_Nil(t *testing.T) {
	if permissionsAreFullyOpen(nil) {
		t.Error("nil should return false")
	}
}

func TestBannedRightsToPermissions_NotBanned(t *testing.T) {
	// ChatBannedRights inversion: SendMessages=false (not banned) → CanSendMessages=true.
	br := &tg.ChatBannedRights{
		SendMessages: false,
		SendMedia:    false,
	}
	perms := BannedRightsToPermissions(br)
	if perms == nil {
		t.Fatal("nil output")
	}
	if !perms.CanSendMessages {
		t.Error("CanSendMessages should be true when SendMessages is false (not banned)")
	}
}

func TestBannedRightsToPermissions_Banned(t *testing.T) {
	br := &tg.ChatBannedRights{
		SendMessages: true, // banned → CanSendMessages=false
	}
	perms := BannedRightsToPermissions(br)
	if perms.CanSendMessages {
		t.Error("CanSendMessages should be false when banned")
	}
}

func TestBannedRightsToPermissions_Nil(t *testing.T) {
	perms := BannedRightsToPermissions(nil)
	if perms != nil {
		t.Error("nil banned rights should return nil")
	}
}

func TestAdminRightsFromParams_AllTrue(t *testing.T) {
	params := map[string]bool{
		"can_change_info": true, "can_delete_messages": true,
		"can_restrict_members": true, "can_invite_users": true,
		"can_pin_messages": true, "can_manage_topics": true,
	}
	rights := AdminRightsFromParams(params)
	if rights == nil {
		t.Fatal("nil output")
	}
	if !rights.ChangeInfo {
		t.Error("ChangeInfo should be true")
	}
	if !rights.DeleteMessages {
		t.Error("DeleteMessages should be true")
	}
}

func TestAdminRightsFromParams_Empty(t *testing.T) {
	rights := AdminRightsFromParams(map[string]bool{})
	if rights == nil {
		t.Fatal("nil output for empty params")
	}
	// All should be false.
	if rights.ChangeInfo || rights.DeleteMessages {
		t.Error("empty params should produce all-false rights")
	}
}

func TestAdminRightsFromParams_Nil(t *testing.T) {
	rights := AdminRightsFromParams(nil)
	if rights == nil {
		t.Fatal("nil output for nil params")
	}
}
