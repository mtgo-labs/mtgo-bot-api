// gotgbot (github.com/PaulSonOfLars/gotgbot) example pointing at a local
// mtgo-bot-api server. gotgbot has no native per-bot API-URL setting, so we
// pass a BaseBotClient whose DefaultRequestOpts.APIURL points at the local
// server (every request then goes there).
//
// Run: BOT_TOKEN=<token> go run ./gotgbot
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const apiRoot = "http://localhost:8081"

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN env var is required")
	}

	bot, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		BotClient: &gotgbot.BaseBotClient{
			Client: http.Client{Timeout: 30 * time.Second},
			// Route every request to the local mtgo-bot-api server.
			DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: apiRoot},
		},
	})
	if err != nil {
		log.Fatalf("gotgbot: %v", err)
	}

	log.Printf("✅ gotgbot connected: @%s (%d)", bot.Username, bot.Id)

	// Long-poll loop.
	offset := int64(0)
	log.Println("📡 gotgbot polling…")
	for {
		updates, err := bot.GetUpdates(&gotgbot.GetUpdatesOpts{
			Offset:  offset,
			Timeout: 10,
		})
		if err != nil {
			log.Printf("getUpdates: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateId + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			switch {
			case strings.HasPrefix(u.Message.Text, "/start"):
				_, _ = bot.SendMessage(u.Message.Chat.Id, "👋 Hello from mtgo-bot-api via *gotgbot*", &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
			case strings.HasPrefix(u.Message.Text, "/test"):
				_, _ = bot.SendMessage(u.Message.Chat.Id, "📦 gotgbot round-trip OK", nil)
			}
		}
	}
}
