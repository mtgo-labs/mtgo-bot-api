package client

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("sendphoto", (*Client).sendPhoto)
	Register("senddocument", (*Client).sendDocument)
	Register("sendaudio", (*Client).sendAudio)
	Register("sendvideo", (*Client).sendVideo)
	Register("sendanimation", (*Client).sendAnimation)
	Register("sendvoice", (*Client).sendVoice)
	Register("sendvideonote", (*Client).sendVideoNote)
	Register("sendsticker", (*Client).sendSticker)
}

// sendPhoto sends a photo. Reference: Client.cpp process_send_photo_query.
func (c *Client) sendPhoto(ctx context.Context, q *server.Query) (any, error) {
	return c.sendMediaPhoto(ctx, q, "photo")
}

// sendDocument sends a document. Reference: Client.cpp process_send_document_query.
func (c *Client) sendDocument(ctx context.Context, q *server.Query) (any, error) {
	return c.sendSingleDoc(ctx, q, "document", nil, "application/octet-stream")
}

// sendAudio sends an audio file. Reference: Client.cpp process_send_audio_query.
func (c *Client) sendAudio(ctx context.Context, q *server.Query) (any, error) {
	attrs := []tg.DocumentAttributeClass{
		buildAudioAttrs(q, false),
	}
	return c.sendSingleDoc(ctx, q, "audio", attrs, "audio/mpeg")
}

// sendVideo sends a video. Reference: Client.cpp process_send_video_query.
func (c *Client) sendVideo(ctx context.Context, q *server.Query) (any, error) {
	attrs := []tg.DocumentAttributeClass{
		buildVideoAttrs(q),
	}
	return c.sendSingleDoc(ctx, q, "video", attrs, "video/mp4")
}

// sendAnimation sends an animation (GIF). Reference: Client.cpp process_send_animation_query.
func (c *Client) sendAnimation(ctx context.Context, q *server.Query) (any, error) {
	attrs := []tg.DocumentAttributeClass{
		&tg.DocumentAttributeAnimated{},
		buildVideoAttrs(q),
	}
	return c.sendSingleDoc(ctx, q, "animation", attrs, "video/mp4")
}

// sendVoice sends a voice message. Reference: Client.cpp process_send_voice_query.
func (c *Client) sendVoice(ctx context.Context, q *server.Query) (any, error) {
	attrs := []tg.DocumentAttributeClass{
		buildAudioAttrs(q, true),
	}
	return c.sendSingleDoc(ctx, q, "voice", attrs, "audio/ogg")
}

// sendVideoNote sends a round video note. Reference: Client.cpp process_send_video_note_query.
func (c *Client) sendVideoNote(ctx context.Context, q *server.Query) (any, error) {
	attrs := []tg.DocumentAttributeClass{
		buildVideoNoteAttrs(q),
	}
	return c.sendSingleDoc(ctx, q, "video_note", attrs, "video/mp4")
}

// sendSticker sends a sticker. Reference: Client.cpp process_send_sticker_query.
func (c *Client) sendSticker(ctx context.Context, q *server.Query) (any, error) {
	// Stickers are documents; Telegram infers attributes from the file itself.
	// We pass no explicit attributes — the server-side document carries them.
	return c.sendSingleDoc(ctx, q, "sticker", nil, "application/octet-stream")
}

// sendMediaPhoto is the shared photo-sending path used by sendPhoto.
func (c *Client) sendMediaPhoto(ctx context.Context, q *server.Query, paramName string) (any, error) {
	if !hasMediaParam(q, paramName) {
		return nil, NewError(400, "Bad Request: there is no photo in the request")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	media, err := c.photoMediaInput(ctx, q, paramName)
	if err != nil {
		return nil, err
	}
	return c.doSendMedia(ctx, peer, media, q)
}

// sendSingleDoc is the shared document-sending path for all document-type
// media methods (document, audio, video, animation, voice, video_note, sticker).
func (c *Client) sendSingleDoc(
	ctx context.Context,
	q *server.Query,
	paramName string,
	attrs []tg.DocumentAttributeClass,
	mimeType string,
) (any, error) {
	if !hasMediaParam(q, paramName) {
		return nil, NewError(400, "Bad Request: "+mediaMissingMsg(paramName))
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	media, err := c.docMediaInput(ctx, q, paramName, attrs, mimeType)
	if err != nil {
		return nil, err
	}
	return c.doSendMedia(ctx, peer, media, q)
}

// hasMediaParam returns true when the given media parameter is present as
// either a string value (file_id/URL) or a multipart file upload.
func hasMediaParam(q *server.Query, paramName string) bool {
	if q.Arg(paramName) != "" {
		return true
	}
	if _, ok := q.File(paramName); ok {
		return true
	}
	return false
}

// mediaMissingMsg returns the official Bot API error message for a missing
// media parameter, matching the C++ process_send_*_query methods which
// validate content before chat_id.
func mediaMissingMsg(paramName string) string {
	switch paramName {
	case "photo":
		return "there is no photo in the request"
	case "document":
		return "there is no document in the request"
	case "audio":
		return "there is no audio in the request"
	case "video":
		return "there is no video in the request"
	case "animation":
		return "there is no animation in the request"
	case "voice":
		return "there is no voice in the request"
	case "video_note":
		return "there is no video note in the request"
	case "sticker":
		return "there is no sticker in the request"
	default:
		return "parameter \"" + paramName + "\" is required"
	}
}

// doSendMedia sends a media message via messages.sendMedia and returns the
// converted Bot API Message.
func (c *Client) doSendMedia(ctx context.Context, peer tg.InputPeerClass, media tg.InputMediaClass, q *server.Query) (any, error) {
	result, err := c.doSendMediaResult(ctx, peer, media, q)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// doSendMediaResult builds and invokes messages.sendMedia.
func (c *Client) doSendMediaResult(ctx context.Context, peer tg.InputPeerClass, media tg.InputMediaClass, q *server.Query) (any, error) {
	if err := c.checkWriteAccess(ctx, q); err != nil {
		return nil, err
	}
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
	}
	if err := applySendMediaOpts(req, q); err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if rm := q.Arg("reply_markup"); rm != "" {
		markup, err := convert.ReplyMarkup(rm)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		req.ReplyMarkup = markup
	}
	req.SetFlags()

	result, err := c.rpc.MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, c.sendRPCError(ctx, err, q)
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return true, nil
	}
	// A fresh document upload returns UpdateShortSentMessage whose media Document
	// has the file identity but no attributes and a default mime, so the
	// converter would emit a generic "document". Restore the typed mime +
	// attributes from the InputMedia we sent (mirrors TDLib reconstructing the
	// media locally). A file_id/URL resend's Document already carries attributes
	// and is left untouched.
	if uploaded, ok := media.(*tg.InputMediaUploadedDocument); ok {
		applyUploadedDocAttrs(msg, uploaded)
	}
	// Enrich partial messages from UpdateShortSentMessage.
	c.enrichPartialMessage(msg, peer, "")
	c.cachePeersFromMessage(ctx, msg)
	out := c.botMessage(ctx, msg, extractChats(result))
	// A fresh upload returns UpdateShortSentMessage whose media Document carries
	// no attributes, so file_name (DocumentAttributeFilename) is absent. The
	// reference (TDLib) restores it from the uploaded file's original name; do
	// the same, never overriding a filename already present (e.g. a file_id
	// resend whose stored document has the attribute).
	if name := uploadedFileName(q); name != "" {
		setMediaFileName(out, name)
	}
	return out, nil
}

// applyUploadedDocAttrs restores the typed media on a freshly-uploaded
// document's response. See doSendMediaResult for why this is needed.
//
// The Bot API type (audio/video/animation/…) is determined by the METHOD, not
// by what Telegram detected: a short video sent via sendVideo must stay "video"
// even though the response Document may carry DocumentAttributeAnimated. So the
// uploaded attributes win for typing. Values the caller omitted (duration,
// width/height) are filled from the response's detected attributes when present.
func applyUploadedDocAttrs(msg *tg.Message, uploaded *tg.InputMediaUploadedDocument) {
	if msg == nil || uploaded == nil || msg.Media == nil {
		return
	}
	mmd, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok || mmd == nil {
		return
	}
	doc, ok := mmd.Document.(*tg.Document)
	if !ok || doc == nil {
		return
	}
	if uploaded.MimeType != "" {
		doc.MimeType = uploaded.MimeType
	}
	doc.Attributes = mergeUploadedAttrs(uploaded.Attributes, doc.Attributes)
}

// mergeUploadedAttrs returns the uploaded attributes (which fix the Bot API
// type), enriching any zero value-carrying attribute (duration/width/height)
// from the detected attributes carried by the response Document.
func mergeUploadedAttrs(uploaded, detected []tg.DocumentAttributeClass) []tg.DocumentAttributeClass {
	var detVideo *tg.DocumentAttributeVideo
	var detAudio *tg.DocumentAttributeAudio
	for _, a := range detected {
		switch v := a.(type) {
		case *tg.DocumentAttributeVideo:
			detVideo = v
		case *tg.DocumentAttributeAudio:
			detAudio = v
		}
	}
	out := make([]tg.DocumentAttributeClass, 0, len(uploaded)+len(detected))
	seen := map[string]bool{}
	for _, a := range uploaded {
		switch v := a.(type) {
		case *tg.DocumentAttributeVideo:
			if detVideo != nil {
				if v.Duration == 0 {
					v.Duration = detVideo.Duration
				}
				if v.W == 0 {
					v.W = detVideo.W
				}
				if v.H == 0 {
					v.H = detVideo.H
				}
			}
		case *tg.DocumentAttributeAudio:
			if detAudio != nil && v.Duration == 0 {
				v.Duration = detAudio.Duration
			}
		}
		out = append(out, a)
		seen[fmt.Sprintf("%T", a)] = true
	}
	// Preserve detected non-type attributes the uploaded set lacks (e.g.
	// DocumentAttributeImageSize carrying the sticker's dimensions, detected
	// server-side). Type-determining attrs (Sticker/Animated/Audio/Video) stay
	// owned by the method and are not copied from the response.
	for _, a := range detected {
		switch a.(type) {
		case *tg.DocumentAttributeSticker, *tg.DocumentAttributeAnimated,
			*tg.DocumentAttributeVideo, *tg.DocumentAttributeAudio:
			continue
		}
		key := fmt.Sprintf("%T", a)
		if !seen[key] {
			out = append(out, a)
			seen[key] = true
		}
	}
	return out
}

// uploadedFileName returns the original filename of the uploaded file in q, if
// any (empty for file_id/URL resends, which carry no multipart file).
func uploadedFileName(q *server.Query) string {
	for _, f := range q.Files {
		if f.FileName != "" {
			return f.FileName
		}
	}
	return ""
}

// setMediaFileName sets the file_name on whichever media field the message
// carries, mirroring the reference which always emits the original filename.
func setMediaFileName(msg *apitypes.Message, name string) {
	if msg == nil {
		return
	}
	if msg.Document != nil && msg.Document.FileName == "" {
		msg.Document.FileName = name
	}
	if msg.Audio != nil && msg.Audio.FileName == "" {
		msg.Audio.FileName = name
	}
	if msg.Video != nil && msg.Video.FileName == "" {
		msg.Video.FileName = name
	}
	if msg.Animation != nil && msg.Animation.FileName == "" {
		msg.Animation.FileName = name
	}
}

// --- attribute builders ---

// buildAudioAttrs constructs DocumentAttributeAudio from Bot API params.
func buildAudioAttrs(q *server.Query, voice bool) *tg.DocumentAttributeAudio {
	a := &tg.DocumentAttributeAudio{
		Voice:    voice,
		Duration: parseInt32Default(q.Arg("duration"), 0),
	}
	if t := q.Arg("title"); t != "" {
		a.Title = t
	}
	if p := q.Arg("performer"); p != "" {
		a.Performer = p
	}
	if voice {
		a.Flags.Set(3) // voice flag
	}
	a.SetFlags()
	return a
}

// buildVideoAttrs constructs DocumentAttributeVideo from Bot API params.
func buildVideoAttrs(q *server.Query) *tg.DocumentAttributeVideo {
	v := &tg.DocumentAttributeVideo{
		Duration:          parseFloatDefault(q.Arg("duration"), 0),
		W:                 parseInt32Default(q.Arg("width"), 0),
		H:                 parseInt32Default(q.Arg("height"), 0),
		SupportsStreaming: q.ArgBool("supports_streaming"),
	}
	v.SetFlags()
	return v
}

// buildVideoNoteAttrs constructs a round-video attribute for video notes.
func buildVideoNoteAttrs(q *server.Query) *tg.DocumentAttributeVideo {
	length := parseInt32Default(q.Arg("length"), 0)
	v := &tg.DocumentAttributeVideo{
		Duration:     parseFloatDefault(q.Arg("duration"), 0),
		W:            length,
		H:            length,
		RoundMessage: true,
	}
	v.SetFlags()
	return v
}

// parseInt32Default parses a string to int32, returning def on failure.
func parseInt32Default(s string, def int32) int32 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return def
	}
	return int32(n)
}

// parseFloatDefault parses a string to float64, returning def on failure.
func parseFloatDefault(s string, def float64) float64 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return n
}

// parseReplyToID parses a reply_to_message_id string into an InputReplyToMessage.
func parseReplyToID(s string) (*tg.InputReplyToMessage, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return nil, err
	}
	return &tg.InputReplyToMessage{ReplyToMsgID: int32(id)}, nil
}
