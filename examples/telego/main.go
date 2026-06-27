// telego (github.com/mymmrac/telego) example pointing at a local mtgo-bot-api server.
//
// Run: BOT_TOKEN=<token> go run ./telego
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mymmrac/telego"
)

const apiRoot = "http://localhost:8081"

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN env var is required")
	}

	// WithAPIServer points telego at the local server.
	bot, err := telego.NewBot(token, telego.WithAPIServer(apiRoot))
	if err != nil {
		log.Fatalf("telego: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	me, err := bot.GetMe(ctx)
	if err != nil {
		log.Fatalf("getMe: %v", err)
	}
	log.Printf("✅ telego connected: @%s (%d)", me.Username, me.ID)

	updates, err := bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{Timeout: 10})
	if err != nil {
		log.Fatalf("long polling: %v", err)
	}

	log.Println("📡 telego polling…")
	for update := range updates {
		if update.Message == nil || update.Message.Text == "" {
			continue
		}
		switch update.Message.Text {
		case "/start":
			_, _ = bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: update.Message.Chat.ID},
				Text:   "👋 Hello from mtgo-bot-api via **telego**!",
			})
		case "/test":
			_, _ = bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: update.Message.Chat.ID},
				Text:   fmt.Sprintf("📦 telego round-trip OK (chat %d)", update.Message.Chat.ID),
			})
		}
	}
}
