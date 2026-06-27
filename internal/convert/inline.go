package convert

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// InlineQueryResults converts a JSON-encoded array of Bot API InlineQueryResult
// objects into tg.InputBotInlineResultClass slices for the TL RPC call.
func InlineQueryResults(jsonData string) ([]tg.InputBotInlineResultClass, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(jsonData), &raw); err != nil {
		return nil, fmt.Errorf("invalid results JSON: %w", err)
	}
	results := make([]tg.InputBotInlineResultClass, 0, len(raw))
	for i, r := range raw {
		var base struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(r, &base); err != nil {
			return nil, fmt.Errorf("result %d: invalid JSON: %w", i, err)
		}
		result, err := convertInlineResult(base.Type, base.ID, r)
		if err != nil {
			return nil, fmt.Errorf("result %d (%s): %w", i, base.Type, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func convertInlineResult(resultType, id string, raw json.RawMessage) (tg.InputBotInlineResultClass, error) {
	switch resultType {
	case "article":
		return convertInlineResultArticle(id, raw)
	case "photo":
		return convertInlineResultPhoto(id, raw)
	case "gif", "mpeg4_gif", "video", "audio", "voice", "document", "sticker":
		return convertInlineResultDocument(id, raw, resultType)
	case "location", "venue":
		return convertInlineResultLocation(id, raw, resultType)
	case "contact":
		return convertInlineResultContact(id, raw)
	case "game":
		return convertInlineResultGame(id, raw)
	default:
		return nil, fmt.Errorf("unsupported result type: %s", resultType)
	}
}

// ensureInlineMessage returns msg if non-nil; otherwise it falls back to
// inputBotInlineMessageMediaAuto (using caption as the message text). Media
// inline results (photo/gif/video/...) may omit input_message_content; the
// official Bot API defaults to MediaAuto in that case. send_message is a
// required TL field, so a nil value would panic at encode time (ECONNRESET).
func ensureInlineMessage(msg tg.InputBotInlineMessageClass, caption string) tg.InputBotInlineMessageClass {
	if msg != nil {
		return msg
	}
	m := &tg.InputBotInlineMessageMediaAuto{Message: caption}
	m.SetFlags()
	return m
}

// defaultInlineMime returns the conventional MIME type for an inline document
// result type when the client omits mime_type.
func defaultInlineMime(docType string) string {
	switch docType {
	case "gif":
		return "image/gif"
	case "mpeg4_gif", "video":
		return "video/mp4"
	case "audio":
		return "audio/mpeg"
	case "voice":
		return "audio/ogg"
	default: // "document", "sticker", ...
		return "application/octet-stream"
	}
}

// jsonFieldString extracts a string field from a JSON object by name. Returns
// "" if the field is absent or not a string. Used to read type-specific URL
// fields (gif_url, mpeg4_url, ...) without declaring every variant in a struct.
func jsonFieldString(raw json.RawMessage, field string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(m[field], &s)
	return s
}

func convertInlineResultArticle(id string, raw json.RawMessage) (tg.InputBotInlineResultClass, error) {
	var r struct {
		Title               string          `json:"title"`
		URL                 string          `json:"url"`
		Description         string          `json:"description"`
		ThumbURL            string          `json:"thumb_url"`
		InputMessageContent json.RawMessage `json:"input_message_content"`
		ReplyMarkup         json.RawMessage `json:"reply_markup"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg, err := convertInputMessageContent(r.InputMessageContent)
	if err != nil {
		return nil, err
	}
	result := &tg.InputBotInlineResult{
		ID:          id,
		Type:        "article",
		Title:       r.Title,
		SendMessage: ensureInlineMessage(msg, ""),
	}
	if r.URL != "" {
		result.URL = r.URL
		result.Flags.Set(1)
	}
	if r.Description != "" {
		result.Description = r.Description
		result.Flags.Set(2)
	}
	if r.ThumbURL != "" {
		result.Thumb = &tg.InputWebDocument{URL: r.ThumbURL, Size: 0, MimeType: "image/jpeg"}
		result.Flags.Set(3)
	}
	result.SetFlags()
	return result, nil
}

func convertInlineResultPhoto(id string, raw json.RawMessage) (tg.InputBotInlineResultClass, error) {
	var r struct {
		PhotoURL            string          `json:"photo_url"`
		PhotoFileID         string          `json:"photo_file_id"`
		ThumbURL            string          `json:"thumb_url"`
		ThumbnailURL        string          `json:"thumbnail_url"`
		PhotoWidth          int             `json:"photo_width"`
		PhotoHeight         int             `json:"photo_height"`
		Title               string          `json:"title"`
		Description         string          `json:"description"`
		Caption             string          `json:"caption"`
		InputMessageContent json.RawMessage `json:"input_message_content"`
		ReplyMarkup         json.RawMessage `json:"reply_markup"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg, err := convertInputMessageContent(r.InputMessageContent)
	if err != nil {
		return nil, err
	}
	thumb := r.ThumbURL
	if thumb == "" {
		thumb = r.ThumbnailURL
	}
	result := &tg.InputBotInlineResult{
		ID:          id,
		Type:        "photo",
		Title:       r.Title,
		Description: r.Description,
		SendMessage: ensureInlineMessage(msg, r.Caption),
	}
	// content (flag 5) = the actual photo; thumb (flag 4) = the thumbnail.
	// Do NOT put photo_url in the url (flag 3) field — that's the open-URL
	// behavior, not the media. Mirrors TDLib inputInlineQueryResultPhoto.
	if r.PhotoURL != "" {
		result.Content = &tg.InputWebDocument{URL: r.PhotoURL, Size: 0, MimeType: "image/jpeg"}
	}
	if thumb != "" {
		result.Thumb = &tg.InputWebDocument{URL: thumb, Size: 0, MimeType: "image/jpeg"}
	}
	result.SetFlags()
	return result, nil
}

func convertInlineResultDocument(id string, raw json.RawMessage, docType string) (tg.InputBotInlineResultClass, error) {
	var r struct {
		Title               string          `json:"title"`
		Description         string          `json:"description"`
		ThumbURL            string          `json:"thumb_url"`
		ThumbnailURL        string          `json:"thumbnail_url"`
		MimeType            string          `json:"mime_type"`
		Caption             string          `json:"caption"`
		InputMessageContent json.RawMessage `json:"input_message_content"`
		ReplyMarkup         json.RawMessage `json:"reply_markup"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg, err := convertInputMessageContent(r.InputMessageContent)
	if err != nil {
		return nil, err
	}
	thumb := r.ThumbURL
	if thumb == "" {
		thumb = r.ThumbnailURL
	}
	// Each media type carries its content in a type-specific URL field
	// (gif_url, mpeg4_url, video_url, audio_url, voice_url, document_url),
	// falling back to a <type>_file_id field — NOT the generic "url" field
	// (which is the open-URL behavior). Mirrors the reference Client.cpp
	// gif/mpeg4_gif/video/audio/voice/document cases.
	mediaURL := jsonFieldString(raw, docType+"_url")
	if mediaURL == "" {
		mediaURL = jsonFieldString(raw, docType+"_file_id")
	}
	mime := r.MimeType
	if mime == "" {
		mime = defaultInlineMime(docType)
	}
	result := &tg.InputBotInlineResult{
		ID:          id,
		Type:        docType,
		Title:       r.Title,
		Description: r.Description,
		SendMessage: ensureInlineMessage(msg, r.Caption),
	}
	if mediaURL != "" {
		result.Content = &tg.InputWebDocument{URL: mediaURL, Size: 0, MimeType: mime}
	}
	if thumb != "" {
		result.Thumb = &tg.InputWebDocument{URL: thumb, Size: 0, MimeType: "image/jpeg"}
	}
	result.SetFlags()
	return result, nil
}

func convertInlineResultLocation(id string, raw json.RawMessage, resultType string) (tg.InputBotInlineResultClass, error) {
	var r struct {
		Latitude            float64         `json:"latitude"`
		Longitude           float64         `json:"longitude"`
		Title               string          `json:"title"`
		ThumbURL            string          `json:"thumb_url"`
		InputMessageContent json.RawMessage `json:"input_message_content"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg, err := convertInputMessageContent(r.InputMessageContent)
	if err != nil {
		return nil, err
	}
	result := &tg.InputBotInlineResult{
		ID:          id,
		Type:        resultType,
		Title:       r.Title,
		SendMessage: ensureInlineMessage(msg, ""),
	}
	if r.ThumbURL != "" {
		result.Thumb = &tg.InputWebDocument{URL: r.ThumbURL, Size: 0, MimeType: "image/jpeg"}
		result.Flags.Set(3)
	}
	result.SetFlags()
	return result, nil
}

func convertInlineResultContact(id string, raw json.RawMessage) (tg.InputBotInlineResultClass, error) {
	var r struct {
		PhoneNumber         string          `json:"phone_number"`
		FirstName           string          `json:"first_name"`
		LastName            string          `json:"last_name"`
		ThumbURL            string          `json:"thumb_url"`
		InputMessageContent json.RawMessage `json:"input_message_content"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg, err := convertInputMessageContent(r.InputMessageContent)
	if err != nil {
		return nil, err
	}
	result := &tg.InputBotInlineResult{
		ID:          id,
		Type:        "contact",
		Title:       strings.TrimSpace(r.FirstName + " " + r.LastName),
		SendMessage: ensureInlineMessage(msg, ""),
	}
	if r.ThumbURL != "" {
		result.Thumb = &tg.InputWebDocument{URL: r.ThumbURL, Size: 0, MimeType: "image/jpeg"}
		result.Flags.Set(3)
	}
	result.SetFlags()
	return result, nil
}

func convertInlineResultGame(id string, raw json.RawMessage) (tg.InputBotInlineResultClass, error) {
	var r struct {
		GameShortName string `json:"game_short_name"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &tg.InputBotInlineResultGame{
		ID:        id,
		ShortName: r.GameShortName,
	}, nil
}

// convertInputMessageContent converts a Bot API InputMessageContent JSON into
// a tg.InputBotInlineMessageClass.
func convertInputMessageContent(raw json.RawMessage) (tg.InputBotInlineMessageClass, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var probe struct {
		MessageText string  `json:"message_text"`
		Latitude    float64 `json:"latitude"`
		Title       string  `json:"title"`
		PhoneNumber string  `json:"phone_number"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if probe.MessageText != "" {
		return convertInputTextMessage(raw)
	}
	if probe.Latitude != 0 {
		if probe.Title != "" {
			return convertInputVenueMessage(raw)
		}
		return convertInputLocationMessage(raw)
	}
	if probe.PhoneNumber != "" {
		return convertInputContactMessage(raw)
	}
	return nil, nil
}

func convertInputTextMessage(raw json.RawMessage) (tg.InputBotInlineMessageClass, error) {
	var r struct {
		MessageText string `json:"message_text"`
		ParseMode   string `json:"parse_mode"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg := &tg.InputBotInlineMessageText{
		Message: r.MessageText,
	}
	msg.SetFlags()
	return msg, nil
}

func convertInputLocationMessage(raw json.RawMessage) (tg.InputBotInlineMessageClass, error) {
	var r struct {
		Latitude             float64 `json:"latitude"`
		Longitude            float64 `json:"longitude"`
		HorizontalAccuracy   float64 `json:"horizontal_accuracy"`
		LivePeriod           int     `json:"live_period"`
		Heading              int     `json:"heading"`
		ProximityAlertRadius int     `json:"proximity_alert_radius"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	geo := &tg.InputGeoPoint{Lat: r.Latitude, Long: r.Longitude}
	if r.HorizontalAccuracy > 0 {
		geo.AccuracyRadius = int32(r.HorizontalAccuracy)
		geo.Flags.Set(0)
	}
	msg := &tg.InputBotInlineMessageMediaGeo{GeoPoint: geo}
	if r.LivePeriod > 0 {
		msg.Period = int32(r.LivePeriod)
		msg.Flags.Set(1)
	}
	if r.Heading > 0 {
		msg.Heading = int32(r.Heading)
		msg.Flags.Set(0)
	}
	if r.ProximityAlertRadius > 0 {
		msg.ProximityNotificationRadius = int32(r.ProximityAlertRadius)
		msg.Flags.Set(3)
	}
	msg.SetFlags()
	return msg, nil
}

func convertInputVenueMessage(raw json.RawMessage) (tg.InputBotInlineMessageClass, error) {
	var r struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Title     string  `json:"title"`
		Address   string  `json:"address"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg := &tg.InputBotInlineMessageMediaVenue{
		GeoPoint: &tg.InputGeoPoint{Lat: r.Latitude, Long: r.Longitude},
		Title:    r.Title,
		Address:  r.Address,
	}
	msg.SetFlags()
	return msg, nil
}

func convertInputContactMessage(raw json.RawMessage) (tg.InputBotInlineMessageClass, error) {
	var r struct {
		PhoneNumber string `json:"phone_number"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		VCard       string `json:"vcard"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	msg := &tg.InputBotInlineMessageMediaContact{
		PhoneNumber: r.PhoneNumber,
		FirstName:   r.FirstName,
		LastName:    r.LastName,
	}
	if r.VCard != "" {
		msg.Vcard = r.VCard
		msg.Flags.Set(0)
	}
	msg.SetFlags()
	return msg, nil
}

// InlineMessageIDFromStr parses a Bot API inline_message_id string into an
// InputBotInlineMessageIDClass.
func InlineMessageIDFromStr(s string) (tg.InputBotInlineMessageIDClass, error) {
	if s == "" {
		return nil, errors.New("inline_message_id is required")
	}
	data, err := decodeInlineMessageID(s)
	if err != nil {
		return nil, fmt.Errorf("invalid inline_message_id: %w", err)
	}
	return parseInlineMessageID(data)
}

func decodeInlineMessageID(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}

func parseInlineMessageID(data []byte) (tg.InputBotInlineMessageIDClass, error) {
	if len(data) < 16 {
		return nil, errors.New("inline_message_id too short")
	}
	if len(data) >= 24 {
		dcID := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16 | int32(data[3])<<24
		ownerID := int64(data[4]) | int64(data[5])<<8 | int64(data[6])<<16 | int64(data[7])<<24 |
			int64(data[8])<<32 | int64(data[9])<<40 | int64(data[10])<<48 | int64(data[11])<<56
		id := int32(data[12]) | int32(data[13])<<8 | int32(data[14])<<16 | int32(data[15])<<24
		accessHash := int64(data[16]) | int64(data[17])<<8 | int64(data[18])<<16 | int64(data[19])<<24 |
			int64(data[20])<<32 | int64(data[21])<<40 | int64(data[22])<<48 | int64(data[23])<<56
		return &tg.InputBotInlineMessageID64{
			DCID:       dcID,
			OwnerID:    ownerID,
			ID:         id,
			AccessHash: accessHash,
		}, nil
	}
	return nil, errors.New("unsupported inline_message_id format")
}

// HighScores converts tg.HighScore slices + user map into Bot API GameHighScore.
func HighScores(scores []*tg.HighScore, users map[int64]*tg.User) []apitypes.GameHighScore {
	result := make([]apitypes.GameHighScore, 0, len(scores))
	for _, s := range scores {
		u, ok := users[s.UserID]
		var user *apitypes.User
		if ok {
			user = User(u)
		} else {
			user = &apitypes.User{ID: s.UserID}
		}
		result = append(result, apitypes.GameHighScore{
			Position: int(s.Pos),
			User:     user,
			Score:    int(s.Score),
		})
	}
	return result
}

// InlineMessageIDFromTL converts a tg.InputBotInlineMessageIDClass into a
// Bot API inline_message_id string (base64-encoded).
func InlineMessageIDFromTL(id tg.InputBotInlineMessageIDClass) string {
	if id == nil {
		return ""
	}
	switch msgID := id.(type) {
	case *tg.InputBotInlineMessageID:
		var buf [16]byte
		buf[0] = byte(msgID.DCID)
		buf[1] = byte(msgID.DCID >> 8)
		buf[2] = byte(msgID.DCID >> 16)
		buf[3] = byte(msgID.DCID >> 24)
		buf[4] = byte(msgID.ID)
		buf[5] = byte(msgID.ID >> 8)
		buf[6] = byte(msgID.ID >> 16)
		buf[7] = byte(msgID.ID >> 24)
		buf[8] = byte(msgID.AccessHash)
		buf[9] = byte(msgID.AccessHash >> 8)
		buf[10] = byte(msgID.AccessHash >> 16)
		buf[11] = byte(msgID.AccessHash >> 24)
		buf[12] = byte(msgID.AccessHash >> 32)
		buf[13] = byte(msgID.AccessHash >> 40)
		buf[14] = byte(msgID.AccessHash >> 48)
		buf[15] = byte(msgID.AccessHash >> 56)
		return base64.RawURLEncoding.EncodeToString(buf[:])
	case *tg.InputBotInlineMessageID64:
		var buf [24]byte
		buf[0] = byte(msgID.DCID)
		buf[1] = byte(msgID.DCID >> 8)
		buf[2] = byte(msgID.DCID >> 16)
		buf[3] = byte(msgID.DCID >> 24)
		buf[4] = byte(msgID.OwnerID)
		buf[5] = byte(msgID.OwnerID >> 8)
		buf[6] = byte(msgID.OwnerID >> 16)
		buf[7] = byte(msgID.OwnerID >> 24)
		buf[8] = byte(msgID.OwnerID >> 32)
		buf[9] = byte(msgID.OwnerID >> 40)
		buf[10] = byte(msgID.OwnerID >> 48)
		buf[11] = byte(msgID.OwnerID >> 56)
		buf[12] = byte(msgID.ID)
		buf[13] = byte(msgID.ID >> 8)
		buf[14] = byte(msgID.ID >> 16)
		buf[15] = byte(msgID.ID >> 24)
		buf[16] = byte(msgID.AccessHash)
		buf[17] = byte(msgID.AccessHash >> 8)
		buf[18] = byte(msgID.AccessHash >> 16)
		buf[19] = byte(msgID.AccessHash >> 24)
		buf[20] = byte(msgID.AccessHash >> 32)
		buf[21] = byte(msgID.AccessHash >> 40)
		buf[22] = byte(msgID.AccessHash >> 48)
		buf[23] = byte(msgID.AccessHash >> 56)
		return base64.RawURLEncoding.EncodeToString(buf[:])
	default:
		return ""
	}
}

// KeyboardButtonFromJSON converts a Bot API KeyboardButton JSON into a
// tg.KeyboardButtonClass for savePreparedKeyboardButton.
func KeyboardButtonFromJSON(jsonStr string) (tg.KeyboardButtonClass, error) {
	var btn struct {
		Text   string `json:"text"`
		URL    string `json:"url,omitempty"`
		WebApp struct {
			URL string `json:"url"`
		} `json:"web_app"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &btn); err != nil {
		return nil, fmt.Errorf("invalid button JSON: %w", err)
	}
	if btn.Text == "" {
		return nil, errors.New("button text is required")
	}
	if btn.WebApp.URL != "" {
		return &tg.KeyboardButtonWebView{
			Text: btn.Text,
			URL:  btn.WebApp.URL,
		}, nil
	}
	if btn.URL != "" {
		return &tg.KeyboardButtonURL{
			Text: btn.Text,
			URL:  btn.URL,
		}, nil
	}
	return &tg.KeyboardButton{Text: btn.Text}, nil
}
