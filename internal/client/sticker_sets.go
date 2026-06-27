package client

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("getstickerset", (*Client).getStickerSet)
	Register("getforumtopiciconstickers", (*Client).getForumTopicIconStickers)
	Register("uploadstickerfile", (*Client).uploadStickerFile)
	Register("createnewstickerset", (*Client).createNewStickerSet)
	Register("addstickertoset", (*Client).addStickerToSet)
	Register("replacestickerinset", (*Client).replaceStickerInSet)
	Register("setstickersetthumbnail", (*Client).setStickerSetThumbnail)
	Register("setstickersettitle", (*Client).setStickerSetTitle)
	Register("deletestickerset", (*Client).deleteStickerSet)
	Register("setcustomemojistickersetthumbnail", (*Client).setCustomEmojiStickerSetThumbnail)
	Register("setstickerpositioninset", (*Client).setStickerPositionInSet)
	Register("deletestickerfromset", (*Client).deleteStickerFromSet)
	Register("setstickeremojilist", (*Client).setStickerEmojiList)
	Register("setstickerkeywords", (*Client).setStickerKeywords)
	Register("setstickermaskposition", (*Client).setStickerMaskPosition)
}

// stickerSetInput resolves a Bot API sticker set name into an InputStickerSetShortName.
func stickerSetInput(name string) (*tg.InputStickerSetShortName, error) {
	if name == "" {
		return nil, NewError(400, "Bad Request: sticker set name must be non-empty")
	}
	return &tg.InputStickerSetShortName{ShortName: name}, nil
}

// fileIDToInputDocument decodes a Bot API file_id into a tg.InputDocument
// (id, access_hash, file_reference) for use in stickers.* RPC calls.
func fileIDToInputDocument(fileID string) (*tg.InputDocument, error) {
	if fileID == "" {
		return nil, NewError(400, "Bad Request: sticker is not specified")
	}
	d, err := fileid.Decode(fileID)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid file_id: "+err.Error())
	}
	return &tg.InputDocument{
		ID:            d.ID,
		AccessHash:    d.AccessHash,
		FileReference: d.FileReference,
	}, nil
}

// connectStickers ensures the bot is connected for stickers.* calls.
func connectStickers(ctx context.Context, c *Client) error {
	if err := c.ensureConnected(ctx); err != nil {
		return &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}
	return nil
}

// setStickerSetTitle renames a sticker set.
// Reference: Client.cpp process_set_sticker_set_title_query.
func (c *Client) setStickerSetTitle(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	ss, err := stickerSetInput(q.Arg("name"))
	if err != nil {
		return nil, err
	}
	title := q.Arg("title")
	if title == "" {
		return nil, NewError(400, "Bad Request: parameter \"title\" is required")
	}
	_, err = c.rpc.StickersRenameStickerSet(ctx, &tg.StickersRenameStickerSetRequest{
		Stickerset: ss,
		Title:      title,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteStickerSet deletes a sticker set.
// Reference: Client.cpp process_delete_sticker_set_query.
func (c *Client) deleteStickerSet(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	ss, err := stickerSetInput(q.Arg("name"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.StickersDeleteStickerSet(ctx, &tg.StickersDeleteStickerSetRequest{
		Stickerset: ss,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setCustomEmojiStickerSetThumbnail sets a custom emoji as the thumbnail of a
// custom emoji sticker set.
// Reference: Client.cpp process_set_custom_emoji_sticker_set_thumbnail_query.
func (c *Client) setCustomEmojiStickerSetThumbnail(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	ss, err := stickerSetInput(q.Arg("name"))
	if err != nil {
		return nil, err
	}
	var thumbDocID int64
	if v := q.Arg("custom_emoji_id"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			thumbDocID = n
		}
	}
	_, err = c.rpc.StickersSetStickerSetThumb(ctx, &tg.StickersSetStickerSetThumbRequest{
		Stickerset:      ss,
		ThumbDocumentID: thumbDocID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setStickerPositionInSet changes the position of a sticker in its set.
// Reference: Client.cpp process_set_sticker_position_in_set_query.
// Params: sticker (file_id), position.
func (c *Client) setStickerPositionInSet(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(q.Arg("sticker"))
	if err != nil {
		return nil, err
	}
	pos, err := q.ArgInt64("position")
	if err != nil {
		return nil, NewError(400, "Bad Request: parameter \"position\" is required")
	}
	_, err = c.rpc.StickersChangeStickerPosition(ctx, &tg.StickersChangeStickerPositionRequest{
		Sticker:  doc,
		Position: int32(pos),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteStickerFromSet removes a sticker from its set.
// Reference: Client.cpp process_delete_sticker_from_set_query.
// Params: sticker (file_id).
func (c *Client) deleteStickerFromSet(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(q.Arg("sticker"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.StickersRemoveStickerFromSet(ctx, &tg.StickersRemoveStickerFromSetRequest{
		Sticker: doc,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setStickerEmojiList changes the emoji list of a sticker.
// Reference: Client.cpp process_set_sticker_emoji_list_query.
// Params: sticker (file_id), emoji_list (JSON array).
func (c *Client) setStickerEmojiList(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(q.Arg("sticker"))
	if err != nil {
		return nil, err
	}
	emojis := parseStringArray(q.Arg("emoji_list"))
	if len(emojis) == 0 {
		return nil, NewError(400, "Bad Request: parameter \"emoji_list\" is required")
	}
	_, err = c.rpc.StickersChangeSticker(ctx, &tg.StickersChangeStickerRequest{
		Sticker: doc,
		Emoji:   strings.Join(emojis, ","),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setStickerKeywords changes the keywords of a sticker.
// Reference: Client.cpp process_set_sticker_keywords_query.
// Params: sticker (file_id), keywords (JSON array).
func (c *Client) setStickerKeywords(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(q.Arg("sticker"))
	if err != nil {
		return nil, err
	}
	keywords := parseStringArray(q.Arg("keywords"))
	// Empty keywords clears the list (valid operation).
	_, err = c.rpc.StickersChangeSticker(ctx, &tg.StickersChangeStickerRequest{
		Sticker:  doc,
		Keywords: strings.Join(keywords, "\n"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setStickerMaskPosition changes the mask position of a mask sticker.
// Reference: Client.cpp process_set_sticker_mask_position_query.
// Params: sticker (file_id), mask_position (JSON object).
func (c *Client) setStickerMaskPosition(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(q.Arg("sticker"))
	if err != nil {
		return nil, err
	}
	mc, err := parseMaskPosition(q.Arg("mask_position"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.StickersChangeSticker(ctx, &tg.StickersChangeStickerRequest{
		Sticker:    doc,
		MaskCoords: mc,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// parseStringArray parses a JSON array of strings.
func parseStringArray(raw string) []string {
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

// parseMaskPosition parses a Bot API MaskPosition JSON into tg.MaskCoords.
// Point mapping: forehead=0, eyes=1, mouth=2, chin=3.
func parseMaskPosition(raw string) (*tg.MaskCoords, error) {
	if raw == "" {
		return nil, nil // clearing the mask position
	}
	var mp struct {
		Point  string  `json:"point"`
		XShift float64 `json:"x_shift"`
		YShift float64 `json:"y_shift"`
		Scale  float64 `json:"scale"`
	}
	if err := json.Unmarshal([]byte(raw), &mp); err != nil {
		return nil, NewError(400, "Bad Request: invalid mask_position JSON: "+err.Error())
	}
	var n int32
	switch mp.Point {
	case "forehead":
		n = 0
	case "eyes":
		n = 1
	case "mouth":
		n = 2
	case "chin":
		n = 3
	default:
		return nil, NewError(400, "Bad Request: unknown mask_position point: "+mp.Point)
	}
	return &tg.MaskCoords{
		N:    n,
		X:    mp.XShift,
		Y:    mp.YShift,
		Zoom: mp.Scale,
	}, nil
}

// getStickerSet returns a sticker set by its short name.
// Reference: Client.cpp process_get_sticker_set_query.
// Params: name (sticker set short name).
func (c *Client) getStickerSet(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	// No name pre-validation: the reference passes it straight to MTProto, so an
	// empty or unknown name surfaces as the raw "STICKERSET_INVALID" error
	// (verified vs api.telegram.org). The "sticker set not found" default belongs
	// to a different sticker method (Client.cpp:7241), not getStickerSet.
	name := q.Arg("name")
	res, err := c.rpc.MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
		Stickerset: &tg.InputStickerSetShortName{ShortName: name},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	mss, ok := res.(*tg.MessagesStickerSet)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected sticker set response type")
	}
	return convert.StickerSetFromMessages(mss), nil
}

// getForumTopicIconStickers returns the default set of forum topic icons.
// Reference: Client.cpp process_get_forum_topic_icon_stickers_query.
// TDLib resolves this via SpecialStickerSetType::default_topic_icons() →
// inputStickerSetEmojiDefaultTopicIcons → messages.getStickerSet.
func (c *Client) getForumTopicIconStickers(ctx context.Context, q *server.Query) (any, error) {
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	res, err := c.rpc.MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
		Stickerset: &tg.InputStickerSetEmojiDefaultTopicIcons{},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	mss, ok := res.(*tg.MessagesStickerSet)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected sticker set response type")
	}
	setName := ""
	if ss, ok := mss.Set.(*tg.StickerSet); ok {
		setName = ss.ShortName
	}
	stickers := convert.StickersFromDocuments(mss.Documents, setName)
	return stickers, nil
}

// --- File-upload sticker methods ---

// stickerFormatMime maps Bot API sticker_format to MIME type.
func stickerFormatMime(format string) string {
	switch format {
	case "animated":
		return "application/x-tgsticker"
	case "video":
		return "video/webm"
	default: // "static"
		return "image/webp"
	}
}

// uploadToInputDocument resolves a Bot API sticker parameter (file_id or
// multipart upload) into a *tg.InputDocument. For multipart uploads, the file
// is uploaded via messages.uploadMedia and the resulting Document is returned.
func (c *Client) uploadToInputDocument(ctx context.Context, q *server.Query, paramName, mimeType string) (*tg.InputDocument, error) {
	// file_id path — already uploaded.
	if val := q.Arg(paramName); val != "" {
		return fileIDToInputDocument(val)
	}
	// Multipart upload path.
	f, ok := q.File(paramName)
	if !ok {
		return nil, NewError(400, "Bad Request: parameter \""+paramName+"\" is required")
	}
	file, err := os.Open(f.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read uploaded file")
	}
	defer func() { _ = file.Close() }()

	id, err := generateFileID()
	if err != nil {
		return nil, NewError(500, "Internal Server Error: file ID generation failed")
	}
	inputFile, err := c.uploadFile(ctx, id, f.FileName, f.Size, file)
	if err != nil {
		return nil, rpcError(err)
	}

	media := &tg.InputMediaUploadedDocument{
		File:     inputFile,
		MimeType: coalesceMimeType(f.MimeType, mimeType),
	}
	media.SetFlags()

	res, err := c.rpc.MessagesUploadMedia(ctx, &tg.MessagesUploadMediaRequest{
		Peer:  &tg.InputPeerSelf{},
		Media: media,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	mmd, ok := res.(*tg.MessageMediaDocument)
	if !ok || mmd.Document == nil {
		return nil, NewError(500, "Internal Server Error: unexpected upload response")
	}
	doc, ok := mmd.Document.(*tg.Document)
	if !ok {
		return nil, NewError(500, "Internal Server Error: no document in upload response")
	}
	return &tg.InputDocument{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}, nil
}

// uploadStickerFile uploads a sticker file and returns a Bot API File.
// Reference: Client.cpp process_upload_sticker_file_query.
// Params: user_id, sticker (multipart or file_id), sticker_format.
func (c *Client) uploadStickerFile(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	doc, err := c.uploadToInputDocument(ctx, q, "sticker", stickerFormatMime(q.Arg("sticker_format")))
	if err != nil {
		return nil, err
	}
	// For uploadStickerFile, we return the raw document info as a File.
	// The file_id encodes the document for later use in createNewStickerSet.
	return &apitypes.File{
		FileID:       fileid.EncodeDocument(0, doc.ID, doc.AccessHash, doc.FileReference, fileid.TypeSticker),
		FileUniqueID: fileid.EncodeDocumentUnique(doc.ID, fileid.TypeSticker),
	}, nil
}

// inputStickerDescriptor is the Bot API JSON shape for a single InputSticker
// in the stickers array of createNewStickerSet.
type inputStickerDescriptor struct {
	Sticker      string                 `json:"sticker"`
	EmojiList    []string               `json:"emoji_list"`
	Format       string                 `json:"format"`
	MaskPosition *apitypes.MaskPosition `json:"mask_position,omitempty"`
	Keywords     []string               `json:"keywords,omitempty"`
}

// createNewStickerSet creates a new sticker set.
// Reference: Client.cpp process_create_new_sticker_set_query.
// Params: user_id, name, title, stickers (JSON array), sticker_type, needs_repainting.
func (c *Client) createNewStickerSet(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	name := q.Arg("name")
	title := q.Arg("title")
	if name == "" || title == "" {
		return nil, NewError(400, "Bad Request: name and title are required")
	}

	// Parse the stickers JSON array.
	stickersRaw := q.Arg("stickers")
	if stickersRaw == "" {
		return nil, NewError(400, "Bad Request: parameter \"stickers\" is required")
	}
	var descs []inputStickerDescriptor
	if err := json.Unmarshal([]byte(stickersRaw), &descs); err != nil {
		return nil, NewError(400, "Bad Request: invalid stickers JSON: "+err.Error())
	}
	if len(descs) == 0 {
		return nil, NewError(400, "Bad Request: stickers must not be empty")
	}

	// Build InputStickerSetItem for each sticker.
	items := make([]*tg.InputStickerSetItem, 0, len(descs))
	for _, d := range descs {
		doc, err := c.uploadToInputDocument(ctx, q, "sticker", stickerFormatMime(d.Format))
		if err != nil {
			return nil, err
		}
		item := &tg.InputStickerSetItem{
			Document: doc,
			Emoji:    strings.Join(d.EmojiList, ","),
		}
		if len(d.Keywords) > 0 {
			item.Keywords = strings.Join(d.Keywords, "\n")
		}
		if d.MaskPosition != nil {
			item.MaskCoords, _ = parseMaskPositionJSON(d.MaskPosition)
		}
		item.SetFlags()
		items = append(items, item)
	}

	// user_id identifies the set owner (stickers.createStickerSet.user_id is a
	// required wire field). Default to self (the bot's own set); accept an
	// explicit user_id to act on behalf of another user.
	var inputUser tg.InputUserClass = &tg.InputUserSelf{}
	if uid > 0 {
		inputUser = &tg.InputUser{UserID: uid}
	}
	req := &tg.StickersCreateStickerSetRequest{
		UserID:    inputUser,
		Title:     title,
		ShortName: name,
		Stickers:  items,
		Software:  "mtgo-bot-api",
	}
	switch q.Arg("sticker_type") {
	case "mask":
		req.Masks = true
	case "custom_emoji":
		req.Emojis = true
	}
	// Legacy boolean alias for sticker_type=mask.
	if q.ArgBool("contains_masks") {
		req.Masks = true
	}
	if q.ArgBool("needs_repainting") {
		req.TextColor = true
	}
	req.SetFlags()

	_, err = c.rpc.StickersCreateStickerSet(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// addStickerToSet adds a sticker to an existing set.
// Reference: Client.cpp process_add_sticker_to_set_query.
// Params: user_id, name, sticker (JSON InputSticker or multipart/file_id).
func (c *Client) addStickerToSet(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	ss, err := stickerSetInput(q.Arg("name"))
	if err != nil {
		return nil, err
	}
	doc, err := c.uploadToInputDocument(ctx, q, "sticker", stickerFormatMime(q.Arg("format")))
	if err != nil {
		return nil, err
	}
	item := &tg.InputStickerSetItem{
		Document: doc,
		Emoji:    strings.Join(parseStringArray(q.Arg("emoji_list")), ","),
	}
	if kw := q.Arg("keywords"); kw != "" {
		item.Keywords = strings.Join(parseStringArray(kw), "\n")
	}
	item.SetFlags()

	_, err = c.rpc.StickersAddStickerToSet(ctx, &tg.StickersAddStickerToSetRequest{
		Stickerset: ss,
		Sticker:    item,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// replaceStickerInSet replaces a sticker in a set.
// Reference: Client.cpp process_replace_sticker_in_set_query.
// Params: user_id, name, old_sticker (file_id), sticker (JSON InputSticker or multipart/file_id).
func (c *Client) replaceStickerInSet(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	// name is not sent on the wire (stickers.replaceSticker identifies the set via
	// the old_sticker document's access_hash), but TDLib validates it is non-empty
	// first (StickersManager.cpp add_sticker_to_set: "Sticker set name must be non-empty").
	if q.Arg("name") == "" {
		return nil, NewError(400, "Bad Request: Sticker set name must be non-empty")
	}
	oldDoc, err := fileIDToInputDocument(q.Arg("old_sticker"))
	if err != nil {
		return nil, err
	}
	newDoc, err := c.uploadToInputDocument(ctx, q, "sticker", stickerFormatMime(q.Arg("format")))
	if err != nil {
		return nil, err
	}
	item := &tg.InputStickerSetItem{
		Document: newDoc,
		Emoji:    strings.Join(parseStringArray(q.Arg("emoji_list")), ","),
	}
	item.SetFlags()

	_, err = c.rpc.StickersReplaceSticker(ctx, &tg.StickersReplaceStickerRequest{
		Sticker:    oldDoc,
		NewSticker: item,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setStickerSetThumbnail sets or removes the thumbnail of a sticker set.
// Reference: Client.cpp process_set_sticker_set_thumbnail_query.
// Params: user_id, name, thumbnail (multipart or file_id), format.
func (c *Client) setStickerSetThumbnail(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := connectStickers(ctx, c); err != nil {
		return nil, err
	}
	ss, err := stickerSetInput(q.Arg("name"))
	if err != nil {
		return nil, err
	}
	// If thumbnail is empty → remove the thumbnail (no thumb, no thumb_document_id).
	if q.Arg("thumbnail") == "" && q.Arg("thumb") == "" {
		if _, ok := q.File("thumbnail"); !ok {
			if _, ok2 := q.File("thumb"); !ok2 {
				// Remove thumbnail — send request with no flags set.
				_, err = c.rpc.StickersSetStickerSetThumb(ctx, &tg.StickersSetStickerSetThumbRequest{
					Stickerset: ss,
				})
				if err != nil {
					return nil, rpcError(err)
				}
				return true, nil
			}
		}
	}
	// Upload or decode the thumbnail.
	paramName := "thumbnail"
	doc, err := c.uploadToInputDocument(ctx, q, paramName, stickerFormatMime(q.Arg("format")))
	if err != nil {
		// Try legacy "thumb" parameter.
		doc, err = c.uploadToInputDocument(ctx, q, "thumb", stickerFormatMime(q.Arg("format")))
		if err != nil {
			return nil, err
		}
	}
	_, err = c.rpc.StickersSetStickerSetThumb(ctx, &tg.StickersSetStickerSetThumbRequest{
		Stickerset: ss,
		Thumb:      doc,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// parseMaskPositionJSON converts an already-parsed MaskPosition struct into tg.MaskCoords.
func parseMaskPositionJSON(mp *apitypes.MaskPosition) (*tg.MaskCoords, error) {
	if mp == nil {
		return nil, nil
	}
	var n int32
	switch mp.Point {
	case "forehead":
		n = 0
	case "eyes":
		n = 1
	case "mouth":
		n = 2
	case "chin":
		n = 3
	default:
		return nil, nil
	}
	return &tg.MaskCoords{N: n, X: mp.XShift, Y: mp.YShift, Zoom: mp.Scale}, nil
}
