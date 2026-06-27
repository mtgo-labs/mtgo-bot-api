// telebot (gopkg.in/telebot/v4) example pointing at a local mtgo-bot-api server.
//
// Run: BOT_TOKEN=<token> go run ./telebot
package main

import (
	"log"
	"os"
	"time"

	tele "gopkg.in/telebot.v4"
)

const apiRoot = "http://localhost:8081"

func main() {
	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		URL:    apiRoot, // point telebot at the local server
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("telebot: %v", err)
	}

	me := b.Me
	log.Printf("✅ telebot connected: @%s (%d)", me.Username, me.ID)

	b.Handle("/start", func(c tele.Context) error {
		return c.Send("👋 Hello from mtgo-bot-api via **telebot**!", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	})

	b.Handle("/test", func(c tele.Context) error {
		return c.Send("📦 telebot round-trip OK")
	})

	log.Println("📡 telebot polling…")
	b.Start()
}
