package types

// MarshalJSON emits ChatMember in the official Client.cpp JsonChatMember::store
// (5712) order: user, status, then per-status fields. The administrator rights
// (json_store_administrator_rights, Client.cpp:17461) and restricted permissions
// (json_store_permissions, :17492) are emitted UNCONDITIONALLY (not omitempty)
// and chat-type-gated, exactly as the reference. custom_title (creator/admin)
// and tag (member/restricted) come from the same value but are placed per status.
//
// chatType (set via SetChatType) selects the gate set. An unset chatType
// defaults to "supergroup" — the dominant admin context (parseChatID collapses
// supergroup+channel into one path; megagroups dominate). Broadcast channels
// should SetChatType("channel") for correct can_post_messages/can_edit_messages
// /can_manage_direct_messages gating.
func (m ChatMember) MarshalJSON() ([]byte, error) {
	ct := m.chatType
	if ct == "" {
		ct = ChatTypeSupergroup
	}
	f := make([]jsonField, 0, 32)
	if m.User != nil {
		f = append(f, jsonField{"user", m.User})
	}
	f = append(f, jsonField{"status", m.Status})
	switch m.Status {
	case "creator":
		if m.CustomTitle != "" {
			f = append(f, jsonField{"custom_title", m.CustomTitle})
		}
		f = append(f, jsonField{"is_anonymous", m.IsAnonymous})
	case "administrator":
		f = append(f, jsonField{"can_be_edited", m.CanBeEdited})
		f = appendChatAdminRights(f, m, ct)
		f = append(f, jsonField{"can_manage_voice_chats", m.CanManageVideoChats})
		if m.CustomTitle != "" {
			f = append(f, jsonField{"custom_title", m.CustomTitle})
		}
	case "member":
		if m.Tag != "" {
			f = append(f, jsonField{"tag", m.Tag})
		}
		if m.UntilDate > 0 {
			f = append(f, jsonField{"until_date", m.UntilDate})
		}
	case "restricted":
		if m.Tag != "" {
			f = append(f, jsonField{"tag", m.Tag})
		}
		if ct == ChatTypeSupergroup { // Client.cpp:5767 — perms only for supergroups
			f = append(f, jsonField{"until_date", m.UntilDate})
			f = appendChatPermissions(f, m)
			f = append(f, jsonField{"is_member", m.IsMember})
		}
	case "kicked":
		f = append(f, jsonField{"until_date", m.UntilDate})
	}
	return marshalOrdered(f)
}

// appendChatAdminRights emits json_store_administrator_rights (Client.cpp:17461)
// in order, chat-type-gated.
func appendChatAdminRights(f []jsonField, m ChatMember, ct ChatType) []jsonField {
	f = append(f, jsonField{"can_manage_chat", m.CanManageChat})
	f = append(f, jsonField{"can_change_info", m.CanChangeInfo})
	if ct == ChatTypeChannel {
		f = append(f, jsonField{"can_post_messages", m.CanPostMessages})
		f = append(f, jsonField{"can_edit_messages", m.CanEditMessages})
	}
	f = append(f, jsonField{"can_delete_messages", m.CanDeleteMessages})
	f = append(f, jsonField{"can_invite_users", m.CanInviteUsers})
	f = append(f, jsonField{"can_restrict_members", m.CanRestrictMembers})
	if ct == ChatTypeGroup || ct == ChatTypeSupergroup {
		f = append(f, jsonField{"can_pin_messages", m.CanPinMessages})
	}
	if ct == ChatTypeSupergroup {
		f = append(f, jsonField{"can_manage_topics", m.CanManageTopics})
	}
	f = append(f, jsonField{"can_promote_members", m.CanPromoteMembers})
	f = append(f, jsonField{"can_manage_video_chats", m.CanManageVideoChats})
	f = append(f, jsonField{"can_post_stories", m.CanPostStories})
	f = append(f, jsonField{"can_edit_stories", m.CanEditStories})
	f = append(f, jsonField{"can_delete_stories", m.CanDeleteStories})
	if ct == ChatTypeChannel {
		f = append(f, jsonField{"can_manage_direct_messages", m.CanManageDirectMessages})
	}
	if ct == ChatTypeGroup || ct == ChatTypeSupergroup {
		f = append(f, jsonField{"can_manage_tags", m.CanManageTags})
	}
	f = append(f, jsonField{"is_anonymous", m.IsAnonymous})
	return f
}

// appendChatPermissions emits json_store_permissions (Client.cpp:17492) in order.
func appendChatPermissions(f []jsonField, m ChatMember) []jsonField {
	f = append(f, jsonField{"can_send_messages", m.CanSendMessages})
	f = append(f, jsonField{"can_send_media_messages", m.CanSendMediaMessages})
	f = append(f, jsonField{"can_send_audios", m.CanSendAudios})
	f = append(f, jsonField{"can_send_documents", m.CanSendDocuments})
	f = append(f, jsonField{"can_send_photos", m.CanSendPhotos})
	f = append(f, jsonField{"can_send_videos", m.CanSendVideos})
	f = append(f, jsonField{"can_send_video_notes", m.CanSendVideoNotes})
	f = append(f, jsonField{"can_send_voice_notes", m.CanSendVoiceNotes})
	f = append(f, jsonField{"can_send_polls", m.CanSendPolls})
	f = append(f, jsonField{"can_send_other_messages", m.CanSendOtherMessages})
	f = append(f, jsonField{"can_add_web_page_previews", m.CanAddWebPagePreviews})
	f = append(f, jsonField{"can_react_to_messages", m.CanReactToMessages})
	f = append(f, jsonField{"can_edit_tag", m.CanEditTag})
	f = append(f, jsonField{"can_change_info", m.CanChangeInfo})
	f = append(f, jsonField{"can_invite_users", m.CanInviteUsers})
	f = append(f, jsonField{"can_pin_messages", m.CanPinMessages})
	f = append(f, jsonField{"can_manage_topics", m.CanManageTopics})
	return f
}
