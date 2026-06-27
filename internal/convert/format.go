package convert

import (
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
	"github.com/mtgo-labs/mtgo/tg"
)

var htmlEntityRE = regexp.MustCompile(`(?is)<(/?)(b|strong|i|em|u|ins|s|strike|del|code|pre|tg-spoiler)(?:\s+[^>]*)?>`)

// FormattedText converts Bot API parse_mode/entities input to plain text plus
// raw TL message entities suitable for messages.sendMessage.
func FormattedText(text, parseMode, entitiesJSON string) (string, []tg.MessageEntityClass, error) {
	if entitiesJSON != "" {
		entities, err := botAPIEntities(entitiesJSON)
		return text, entities, err
	}
	switch strings.ToLower(parseMode) {
	case "", "none":
		return text, nil, nil
	case "html":
		return parseHTMLText(text)
	case "markdown", "markdownv2":
		return parseMarkdownText(text, parseMode)
	default:
		return "", nil, errors.New("unsupported parse_mode")
	}
}

func botAPIEntities(raw string) ([]tg.MessageEntityClass, error) {
	var in []apitypes.MessageEntity
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return nil, errors.New("can't parse entities JSON object")
	}
	out := make([]tg.MessageEntityClass, 0, len(in))
	for _, ent := range in {
		if e := botAPIEntity(ent); e != nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func botAPIEntity(ent apitypes.MessageEntity) tg.MessageEntityClass {
	offset, length := int32(ent.Offset), int32(ent.Length)
	switch ent.Type {
	case "mention":
		return &tg.MessageEntityMention{Offset: offset, Length: length}
	case "hashtag":
		return &tg.MessageEntityHashtag{Offset: offset, Length: length}
	case "bot_command":
		return &tg.MessageEntityBotCommand{Offset: offset, Length: length}
	case "url":
		return &tg.MessageEntityURL{Offset: offset, Length: length}
	case "email":
		return &tg.MessageEntityEmail{Offset: offset, Length: length}
	case "bold":
		return &tg.MessageEntityBold{Offset: offset, Length: length}
	case "italic":
		return &tg.MessageEntityItalic{Offset: offset, Length: length}
	case "code":
		return &tg.MessageEntityCode{Offset: offset, Length: length}
	case "pre":
		return &tg.MessageEntityPre{Offset: offset, Length: length, Language: ent.Language}
	case "text_link":
		return &tg.MessageEntityTextURL{Offset: offset, Length: length, URL: ent.URL}
	case "underline":
		return &tg.MessageEntityUnderline{Offset: offset, Length: length}
	case "strikethrough":
		return &tg.MessageEntityStrike{Offset: offset, Length: length}
	case "spoiler":
		return &tg.MessageEntitySpoiler{Offset: offset, Length: length}
	case "blockquote", "expandable_blockquote":
		return &tg.MessageEntityBlockquote{Offset: offset, Length: length}
	default:
		return nil
	}
}

type openEntity struct {
	typ    string
	offset int32
}

type entitySpan struct {
	typ    string
	offset int32
	length int32
}

func parseHTMLText(text string) (string, []tg.MessageEntityClass, error) {
	var out strings.Builder
	var spans []entitySpan
	var stack []openEntity
	last := 0
	matches := htmlEntityRE.FindAllStringSubmatchIndex(text, -1)
	for _, m := range matches {
		out.WriteString(html.UnescapeString(text[last:m[0]]))
		last = m[1]
		closing := m[2] >= 0 && m[3] > m[2]
		tag := strings.ToLower(text[m[4]:m[5]])
		typ := htmlTagType(tag)
		if !closing {
			stack = append(stack, openEntity{typ: typ, offset: utf16Len(out.String())})
			continue
		}
		idx := -1
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].typ == typ {
				idx = i
				break
			}
		}
		if idx < 0 {
			return "", nil, errors.New("can't parse entities")
		}
		start := stack[idx]
		stack = append(stack[:idx], stack[idx+1:]...)
		length := utf16Len(out.String()) - start.offset
		if length > 0 {
			spans = append(spans, entitySpan{typ: typ, offset: start.offset, length: length})
		}
	}
	out.WriteString(html.UnescapeString(text[last:]))
	if len(stack) != 0 {
		return "", nil, errors.New("can't parse entities")
	}
	return out.String(), spansToEntities(spans), nil
}

func htmlTagType(tag string) string {
	switch tag {
	case "b", "strong":
		return "bold"
	case "i", "em":
		return "italic"
	case "u", "ins":
		return "underline"
	case "s", "strike", "del":
		return "strikethrough"
	case "tg-spoiler":
		return "spoiler"
	default:
		return tag
	}
}

func parseMarkdownText(text, mode string) (string, []tg.MessageEntityClass, error) {
	var out strings.Builder
	var spans []entitySpan
	var stack []openEntity
	for i := 0; i < len(text); {
		if len(stack) > 0 && (stack[len(stack)-1].typ == "code" || stack[len(stack)-1].typ == "pre") {
			closeToken := "`"
			if stack[len(stack)-1].typ == "pre" {
				closeToken = "```"
			}
			if strings.HasPrefix(text[i:], closeToken) {
				start := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if length := utf16Len(out.String()) - start.offset; length > 0 {
					spans = append(spans, entitySpan{typ: start.typ, offset: start.offset, length: length})
				}
				i += len(closeToken)
				continue
			}
			out.WriteByte(text[i])
			i++
			continue
		}
		if text[i] == '\\' && i+1 < len(text) {
			out.WriteByte(text[i+1])
			i += 2
			continue
		}
		token, typ := markdownToken(text[i:], mode)
		if token == "" {
			out.WriteByte(text[i])
			i++
			continue
		}
		offset := utf16Len(out.String())
		idx := -1
		for j := len(stack) - 1; j >= 0; j-- {
			if stack[j].typ == typ {
				idx = j
				break
			}
		}
		if idx >= 0 {
			start := stack[idx]
			stack = append(stack[:idx], stack[idx+1:]...)
			if length := offset - start.offset; length > 0 {
				spans = append(spans, entitySpan{typ: typ, offset: start.offset, length: length})
			}
		} else {
			stack = append(stack, openEntity{typ: typ, offset: offset})
		}
		i += len(token)
	}
	if len(stack) != 0 {
		return "", nil, errors.New("can't parse entities")
	}
	return out.String(), spansToEntities(spans), nil
}

func markdownToken(s, mode string) (string, string) {
	lowerMode := strings.ToLower(mode)
	if strings.HasPrefix(s, "```") {
		return "```", "pre"
	}
	if strings.HasPrefix(s, "**") && lowerMode == "markdownv2" {
		return "**", "bold"
	}
	if strings.HasPrefix(s, "__") {
		if lowerMode == "markdownv2" {
			return "__", "underline"
		}
		return "__", "italic"
	}
	if strings.HasPrefix(s, "||") && lowerMode == "markdownv2" {
		return "||", "spoiler"
	}
	if strings.HasPrefix(s, "~") && lowerMode == "markdownv2" {
		return "~", "strikethrough"
	}
	if strings.HasPrefix(s, "*") {
		return "*", "bold"
	}
	if strings.HasPrefix(s, "_") {
		return "_", "italic"
	}
	if strings.HasPrefix(s, "`") {
		return "`", "code"
	}
	return "", ""
}

func spansToEntities(spans []entitySpan) []tg.MessageEntityClass {
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].offset < spans[j].offset })
	out := make([]tg.MessageEntityClass, 0, len(spans))
	for _, span := range spans {
		if ent := span.entity(); ent != nil {
			out = append(out, ent)
		}
	}
	return out
}

func (s entitySpan) entity() tg.MessageEntityClass {
	switch s.typ {
	case "bold":
		return &tg.MessageEntityBold{Offset: s.offset, Length: s.length}
	case "italic":
		return &tg.MessageEntityItalic{Offset: s.offset, Length: s.length}
	case "underline":
		return &tg.MessageEntityUnderline{Offset: s.offset, Length: s.length}
	case "strikethrough":
		return &tg.MessageEntityStrike{Offset: s.offset, Length: s.length}
	case "code":
		return &tg.MessageEntityCode{Offset: s.offset, Length: s.length}
	case "pre":
		return &tg.MessageEntityPre{Offset: s.offset, Length: s.length}
	case "spoiler":
		return &tg.MessageEntitySpoiler{Offset: s.offset, Length: s.length}
	default:
		return nil
	}
}

func utf16Len(s string) int32 {
	return int32(len(utf16.Encode([]rune(s))))
}
