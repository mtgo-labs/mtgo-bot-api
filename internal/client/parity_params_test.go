package client

import (
	"os"
	"strings"
	"testing"
)

// sendPollParams are the deep sendPoll params that must be parsed by the handler
// (guards the P0 cluster in docs/parameter-parity-gaps.md against regression).
var sendPollParams = []string{
	"type", "correct_option_id", "correct_option_ids", "explanation",
	"explanation_parse_mode", "explanation_media", "allows_revoting", "open_period", "close_date",
	"is_closed", "shuffle_options", "hide_results_until_closes", "members_only",
	"question_parse_mode", "country_codes",
}

// TestSendPollDescriptionIsCaption asserts the poll "description" is sent as the message
// caption (TDLib extract_input_caption treats inputMessagePoll.description as the caption →
// messages.sendMedia message field), wired in applySendMediaOpts.
func TestSendPollDescriptionIsCaption(t *testing.T) {
	b, err := os.ReadFile("media_helpers.go")
	if err != nil {
		t.Fatalf("read media_helpers.go: %v", err)
	}
	src := string(b)
	for _, want := range []string{`"description"`, `"description_parse_mode"`, `"description_entities"`} {
		if !strings.Contains(src, want) {
			t.Errorf("applySendMediaOpts does not handle the poll description caption (%s missing)", want)
		}
	}
}

// TestSendPollReadsDeepParams asserts the sendPoll handler reads every deep param.
func TestSendPollReadsDeepParams(t *testing.T) {
	bodies := handlerBodies(t)
	body, ok := bodies["sendPoll"]
	if !ok {
		t.Fatalf("sendPoll handler not found")
	}
	var missing []string
	for _, p := range sendPollParams {
		if !strings.Contains(body, "\""+p+"\"") {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		t.Errorf("sendPoll does not read params: %v", missing)
	} else {
		t.Logf("sendPoll reads all %d deep params", len(sendPollParams))
	}
}

// showCaptionFiles must reference show_caption_above_media (invert_media) after the
// P0 fix. applySendMediaOpts (media_helpers.go) covers the common send path.
var showCaptionFiles = []string{
	"media_helpers.go", // applySendMediaOpts → sendPhoto/Video/Animation/Document/Audio/Voice/sendInvoice
	"send_methods.go",  // sendPaidMedia
	"stories_misc.go",  // sendLivePhoto
	"messages_edit.go", // editMessageCaption
}

// TestShowCaptionAboveMediaRead asserts each file references show_caption_above_media.
func TestShowCaptionAboveMediaRead(t *testing.T) {
	for _, f := range showCaptionFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		if !strings.Contains(string(b), "show_caption_above_media") {
			t.Errorf("%s does not read show_caption_above_media", f)
		}
	}
}

// p1HandlerParams maps a *Client handler to the params it must read (P1 cluster).
var p1HandlerParams = map[string][]string{
	"restrictChatMember": {"use_independent_chat_permissions"},
	"setChatPermissions": {"use_independent_chat_permissions"},
	"banChatMember":      {"revoke_messages"},
	"unbanChatMember":    {"only_if_banned"},
	// direct_messages_topic_id is now read via the shared dmTopicID helper
	// (media_helpers.go); covered behaviorally by TestMessageThreadIDThreading.
	"doForwardOrCopy": {"remove_caption", "message_effect_id"},
	"sendMediaGroup":  {"message_effect_id"},
	"sendMessage":     {"message_effect_id"},
}

// TestP1ParamsRead asserts each P1 handler reads its params.
func TestP1ParamsRead(t *testing.T) {
	bodies := handlerBodies(t)
	var failed int
	for handler, params := range p1HandlerParams {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			failed++
			continue
		}
		for _, p := range params {
			if !strings.Contains(body, "\""+p+"\"") {
				t.Errorf("%s does not read %q", handler, p)
				failed++
			}
		}
	}
	if failed == 0 {
		t.Logf("all P1 handler params present")
	}
}

// messageEffectHelpers are files that must read message_effect_id via applySendMediaOpts
// (covers all single-media sends).
var messageEffectHelpers = []string{"media_helpers.go"}

// TestMessageEffectIDRead asserts applySendMediaOpts reads message_effect_id.
func TestMessageEffectIDRead(t *testing.T) {
	for _, f := range messageEffectHelpers {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(b), "message_effect_id") {
			t.Errorf("%s does not read message_effect_id", f)
		}
	}
}

// p2HandlerParams maps a *Client handler to the P2 param it must read.
var p2HandlerParams = map[string]string{
	"setMessageReaction":  "is_big",
	"answerInlineQuery":   "switch_pm_text",
	"createNewStickerSet": "contains_masks",
	"setUserEmojiStatus":  "emoji_status_expiration_date",
}

// TestP2ParamsRead asserts each P2 handler reads its param.
func TestP2ParamsRead(t *testing.T) {
	bodies := handlerBodies(t)
	var failed int
	for handler, param := range p2HandlerParams {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			failed++
			continue
		}
		if !strings.Contains(body, "\""+param+"\"") {
			t.Errorf("%s does not read %q", handler, param)
			failed++
		}
	}
	// supports_streaming lives in buildVideoAttrs (media_send.go), not a *Client method.
	if b, err := os.ReadFile("media_send.go"); err == nil && !strings.Contains(string(b), "supports_streaming") {
		t.Errorf("media_send.go does not read supports_streaming")
		failed++
	}
	if failed == 0 {
		t.Logf("all P2 params present")
	}
}

// p3HandlerParams maps a *Client story handler to the P3 params it must read.
var p3HandlerParams = map[string][]string{
	"postStory":   {"areas", "post_to_chat_page"},
	"editStory":   {"areas"},
	"repostStory": {"protect_content", "post_to_chat_page"},
}

// TestP3StoryParamsRead asserts the story handlers read their P3 params.
func TestP3StoryParamsRead(t *testing.T) {
	bodies := handlerBodies(t)
	for handler, params := range p3HandlerParams {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			continue
		}
		for _, p := range params {
			if !strings.Contains(body, "\""+p+"\"") {
				t.Errorf("%s does not read %q", handler, p)
			}
		}
	}
}

// TestBusinessProfilePhotoIsPublic asserts the business profile-photo handlers read is_public.
// Traced via TDLib: is_public → user_manager is_fallback → TL photos.*ProfilePhoto fallback.
func TestBusinessProfilePhotoIsPublic(t *testing.T) {
	bodies := handlerBodies(t)
	for _, h := range []string{"setBusinessAccountProfilePhoto", "removeBusinessAccountProfilePhoto"} {
		body, ok := bodies[h]
		if !ok {
			t.Errorf("handler %q not found", h)
			continue
		}
		if !strings.Contains(body, "\"is_public\"") {
			t.Errorf("%s does not read is_public", h)
		}
	}
}

// TestStoryPeerReconciliation asserts postStory/editStory use resolveStoryPeer
// (business_connection_id → connection user, else chat_id) rather than chat_id-only.
func TestStoryPeerReconciliation(t *testing.T) {
	bodies := handlerBodies(t)
	for _, h := range []string{"postStory", "editStory", "repostStory"} {
		body, ok := bodies[h]
		if !ok {
			t.Errorf("handler %q not found", h)
			continue
		}
		if !strings.Contains(body, "resolveStoryPeer") {
			t.Errorf("%s does not use resolveStoryPeer (peer reconciliation missing)", h)
		}
	}
}

// TestGetUserGiftsPeer asserts getUserGifts scopes gifts to the user_id peer.
func TestGetUserGiftsPeer(t *testing.T) {
	body, ok := handlerBodies(t)["getUserGifts"]
	if !ok {
		t.Fatal("getUserGifts not found")
	}
	if !strings.Contains(body, "user_id") || !strings.Contains(body, "ResolvePeer") {
		t.Errorf("getUserGifts does not wire user_id → peer")
	}
}

// TestSendMediaGroupDirectMessagesTopic asserts sendMediaGroup reads direct_messages_topic_id
// (TL messages.sendMultiMedia has no top_msg_id → addressed via reply_to's top_msg_id).
func TestSendMediaGroupDirectMessagesTopic(t *testing.T) {
	body, ok := handlerBodies(t)["sendMediaGroup"]
	if !ok {
		t.Fatal("sendMediaGroup not found")
	}
	if !strings.Contains(body, "direct_messages_topic_id") {
		t.Errorf("sendMediaGroup does not read direct_messages_topic_id")
	}
}

// TestCopyMessageCaptionEdit asserts copyMessage does forward-then-edit when caption /
// show_caption_above_media is given (forwardMessages can't carry caption/invert).
func TestCopyMessageCaptionEdit(t *testing.T) {
	body, ok := handlerBodies(t)["copyMessage"]
	if !ok {
		t.Fatal("copyMessage not found")
	}
	for _, want := range []string{`"caption"`, `"show_caption_above_media"`, "invokeEdit"} {
		if !strings.Contains(body, want) {
			t.Errorf("copyMessage forward-then-edit missing %q", want)
		}
	}
}

// TestAllowPaidBroadcast asserts sendMessage/sendMediaGroup apply allow_paid_broadcast
// (force-resolve a non-starter user's access hash, mirroring TDLib's force_create_dialog).
func TestAllowPaidBroadcast(t *testing.T) {
	bodies := handlerBodies(t)
	for _, h := range []string{"sendMessage", "sendMediaGroup"} {
		body, ok := bodies[h]
		if !ok {
			t.Errorf("handler %q not found", h)
			continue
		}
		if !strings.Contains(body, "resolveBroadcastPeer") {
			t.Errorf("%s does not apply allow_paid_broadcast (resolveBroadcastPeer missing)", h)
		}
	}
}

// TestLiveLocationReturnsMessage asserts editMessageLiveLocation/stopMessageLiveLocation
// return the edited Message (via invokeEdit) on the chat path, not just true.
func TestLiveLocationReturnsMessage(t *testing.T) {
	bodies := handlerBodies(t)
	for _, h := range []string{"editMessageLiveLocation", "stopMessageLiveLocation"} {
		body, ok := bodies[h]
		if !ok {
			t.Errorf("handler %q not found", h)
			continue
		}
		if !strings.Contains(body, "invokeEdit") {
			t.Errorf("%s does not return the edited Message (invokeEdit missing)", h)
		}
	}
}

// TestGiftPremiumTwoStep asserts giftPremiumSubscription completes the two-step
// (getPaymentForm → sendStarsForm) and honors star_count.
func TestGiftPremiumTwoStep(t *testing.T) {
	body, ok := handlerBodies(t)["giftPremiumSubscription"]
	if !ok {
		t.Fatal("giftPremiumSubscription not found")
	}
	for _, want := range []string{`"star_count"`, "PaymentsSendStarsForm", "PaymentsPaymentFormStars"} {
		if !strings.Contains(body, want) {
			t.Errorf("giftPremiumSubscription two-step missing %q", want)
		}
	}
}

// tdlibTracedParams were found by tracing Client.cpp→td_api→TDLib→MTProto (not "blocked"):
// each maps to a real MTProto field. Guards against regression + documents the trace.
var tdlibTracedParams = map[string]string{
	"doForwardOrCopy": "video_start_timestamp", // → messages.forwardMessages video_timestamp (flag 20)
}

// TestTDLibTracedParams asserts the handlers read their traced params.
func TestTDLibTracedParams(t *testing.T) {
	bodies := handlerBodies(t)
	for handler, param := range tdlibTracedParams {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			continue
		}
		if !strings.Contains(body, "\""+param+"\"") {
			t.Errorf("%s does not read %q", handler, param)
		}
	}
	// disable_content_type_detection → force_file lives in docMediaInput (media_helpers.go).
	if b, err := os.ReadFile("media_helpers.go"); err == nil && !strings.Contains(string(b), "disable_content_type_detection") {
		t.Errorf("media_helpers.go does not read disable_content_type_detection")
	}
}

// verifiedParams asserts param names fixed during the needs-verification pass
// (these were wrong-named bugs, not just dropped): the handler must read the
// OFFICIAL name, not the legacy/incorrect one.
var verifiedParams = map[string]struct{ want, notWant string }{
	"editUserStarSubscription": {"telegram_payment_charge_id", "subscription_id"},
}

// TestVerifiedParamNames asserts each handler reads the official param name.
func TestVerifiedParamNames(t *testing.T) {
	bodies := handlerBodies(t)
	for handler, vn := range verifiedParams {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			continue
		}
		if !strings.Contains(body, "\""+vn.want+"\"") {
			t.Errorf("%s does not read %q", handler, vn.want)
		}
		if strings.Contains(body, "\""+vn.notWant+"\"") {
			t.Errorf("%s still reads old/incorrect %q", handler, vn.notWant)
		}
	}
}

// TestGetUserGiftsFilters asserts getUserGifts reads the exclude_* filters.
func TestGetUserGiftsFilters(t *testing.T) {
	body, ok := handlerBodies(t)["getUserGifts"]
	if !ok {
		t.Fatal("getUserGifts not found")
	}
	for _, p := range []string{
		"exclude_unlimited", "exclude_unique", "exclude_limited_upgradable",
		"exclude_limited_non_upgradable", "exclude_from_blockchain", "sort_by_price", "offset", "limit",
	} {
		if !strings.Contains(body, "\""+p+"\"") {
			t.Errorf("getUserGifts does not read %q", p)
		}
	}
}

// botAPI101Params maps handlers to params found dropped by the 2026-06-23 re-audit vs
// Bot API 10.1 (see docs/parameter-parity-gaps.md "Newly detected vs Bot API 10.1").
var botAPI101Params = map[string][]string{
	"sendPoll":                  {"allow_adding_options"},
	"savePreparedInlineMessage": {"allow_user_chats", "allow_bot_chats", "allow_group_chats", "allow_channel_chats"},
	"sendVenue":                 {"foursquare_id", "foursquare_type", "google_place_id", "google_place_type"},
	"answerInlineQuery":         {"button"},
	"editMessageText":           {"rich_message"},
	"sendMessageDraft":          {"draft_id"},
	"sendLivePhoto":             {"has_spoiler"},
}

// TestBotAPI101ParamsRead asserts each handler reads its Bot API 10.1 param(s).
func TestBotAPI101ParamsRead(t *testing.T) {
	bodies := handlerBodies(t)
	for handler, params := range botAPI101Params {
		body, ok := bodies[handler]
		if !ok {
			t.Errorf("handler %q not found", handler)
			continue
		}
		for _, p := range params {
			if !strings.Contains(body, "\""+p+"\"") {
				t.Errorf("%s does not read %q", handler, p)
			}
		}
	}
}

// TestSendMessageDraftIsTypingAction asserts sendMessageDraft emits a typing action
// (messages.setTyping + sendMessageTextDraftAction), NOT messages.saveDraft — it is a
// "user is typing a draft" broadcast. Mirrors Client.cpp process_send_message_draft_query.
func TestSendMessageDraftIsTypingAction(t *testing.T) {
	body, ok := handlerBodies(t)["sendMessageDraft"]
	if !ok {
		t.Fatal("sendMessageDraft not found")
	}
	for _, want := range []string{"MessagesSetTyping", "SendMessageTextDraftAction"} {
		if !strings.Contains(body, want) {
			t.Errorf("sendMessageDraft missing %q", want)
		}
	}
	if strings.Contains(body, "MessagesSaveDraft") {
		t.Errorf("sendMessageDraft must not use MessagesSaveDraft (typing broadcast, not a draft save)")
	}
}

// TestAdminRightsVoiceChatAlias asserts extractAdminRights reads the deprecated
// can_manage_voice_chats alias (Client.cpp:15920 reads voice||video → can_manage_calls).
func TestAdminRightsVoiceChatAlias(t *testing.T) {
	b, err := os.ReadFile("chat_admin.go")
	if err != nil {
		t.Fatalf("read chat_admin.go: %v", err)
	}
	if !strings.Contains(string(b), "can_manage_voice_chats") {
		t.Errorf("extractAdminRights does not read the can_manage_voice_chats alias")
	}
}

// TestDeleteMessageReactionSender asserts deleteMessageReaction (deletemessagereaction)
// deletes a specific sender's reactions per-message via messages.deleteParticipantReaction
// (reading user_id/actor_chat_id), not messages.sendReaction(nil).
func TestDeleteMessageReactionSender(t *testing.T) {
	body, ok := handlerBodies(t)["deleteMessageReaction"]
	if !ok {
		t.Fatal("deleteMessageReaction handler not found")
	}
	for _, want := range []string{`"user_id"`, `"actor_chat_id"`, "MessagesDeleteParticipantReaction"} {
		if !strings.Contains(body, want) {
			t.Errorf("deleteMessageReaction missing %q", want)
		}
	}
	if strings.Contains(body, "MessagesSendReaction") {
		t.Errorf("deleteMessageReaction must not use MessagesSendReaction (per-sender delete, not self-reaction removal)")
	}
}
