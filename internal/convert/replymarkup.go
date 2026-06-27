package convert

import (
	"encoding/json"
	"fmt"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// ReplyMarkup parses a Bot API reply_markup JSON string into a raw
// tg.ReplyMarkupClass suitable for RPC calls. Currently supports
// InlineKeyboardMarkup (the most common type for bots). Returns nil for an
// empty string so callers can omit the field.
func ReplyMarkup(raw string) (tg.ReplyMarkupClass, error) {
	if raw == "" {
		return nil, nil
	}
	var markup struct {
		InlineKeyboard [][]apitypes.InlineKeyboardButton `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(raw), &markup); err != nil {
		return nil, fmt.Errorf("invalid reply_markup JSON: %w", err)
	}
	if markup.InlineKeyboard == nil {
		return nil, nil
	}
	rows := make([]*tg.KeyboardButtonRow, 0, len(markup.InlineKeyboard))
	for _, botRow := range markup.InlineKeyboard {
		buttons := make([]tg.KeyboardButtonClass, 0, len(botRow))
		for _, btn := range botRow {
			buttons = append(buttons, convertInlineButton(btn))
		}
		rows = append(rows, &tg.KeyboardButtonRow{Buttons: buttons})
	}
	return &tg.ReplyInlineMarkup{Rows: rows}, nil
}

// convertInlineButton maps a Bot API InlineKeyboardButton to the matching
// tg.KeyboardButtonClass constructor.
func convertInlineButton(btn apitypes.InlineKeyboardButton) tg.KeyboardButtonClass {
	switch {
	case btn.CallbackData != "":
		return &tg.KeyboardButtonCallback{Text: btn.Text, Data: []byte(btn.CallbackData)}
	case btn.URL != "":
		return &tg.KeyboardButtonURL{Text: btn.Text, URL: btn.URL}
	case btn.WebApp != nil && btn.WebApp.URL != "":
		return &tg.KeyboardButtonWebView{Text: btn.Text, URL: btn.WebApp.URL}
	case btn.SwitchInlineQueryChosenChat != nil:
		return &tg.KeyboardButtonSwitchInline{
			Text:  btn.Text,
			Query: btn.SwitchInlineQueryChosenChat.Query,
		}
	case btn.SwitchInlineQueryCurrentChat != "":
		return &tg.KeyboardButtonSwitchInline{
			Text:     btn.Text,
			Query:    btn.SwitchInlineQueryCurrentChat,
			SamePeer: true,
		}
	case btn.SwitchInlineQuery != "":
		return &tg.KeyboardButtonSwitchInline{Text: btn.Text, Query: btn.SwitchInlineQuery}
	case btn.Pay:
		return &tg.KeyboardButtonBuy{Text: btn.Text}
	default:
		return &tg.KeyboardButton{Text: btn.Text}
	}
}
