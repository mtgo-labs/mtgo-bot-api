package client

import (
	"context"
	"sync"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// recorder is a tg.Invoker that captures every RPC request object. It returns
// (nil, nil) for every call — handlers that post-process the result either
// tolerate nil (extract* helpers are nil-safe via type-assertion ok-guards) or
// return an error, but the request is always captured first. This lets behavioral
// tests assert on the built MTProto request without a live connection.
type recorder struct {
	mu       sync.Mutex
	calls    []tg.TLObject
	response tg.TLObject // canned response returned from RPCInvoke (nil → nil,nil)
	err      error       // canned error returned from RPCInvoke
}

func (r *recorder) RPCInvoke(_ context.Context, input tg.TLObject, _ func(*tg.Reader) (tg.TLObject, error)) (tg.TLObject, error) {
	r.mu.Lock()
	r.calls = append(r.calls, input)
	resp, err := r.response, r.err
	r.mu.Unlock()
	return resp, err
}

func (r *recorder) RPCInvokeRaw(_ context.Context, _ tg.TLObject) ([]byte, error) {
	return nil, nil
}

func (r *recorder) last() tg.TLObject {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return nil
	}
	return r.calls[len(r.calls)-1]
}

// wantReq returns the last captured RPC request, failing if none was captured.
func wantReq(t *testing.T, r *recorder) tg.TLObject {
	t.Helper()
	last := r.last()
	if last == nil {
		t.Fatalf("no RPC call captured")
	}
	return last
}

func wantSendMessage(t *testing.T, r *recorder) *tg.MessagesSendMessageRequest {
	t.Helper()
	req, ok := wantReq(t, r).(*tg.MessagesSendMessageRequest)
	if !ok {
		t.Fatalf("captured %T, want *MessagesSendMessageRequest", r.last())
	}
	return req
}

func wantSendMedia(t *testing.T, r *recorder) *tg.MessagesSendMediaRequest {
	t.Helper()
	req, ok := wantReq(t, r).(*tg.MessagesSendMediaRequest)
	if !ok {
		t.Fatalf("captured %T, want *MessagesSendMediaRequest", r.last())
	}
	return req
}

func wantForwardMessages(t *testing.T, r *recorder) *tg.MessagesForwardMessagesRequest {
	t.Helper()
	req, ok := wantReq(t, r).(*tg.MessagesForwardMessagesRequest)
	if !ok {
		t.Fatalf("captured %T, want *MessagesForwardMessagesRequest", r.last())
	}
	return req
}

func wantSetTyping(t *testing.T, r *recorder) *tg.MessagesSetTypingRequest {
	t.Helper()
	req, ok := wantReq(t, r).(*tg.MessagesSetTypingRequest)
	if !ok {
		t.Fatalf("captured %T, want *MessagesSetTypingRequest", r.last())
	}
	return req
}

func wantEditMessage(t *testing.T, r *recorder) *tg.MessagesEditMessageRequest {
	t.Helper()
	req, ok := wantReq(t, r).(*tg.MessagesEditMessageRequest)
	if !ok {
		t.Fatalf("captured %T, want *MessagesEditMessageRequest", r.last())
	}
	return req
}

func newTestClient(r *recorder) *Client {
	return &Client{
		botID: "123456789",
		ready: true,
		rpc:   tg.NewRPCClient(r),
		msgs:  newMsgCache(defaultMsgCacheCap),
	}
}

func newQ(method string, args map[string]string) *server.Query {
	q := server.NewQuery()
	q.Method = method
	q.Args = args
	return q
}

// --- pure-function tests for the new helpers (the parity logic itself) ---

func TestBuildReplyToTopic(t *testing.T) {
	// message_thread_id only → topic post (ReplyToMsgID = topic, TopMsgID = topic).
	rt := buildReplyTo(newQ("sendmessage", map[string]string{"message_thread_id": "42"}))
	if rt == nil {
		t.Fatal("nil reply-to for topic-only post")
	}
	if rt.TopMsgID != 42 || rt.ReplyToMsgID != 42 {
		t.Errorf("topic-only: TopMsgID=%d ReplyToMsgID=%d, want 42/42", rt.TopMsgID, rt.ReplyToMsgID)
	}
	if !rt.Flags.Has(0) {
		t.Error("topic-only: flag 0 (top_msg_id) not set")
	}
}

func TestBuildReplyToReplyPlusTopic(t *testing.T) {
	rt := buildReplyTo(newQ("sendmessage", map[string]string{"reply_to_message_id": "7", "message_thread_id": "42"}))
	if rt == nil {
		t.Fatal("nil reply-to")
	}
	if rt.ReplyToMsgID != 7 || rt.TopMsgID != 42 {
		t.Errorf("reply+topic: ReplyToMsgID=%d TopMsgID=%d, want 7/42", rt.ReplyToMsgID, rt.TopMsgID)
	}
}

func TestBuildReplyToReplyOnly(t *testing.T) {
	rt := buildReplyTo(newQ("sendmessage", map[string]string{"reply_to_message_id": "7"}))
	if rt == nil {
		t.Fatal("nil reply-to")
	}
	if rt.ReplyToMsgID != 7 || rt.TopMsgID != 0 {
		t.Errorf("reply-only: ReplyToMsgID=%d TopMsgID=%d, want 7/0", rt.ReplyToMsgID, rt.TopMsgID)
	}
}

func TestBuildReplyToNone(t *testing.T) {
	if rt := buildReplyTo(newQ("sendmessage", nil)); rt != nil {
		t.Errorf("expected nil reply-to, got %+v", rt)
	}
}

func TestBuildReplyToDMTopicFallback(t *testing.T) {
	rt := buildReplyTo(newQ("sendmessage", map[string]string{"direct_messages_topic_id": "9"}))
	if rt == nil || rt.TopMsgID != 9 {
		t.Errorf("dm topic fallback: %+v", rt)
	}
}

func TestApplyLinkPreview(t *testing.T) {
	// Legacy disable_web_page_preview → no_webpage.
	var nw, inv bool
	applyLinkPreview(newQ("sendmessage", map[string]string{"disable_web_page_preview": "true"}), &nw, &inv)
	if !nw || inv {
		t.Errorf("legacy: nw=%v inv=%v, want true/false", nw, inv)
	}
	// link_preview_options wins + show_above_text → invert_media.
	nw, inv = false, false
	applyLinkPreview(newQ("sendmessage", map[string]string{
		"disable_web_page_preview": "true", // should be overridden
		"link_preview_options":     `{"is_disabled":false,"show_above_text":true}`,
	}), &nw, &inv)
	if nw || !inv {
		t.Errorf("lpo override: nw=%v inv=%v, want false/true", nw, inv)
	}
}

func TestBuildInvoiceDeepAndFlags(t *testing.T) {
	inv := buildInvoice(newQ("sendinvoice", map[string]string{
		"currency":              "USD",
		"prices":                `[{"label":"x","amount":100}]`,
		"max_tip_amount":        "500",
		"suggested_tip_amounts": `[100,200,300]`,
		"subscription_period":   "2592000",
		"need_name":             "true",
		"is_flexible":           "true",
	}))
	if len(inv.Prices) != 1 || inv.Prices[0].Amount != 100 {
		t.Errorf("prices: %+v", inv.Prices)
	}
	if inv.MaxTipAmount != 500 {
		t.Errorf("max_tip=%d", inv.MaxTipAmount)
	}
	if len(inv.SuggestedTipAmounts) != 3 || inv.SuggestedTipAmounts[2] != 300 {
		t.Errorf("suggested_tips=%v", inv.SuggestedTipAmounts)
	}
	if inv.SubscriptionPeriod != 2592000 {
		t.Errorf("subscription_period=%d", inv.SubscriptionPeriod)
	}
	// Flag correctness (regression guard for the off-by-one bug that set the Test
	// bit for need_name): need_name → flag 1, NOT flag 0 (Test).
	if !inv.NameRequested || !inv.Flags.Has(1) {
		t.Error("need_name did not set NameRequested/flag 1")
	}
	if inv.Flags.Has(0) {
		t.Error("need_name wrongly set the Test flag (0)")
	}
	if !inv.Flexible || !inv.Flags.Has(5) {
		t.Error("is_flexible did not set Flexible/flag 5")
	}
	// tips share flag 8.
	if !inv.Flags.Has(8) {
		t.Error("tip amounts did not set flag 8")
	}
	if !inv.Flags.Has(11) {
		t.Error("subscription_period did not set flag 11")
	}
}

func TestBuildInvoiceMedia(t *testing.T) {
	c := newTestClient(&recorder{})
	media := c.buildInvoiceMedia(context.Background(), newQ("sendinvoice", map[string]string{
		"title":          "T",
		"description":    "D",
		"payload":        "p",
		"currency":       "USD",
		"prices":         `[{"label":"x","amount":1}]`,
		"provider_token": "PROV",
		"provider_data":  `{"k":"v"}`,
		"photo_url":      "https://x/y.jpg",
		"photo_width":    "100",
		"photo_height":   "200",
	}))
	if media.Provider != "PROV" || !media.Flags.Has(3) {
		t.Errorf("provider: %q flag3=%v", media.Provider, media.Flags.Has(3))
	}
	if media.ProviderData == nil || media.ProviderData.Data != `{"k":"v"}` {
		t.Errorf("provider_data: %+v", media.ProviderData)
	}
	if media.Photo == nil || media.Photo.URL != "https://x/y.jpg" {
		t.Errorf("photo: %+v", media.Photo)
	}
	if !media.Flags.Has(0) {
		t.Error("photo did not set flag 0")
	}
	// photo dimensions → DocumentAttributeImageSize attribute (W/H, not Width/Height).
	var img *tg.DocumentAttributeImageSize
	for _, a := range media.Photo.Attributes {
		if d, ok := a.(*tg.DocumentAttributeImageSize); ok {
			img = d
		}
	}
	if img == nil || img.W != 100 || img.H != 200 {
		t.Errorf("photo dims attr: %+v", img)
	}
}

// --- dispatch tests through the recording invoker (handler wiring) ---

func TestMessageThreadIDSendMessage(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "-1001234567890", "text": "hi", "message_thread_id": "42",
	}))
	req := wantSendMessage(t, r)
	rt, ok := req.ReplyTo.(*tg.InputReplyToMessage)
	if !ok {
		t.Fatalf("ReplyTo type %T, want *InputReplyToMessage", req.ReplyTo)
	}
	if rt.TopMsgID != 42 || rt.ReplyToMsgID != 42 {
		t.Errorf("topic post: TopMsgID=%d ReplyToMsgID=%d, want 42/42", rt.TopMsgID, rt.ReplyToMsgID)
	}
}

func TestMessageThreadIDForwardMessages(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _, _ = c.doForwardOrCopy(context.Background(), newQ("forwardmessages", map[string]string{
		"chat_id": "-1001234567890", "from_chat_id": "-1009876543210",
		"message_ids": "[1,2]", "message_thread_id": "55",
	}), false)
	req := wantForwardMessages(t, r)
	if req.TopMsgID != 55 {
		t.Errorf("forwardMessages TopMsgID=%d, want 55", req.TopMsgID)
	}
}

func TestMessageThreadIDChatAction(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.sendChatAction(context.Background(), newQ("sendchataction", map[string]string{
		"chat_id": "-1001234567890", "action": "typing", "message_thread_id": "8",
	}))
	req := wantSetTyping(t, r)
	if req.TopMsgID != 8 {
		t.Errorf("setTyping TopMsgID=%d, want 8", req.TopMsgID)
	}
}

func TestSendLocationLive(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.sendLocation(context.Background(), newQ("sendlocation", map[string]string{
		"chat_id": "-1001234567890", "latitude": "1.5", "longitude": "2.5",
		"live_period": "60", "heading": "90", "proximity_alert_radius": "50",
	}))
	req := wantSendMedia(t, r)
	live, ok := req.Media.(*tg.InputMediaGeoLive)
	if !ok {
		t.Fatalf("media type %T, want *InputMediaGeoLive", req.Media)
	}
	if live.Period != 60 || live.Heading != 90 || live.ProximityNotificationRadius != 50 {
		t.Errorf("live fields: period=%d heading=%d prox=%d", live.Period, live.Heading, live.ProximityNotificationRadius)
	}
}

func TestSendLocationStatic(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.sendLocation(context.Background(), newQ("sendlocation", map[string]string{
		"chat_id": "-1001234567890", "latitude": "1.5", "longitude": "2.5",
	}))
	req := wantSendMedia(t, r)
	if _, ok := req.Media.(*tg.InputMediaGeoPoint); !ok {
		t.Fatalf("static media type %T, want *InputMediaGeoPoint", req.Media)
	}
}

func TestEditMessageTextFormatting(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.editMessageText(context.Background(), newQ("editmessagetext", map[string]string{
		"chat_id": "-1001234567890", "message_id": "5", "text": "*bold*", "parse_mode": "Markdown",
	}))
	req := wantEditMessage(t, r)
	if req.Message != "bold" {
		t.Errorf("Markdown not parsed: Message=%q (want \"bold\")", req.Message)
	}
	if len(req.Entities) == 0 {
		t.Error("parse_mode=Markdown produced no entities")
	}
}

func TestEditMessageTextLinkPreview(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	_, _ = c.editMessageText(context.Background(), newQ("editmessagetext", map[string]string{
		"chat_id": "-1001234567890", "message_id": "5", "text": "hi",
		"link_preview_options": `{"is_disabled":true,"show_above_text":true}`,
	}))
	req := wantEditMessage(t, r)
	if !req.NoWebpage {
		t.Error("link_preview_options.is_disabled did not set NoWebpage")
	}
	if !req.InvertMedia {
		t.Error("show_above_text did not set InvertMedia")
	}
}

func TestSendMessageLinkPreviewURL(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	// link_preview_options.url → switch to messages.sendMedia with inputMediaWebPage.
	_, _ = c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "-1001234567890", "text": "see this",
		"link_preview_options": `{"url":"https://example.com","prefer_large_media":true,"show_above_text":true}`,
	}))
	req := wantSendMedia(t, r)
	wp, ok := req.Media.(*tg.InputMediaWebPage)
	if !ok {
		t.Fatalf("media type %T, want *InputMediaWebPage", req.Media)
	}
	if wp.URL != "https://example.com" || !wp.ForceLargeMedia {
		t.Errorf("webpage: url=%q force_large=%v", wp.URL, wp.ForceLargeMedia)
	}
	if req.Message != "see this" {
		t.Errorf("text not carried on sendMedia: Message=%q", req.Message)
	}
	if !req.InvertMedia {
		t.Error("show_above_text did not set InvertMedia on the sendMedia request")
	}
}

func TestSendMessageTextNoWebPageURL(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	// No url → stays on messages.sendMessage.
	_, _ = c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "-1001234567890", "text": "hi", "disable_web_page_preview": "true",
	}))
	req := wantSendMessage(t, r)
	if !req.NoWebpage {
		t.Error("disable_web_page_preview did not set NoWebpage")
	}
}

func TestSendInvoiceExtendedMedia(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)
	// Invoice paid_media/extended_media → inputMediaInvoice.extended_media (flag 2),
	// a plain InputMedia (no star amount on the invoice path).
	_, _ = c.sendInvoice(context.Background(), newQ("sendinvoice", map[string]string{
		"chat_id": "-1001234567890", "title": "T", "description": "D", "payload": "p",
		"currency": "USD", "prices": `[{"label":"x","amount":100}]`,
		"paid_media": `{"type":"photo","media":"https://example.com/p.jpg"}`,
	}))
	req := wantSendMedia(t, r)
	inv, ok := req.Media.(*tg.InputMediaInvoice)
	if !ok {
		t.Fatalf("media %T, want *InputMediaInvoice", req.Media)
	}
	if !inv.Flags.Has(2) {
		t.Error("paid_media did not set extended_media flag 2")
	}
	photo, ok := inv.ExtendedMedia.(*tg.InputMediaPhotoExternal)
	if !ok {
		t.Fatalf("extended_media %T, want *InputMediaPhotoExternal", inv.ExtendedMedia)
	}
	if photo.URL != "https://example.com/p.jpg" {
		t.Errorf("extended media url=%q", photo.URL)
	}
}
