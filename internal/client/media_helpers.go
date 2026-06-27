package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// mediaSource enumerates how a media parameter was supplied.
type mediaSource int

const (
	srcFileID mediaSource = iota
	srcURL
	srcUpload
)

// docMediaInput resolves a document-type Bot API media parameter (file_id,
// URL, or multipart upload) into a tg.InputMediaClass. The attrs slice is
// applied only to uploaded files (file_id-based documents already carry
// their attributes on Telegram's side). mimeType is the fallback MIME type
// for uploads when the multipart part lacks one.
func (c *Client) docMediaInput(
	ctx context.Context,
	q *server.Query,
	paramName string,
	attrs []tg.DocumentAttributeClass,
	mimeType string,
) (tg.InputMediaClass, error) {
	val := q.Arg(paramName)
	if val == "" {
		if f, ok := q.File(paramName); ok {
			// sendSticker uploads must carry DocumentAttributeSticker so the
			// result classifies as a sticker (mirrors TDLib inputMessageSticker).
			if paramName == "sticker" {
				attrs = append([]tg.DocumentAttributeClass{
					&tg.DocumentAttributeSticker{Alt: q.Arg("emoji"), Stickerset: &tg.InputStickerSetEmpty{}},
				}, attrs...)
			}
			media, err := c.uploadedDocMedia(ctx, q, f, attrs, mimeType)
			if err != nil {
				return nil, err
			}
			// disable_content_type_detection → force_file (TL inputMediaUploadedDocument
			// flag 4): send as a plain file regardless of detected MIME/attributes.
			// Traced via TDLib: Client.cpp disable_content_type_detection → inputMediaDocument
			// force_file (messages.sendMedia). Only meaningful for uploads.
			if u, ok := media.(*tg.InputMediaUploadedDocument); ok && q.ArgBool("disable_content_type_detection") {
				u.ForceFile = true
			}
			return media, nil
		}
		return nil, NewError(400, "Bad Request: parameter \""+paramName+"\" is required")
	}

	src, val := classifyMedia(val)
	switch src {
	case srcURL:
		return &tg.InputMediaDocumentExternal{URL: val}, nil
	case srcFileID:
		inputDoc, err := fileid.ToInputDocument(val)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid file_id: "+err.Error())
		}
		return &tg.InputMediaDocument{ID: inputDoc}, nil
	}
	return nil, NewError(400, "Bad Request: unsupported media source")
}

// photoMediaInput resolves a photo-type media parameter into InputMediaClass.
func (c *Client) photoMediaInput(
	ctx context.Context,
	q *server.Query,
	paramName string,
) (tg.InputMediaClass, error) {
	val := q.Arg(paramName)
	if val == "" {
		if f, ok := q.File(paramName); ok {
			return c.uploadedPhotoMedia(ctx, q, f)
		}
		return nil, NewError(400, "Bad Request: parameter \""+paramName+"\" is required")
	}

	src, val := classifyMedia(val)
	switch src {
	case srcURL:
		return &tg.InputMediaPhotoExternal{URL: val}, nil
	case srcFileID:
		inputPhoto, err := fileid.ToInputPhoto(val)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid file_id: "+err.Error())
		}
		return &tg.InputMediaPhoto{ID: inputPhoto}, nil
	}
	return nil, NewError(400, "Bad Request: unsupported media source")
}

// classifyMedia determines whether a media string is a URL, an attach://
// reference (handled by the caller via the multipart file map), or a file_id.
// Returns the source type and the cleaned value.
func classifyMedia(val string) (mediaSource, string) {
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		return srcURL, val
	}
	return srcFileID, val
}

// uploadedDocMedia handles multipart file upload for document-type media
// (document, audio, video, voice, animation, video_note, sticker).
func (c *Client) uploadedDocMedia(
	ctx context.Context,
	q *server.Query,
	f server.File,
	attrs []tg.DocumentAttributeClass,
	mimeType string,
) (tg.InputMediaClass, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	file, err := os.Open(f.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read uploaded file")
	}
	defer func() { _ = file.Close() }()

	// Sniff the MIME type from content. The multipart part's declared type is
	// unreliable (clients often send application/octet-stream); the reference
	// detects the type from the file content. Fall back to the part/method type.
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, NewError(500, "Internal Server Error: failed to read uploaded file")
	}

	id, err := generateFileID()
	if err != nil {
		return nil, NewError(500, "Internal Server Error: failed to generate file_id")
	}

	inputFile, err := c.uploadFile(ctx, id, f.FileName, f.Size, file)
	if err != nil {
		return nil, rpcError(err)
	}

	mt := detectMime(head)
	if mt == "" {
		mt = coalesceMimeType(f.MimeType, mimeType)
	}
	media := &tg.InputMediaUploadedDocument{
		File:       inputFile,
		MimeType:   mt,
		Attributes: attrs,
	}
	if q.ArgBool("has_spoiler") {
		media.Spoiler = true
	}
	if thumb, ok := c.uploadThumbnail(ctx, q); ok {
		media.Thumb = thumb
	}
	media.SetFlags()
	return media, nil
}

// uploadThumbnail reads the explicit "thumbnail" (legacy "thumb") multipart
// field, uploads it, and returns the InputFile; (nil, false) if absent or the
// upload fails. Mirrors Client.cpp get_input_thumbnail (10638). The explicit
// thumbnail param is unaffected by the server's ignore_inline_thumbnails option
// (which only suppresses client-derived inline thumbnails).
func (c *Client) uploadThumbnail(ctx context.Context, q *server.Query) (tg.InputFileClass, bool) {
	f, ok := q.File("thumbnail")
	if !ok {
		f, ok = q.File("thumb")
		if !ok {
			return nil, false
		}
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, false
	}
	tf, err := os.Open(f.TempPath)
	if err != nil {
		return nil, false
	}
	defer func() { _ = tf.Close() }()
	id, err := generateFileID()
	if err != nil {
		return nil, false
	}
	input, err := c.uploadFile(ctx, id, f.FileName, f.Size, tf)
	if err != nil {
		return nil, false
	}
	return input, true
}

// uploadedPhotoMedia handles multipart file upload for photos.
func (c *Client) uploadedPhotoMedia(
	ctx context.Context,
	q *server.Query,
	f server.File,
) (tg.InputMediaClass, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	file, err := os.Open(f.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read uploaded file")
	}
	defer func() { _ = file.Close() }()

	id, err := generateFileID()
	if err != nil {
		return nil, NewError(500, "Internal Server Error: failed to generate file_id")
	}

	inputFile, err := c.uploadFile(ctx, id, f.FileName, f.Size, file)
	if err != nil {
		return nil, rpcError(err)
	}

	media := &tg.InputMediaUploadedPhoto{File: inputFile}
	if q.ArgBool("has_spoiler") {
		media.Spoiler = true
		media.SetFlags()
	}
	media.SetFlags()
	return media, nil
}

// coalesceMimeType returns the first non-empty MIME type.
func coalesceMimeType(parts ...string) string {
	for _, p := range parts {
		if p != "" {
			return p
		}
	}
	return "application/octet-stream"
}

// detectMime returns the MIME type of common media formats from their magic
// bytes, or "" when unknown. It mirrors TDLib's file-type detection closely
// enough for the formats bots commonly upload (audio, video, voice, images);
// unknown types fall back to the caller's declared/method mime.
func detectMime(head []byte) string {
	switch {
	case len(head) >= 8 && bytes.Equal(head[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "image/jpeg"
	case len(head) >= 6 && (bytes.Equal(head[:6], []byte("GIF87a")) || bytes.Equal(head[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return "image/webp"
	case len(head) >= 4 && bytes.Equal(head[:4], []byte("OggS")):
		return "audio/ogg"
	case len(head) >= 3 && bytes.Equal(head[:3], []byte("ID3")):
		return "audio/mpeg" // MP3 with ID3 tag
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		return "audio/mpeg" // MP3 frame sync
	case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")):
		return "video/mp4" // MP4/M4V/MOV container
	case len(head) >= 4 && bytes.Equal(head[:4], []byte("\x1aE\xdf\xa3")):
		return "video/webm" // WebM/Matroska EBML
	case len(head) >= 4 && bytes.Equal(head[:4], []byte("%PDF")):
		return "application/pdf"
	}
	return ""
}

// applySendMediaOpts sets the common optional fields (caption, reply_to,
// silent, noforwards) on a MessagesSendMediaRequest from the query params.
// applySendMediaOpts sets the common optional fields (caption, reply_to, silent,
// noforwards, show_caption_above_media, message_effect_id) on a MessagesSendMediaRequest
// from the query params. Returns an error on a malformed parse_mode (matches the
// reference's get_formatted_text validation).
func applySendMediaOpts(req *tg.MessagesSendMediaRequest, q *server.Query) error {
	// Caption text + entities. Most media sends use caption/parse_mode/caption_entities.
	// sendPoll uses description/description_parse_mode/description_entities — TDLib's
	// extract_input_caption treats inputMessagePoll.description as the poll's message caption.
	text := q.Arg("caption")
	parseMode := q.Arg("parse_mode")
	entitiesJSON := q.Arg("caption_entities")
	if text == "" {
		text = q.Arg("description")
		parseMode = q.Arg("description_parse_mode")
		entitiesJSON = q.Arg("description_entities")
	}
	if text != "" {
		parsed, ents, err := convert.FormattedText(text, parseMode, entitiesJSON)
		if err != nil {
			return err
		}
		req.Message = parsed
		if len(ents) > 0 {
			req.Entities = ents
		}
	}
	if rt := buildReplyTo(q); rt != nil {
		req.ReplyTo = rt
		req.Flags.Set(0)
	}
	if q.ArgBool("disable_notification") {
		req.Silent = true
		req.Flags.Set(5)
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
		req.Flags.Set(14)
	}
	if q.ArgBool("show_caption_above_media") {
		req.InvertMedia = true
		req.Flags.Set(16)
	}
	if eid, err := q.ArgInt64("message_effect_id"); err == nil && eid != 0 {
		req.Effect = eid
	}
	return nil
}

// topicID returns the message_thread_id (forum topic) as int32, or 0 when the
// parameter is absent or invalid. Unlike parseTopicID (used by the
// forum-management methods that require a topic and 400 otherwise), an absent
// topic here simply means "no topic" — valid for non-forum sends.
func topicID(q *server.Query) int32 {
	v := q.Arg("message_thread_id")
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n <= 0 {
		return 0
	}
	return int32(n)
}

// dmTopicID returns the direct_messages_topic_id as int32, or 0 when absent/invalid.
func dmTopicID(q *server.Query) int32 {
	n, err := q.ArgInt64("direct_messages_topic_id")
	if err != nil || n <= 0 {
		return 0
	}
	return int32(n)
}

// buildReplyTo constructs the InputReplyToMessage for a send, combining a reply
// target (reply_to_message_id, or the bare-integer reply_parameters the MVP
// accepts) with a topic (message_thread_id for forums, or direct_messages_topic_id
// for DM topics). Returns nil when none are set.
//
// messages.sendMessage/sendMedia/sendMultiMedia carry no top-level topic field, so
// the topic is expressed via InputReplyToMessage.TopMsgID (flag 0); when only a
// topic is given (no reply), ReplyToMsgID is set to the topic id because
// inputReplyToMessage.reply_to_msg_id is required. reply_to_message_id takes
// precedence over reply_parameters (preserving prior behavior). messages.forwardMessages
// and messages.setTyping use a top-level TopMsgID instead and do not call this helper.
func buildReplyTo(q *server.Query) *tg.InputReplyToMessage {
	topic := topicID(q)
	if topic == 0 {
		topic = dmTopicID(q)
	}

	var replyMsg int32
	if rp := q.Arg("reply_parameters"); rp != "" {
		if v, err := strconv.ParseInt(rp, 10, 64); err == nil && v > 0 {
			replyMsg = int32(v)
		}
	}
	if rtm := q.Arg("reply_to_message_id"); rtm != "" {
		if rt, err := parseReplyToID(rtm); err == nil && rt != nil {
			replyMsg = rt.ReplyToMsgID
		}
	}

	if replyMsg == 0 && topic == 0 {
		return nil
	}
	rt := &tg.InputReplyToMessage{}
	if replyMsg > 0 {
		rt.ReplyToMsgID = replyMsg
	} else {
		rt.ReplyToMsgID = topic
	}
	if topic > 0 {
		rt.TopMsgID = topic
	}
	rt.SetFlags()
	return rt
}

// applyLinkPreview applies link_preview_options / disable_web_page_preview to the
// no_webpage (flag 1) and invert_media (flag 16) booleans of messages.sendMessage/
// editMessage. link_preview_options wins over the legacy disable_web_page_preview bool.
// The url / prefer_small_media / prefer_large_media fields require switching the send
// to messages.sendMedia with inputMediaWebPage (deferred — not handled here).
func applyLinkPreview(q *server.Query, noWebpage, invert *bool) {
	disabled := q.ArgBool("disable_web_page_preview")
	showAbove := false
	if lp := q.Arg("link_preview_options"); lp != "" {
		var opts struct {
			IsDisabled    bool `json:"is_disabled"`
			ShowAboveText bool `json:"show_above_text"`
		}
		if json.Unmarshal([]byte(lp), &opts) == nil {
			disabled = opts.IsDisabled
			showAbove = opts.ShowAboveText
		}
	}
	if disabled {
		*noWebpage = true
	}
	if showAbove {
		*invert = true
	}
}

// buildInputMediaWebPage returns inputMediaWebPage when link_preview_options
// specifies a url (and is not disabled), else nil. prefer_large_media /
// prefer_small_media map to the constructor's force flags. Mirrors TDLib
// InputMessageText::get_input_media_web_page — when non-nil, the text message is
// sent via messages.sendMedia (with the preview attached as media) instead of
// messages.sendMessage, which has no media field.
func buildInputMediaWebPage(q *server.Query, message string) *tg.InputMediaWebPage {
	lp := q.Arg("link_preview_options")
	if lp == "" {
		return nil
	}
	var opts struct {
		IsDisabled       bool   `json:"is_disabled"`
		URL              string `json:"url"`
		PreferSmallMedia bool   `json:"prefer_small_media"`
		PreferLargeMedia bool   `json:"prefer_large_media"`
	}
	if json.Unmarshal([]byte(lp), &opts) != nil {
		return nil
	}
	if opts.IsDisabled || opts.URL == "" {
		return nil
	}
	// optional (flag 2) = the message carries text: Telegram attaches the preview
	// if resolvable but won't fail with WEBPAGE_NOT_FOUND for a non-previewable URL.
	// Mirrors TDLib InputMessageText::get_input_media_web_page (is_optional = !text.empty()).
	wp := &tg.InputMediaWebPage{
		URL:             opts.URL,
		ForceLargeMedia: opts.PreferLargeMedia,
		ForceSmallMedia: opts.PreferSmallMedia,
		Optional:        message != "",
	}
	wp.SetFlags()
	return wp
}

// inputMediaDescriptor is the Bot API JSON shape for a single media item in
// editMessageMedia, sendMediaGroup, and similar methods.
type inputMediaDescriptor struct {
	Type    string `json:"type"`
	Media   string `json:"media"`
	Caption string `json:"caption,omitempty"`
}

// parseMediaDescriptor parses a single Bot API media JSON object into an
// InputMediaClass. Supports file_id, URL, and attach:// references (multipart
// uploads) for both photo and document-type media.
func (c *Client) parseMediaDescriptor(ctx context.Context, q *server.Query, raw string) (tg.InputMediaClass, string, error) {
	var desc inputMediaDescriptor
	if err := json.Unmarshal([]byte(raw), &desc); err != nil {
		return nil, "", fmt.Errorf("invalid media JSON: %w", err)
	}
	if desc.Media == "" {
		return nil, "", errors.New("media field is required")
	}

	caption := desc.Caption

	// attach://<name> — a multipart file uploaded alongside the request.
	const attachPrefix = "attach://"
	if strings.HasPrefix(desc.Media, attachPrefix) {
		name := desc.Media[len(attachPrefix):]
		f, ok := q.File(name)
		if !ok {
			return nil, "", fmt.Errorf("attached file %q not found", name)
		}
		if desc.Type == "photo" {
			m, err := c.uploadedPhotoMedia(ctx, q, f)
			return m, caption, err
		}
		m, err := c.uploadedDocMedia(ctx, q, f, nil, "application/octet-stream")
		return m, caption, err
	}

	switch desc.Type {
	case "photo":
		src, val := classifyMedia(desc.Media)
		switch src {
		case srcURL:
			return &tg.InputMediaPhotoExternal{URL: val}, caption, nil
		case srcFileID:
			p, err := fileid.ToInputPhoto(val)
			if err != nil {
				return nil, "", fmt.Errorf("invalid photo file_id: %w", err)
			}
			return &tg.InputMediaPhoto{ID: p}, caption, nil
		}
	case "document", "video", "animation", "audio", "voice", "video_note", "sticker":
		src, val := classifyMedia(desc.Media)
		switch src {
		case srcURL:
			return &tg.InputMediaDocumentExternal{URL: val}, caption, nil
		case srcFileID:
			d, err := fileid.ToInputDocument(val)
			if err != nil {
				return nil, "", fmt.Errorf("invalid document file_id: %w", err)
			}
			return &tg.InputMediaDocument{ID: d}, caption, nil
		}
	}
	return nil, "", fmt.Errorf("unsupported media type %q", desc.Type)
}

// paidMediaDescriptor is the Bot API InputPaidMedia JSON shape used by sendInvoice
// (paid_media) and sendPaidMedia (media[]). Only type+media are resolved to
// MTProto InputMedia here; the video sub-fields (thumbnail/cover/duration/
// start_timestamp/supports_streaming) are deferred — cover/start_timestamp are
// paid-media-cover concepts with no plain-InputMedia representation, and
// thumbnail/duration belong on uploaded documents (attached via uploadThumbnail).
type paidMediaDescriptor struct {
	Type  string `json:"type"`
	Media string `json:"media"`
}

// resolvePaidMediaItem resolves a single Bot API InputPaidMedia descriptor into
// the MTProto InputMedia used for invoice extended_media and sendPaidMedia:
// attach:// → uploaded InputMediaUploaded*; URL → InputMedia*External; file_id →
// InputMedia*. Photo (and live_photo) and video are supported.
func (c *Client) resolvePaidMediaItem(ctx context.Context, q *server.Query, raw string) (tg.InputMediaClass, error) {
	var d paidMediaDescriptor
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return nil, fmt.Errorf("invalid paid_media JSON: %w", err)
	}
	if d.Media == "" {
		return nil, errors.New("media field is required")
	}

	const attachPrefix = "attach://"
	switch d.Type {
	case "photo", "live_photo":
		if strings.HasPrefix(d.Media, attachPrefix) {
			f, ok := q.File(d.Media[len(attachPrefix):])
			if !ok {
				return nil, fmt.Errorf("attached file %q not found", d.Media[len(attachPrefix):])
			}
			return c.uploadedPhotoMedia(ctx, q, f)
		}
		src, val := classifyMedia(d.Media)
		if src == srcURL {
			return &tg.InputMediaPhotoExternal{URL: val}, nil
		}
		p, err := fileid.ToInputPhoto(val)
		if err != nil {
			return nil, fmt.Errorf("invalid photo file_id: %w", err)
		}
		return &tg.InputMediaPhoto{ID: p}, nil
	case "video":
		if strings.HasPrefix(d.Media, attachPrefix) {
			f, ok := q.File(d.Media[len(attachPrefix):])
			if !ok {
				return nil, fmt.Errorf("attached file %q not found", d.Media[len(attachPrefix):])
			}
			return c.uploadedDocMedia(ctx, q, f, nil, "video/mp4")
		}
		src, val := classifyMedia(d.Media)
		if src == srcURL {
			return &tg.InputMediaDocumentExternal{URL: val}, nil
		}
		doc, err := fileid.ToInputDocument(val)
		if err != nil {
			return nil, fmt.Errorf("invalid document file_id: %w", err)
		}
		return &tg.InputMediaDocument{ID: doc}, nil
	}
	return nil, fmt.Errorf("unsupported paid media type %q", d.Type)
}

// resolvePaidMediaArray parses a Bot API InputPaidMedia JSON array into the
// MTProto InputMedia list for inputMediaPaidMedia.extended_media (sendPaidMedia).
// Each item is resolved via resolvePaidMediaItem (attach:// upload, URL, file_id).
func (c *Client) resolvePaidMediaArray(ctx context.Context, q *server.Query, raw string) ([]tg.InputMediaClass, error) {
	if raw == "" {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("invalid media array: %w", err)
	}
	out := make([]tg.InputMediaClass, 0, len(items))
	for _, item := range items {
		m, err := c.resolvePaidMediaItem(ctx, q, string(item))
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
