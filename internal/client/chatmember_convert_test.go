package client

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
)

// Regression: convertMtgoChatMember routes the tag value (mtgo CustomTitle, from
// TL rank) under "custom_title" for creator/admin and "tag" for member/restricted,
// and populates until_date / is_member.
func TestConvertMtgoChatMemberPerStatus(t *testing.T) {
	until := time.Unix(1700000000, 0)
	sg := apitypes.ChatTypeSupergroup

	creator := convertMtgoChatMember(&mtgotypes.ChatMember{Status: mtgotypes.ChatMemberStatusOwner, CustomTitle: "boss"}, sg)
	if creator.Status != "creator" || creator.CustomTitle != "boss" || creator.Tag != "" {
		t.Errorf("creator: status=%q custom_title=%q tag=%q", creator.Status, creator.CustomTitle, creator.Tag)
	}

	admin := convertMtgoChatMember(&mtgotypes.ChatMember{Status: mtgotypes.ChatMemberStatusAdministrator, CustomTitle: "adm"}, sg)
	if admin.Status != "administrator" || admin.CustomTitle != "adm" || admin.Tag != "" {
		t.Errorf("admin: status=%q custom_title=%q tag=%q", admin.Status, admin.CustomTitle, admin.Tag)
	}

	member := convertMtgoChatMember(&mtgotypes.ChatMember{Status: mtgotypes.ChatMemberStatusMember, CustomTitle: "m"}, sg)
	if member.Status != "member" || member.Tag != "m" || member.CustomTitle != "" {
		t.Errorf("member: status=%q tag=%q custom_title=%q", member.Status, member.Tag, member.CustomTitle)
	}

	restricted := convertMtgoChatMember(&mtgotypes.ChatMember{
		Status: mtgotypes.ChatMemberStatusRestricted, CustomTitle: "r", IsMember: true, UntilDate: until,
	}, sg)
	if restricted.Status != "restricted" || restricted.Tag != "r" || !restricted.IsMember || restricted.UntilDate != until.Unix() {
		t.Errorf("restricted: status=%q tag=%q is_member=%v until=%d want=%d",
			restricted.Status, restricted.Tag, restricted.IsMember, restricted.UntilDate, until.Unix())
	}

	kicked := convertMtgoChatMember(&mtgotypes.ChatMember{Status: mtgotypes.ChatMemberStatusBanned, UntilDate: until}, sg)
	if kicked.Status != "kicked" || kicked.UntilDate != until.Unix() || kicked.Tag != "" {
		t.Errorf("kicked: status=%q until=%d tag=%q", kicked.Status, kicked.UntilDate, kicked.Tag)
	}
}

// Regression: ChatMember.MarshalJSON emits the admin rights chat-type-gated
// (json_store_administrator_rights, Client.cpp:17461) and the restricted
// permissions block only for supergroups (json_store_permissions, :17492).
func TestChatMemberMarshalGating(t *testing.T) {
	admin := apitypes.ChatMember{User: &apitypes.User{ID: 1}, Status: "administrator", CanPostMessages: true, CanPinMessages: true}

	admin.SetChatType(apitypes.ChatTypeChannel)
	ch := mustJSON(t, admin)
	if !strings.Contains(ch, "can_post_messages") {
		t.Errorf("channel admin must emit can_post_messages: %s", ch)
	}
	if strings.Contains(ch, "can_pin_messages") {
		t.Errorf("channel admin must NOT emit can_pin_messages: %s", ch)
	}

	admin.SetChatType(apitypes.ChatTypeSupergroup)
	sg := mustJSON(t, admin)
	if !strings.Contains(sg, "can_pin_messages") {
		t.Errorf("supergroup admin must emit can_pin_messages: %s", sg)
	}
	if strings.Contains(sg, "can_post_messages") {
		t.Errorf("supergroup admin must NOT emit can_post_messages: %s", sg)
	}

	// Restricted supergroup emits the permissions block; a basic group does not.
	restr := apitypes.ChatMember{User: &apitypes.User{ID: 1}, Status: "restricted"}
	restr.SetChatType(apitypes.ChatTypeSupergroup)
	if j := mustJSON(t, restr); !strings.Contains(j, "can_send_messages") {
		t.Errorf("restricted supergroup must emit the permissions block: %s", j)
	}
	restr.SetChatType(apitypes.ChatTypeGroup)
	if j := mustJSON(t, restr); strings.Contains(j, "can_send_messages") {
		t.Errorf("restricted basic group must NOT emit the permissions block: %s", j)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
