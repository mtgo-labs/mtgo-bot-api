# mtgo-bot-api — multi-library example bots

Minimal bots that connect to a **local mtgo-bot-api server** (`http://localhost:8081`)
and verify it works with popular Bot API libraries. Each bot does `getMe` + handles
`/start`, proving field-shape compatibility end-to-end.

## Start the server first

```bash
cd ../..  # repo root
go run ./cmd/mtgo-bot-api --api-id <ID> --api-hash <HASH>
# → listens on :8081
```

## Run an example

Each subdirectory is independent. Set `BOT_TOKEN` and run:

| Library | Language | Directory | Run |
|---------|----------|-----------|-----|
| [telebot](https://gopkg.in/telebot.v3) | Go | `telebot/` | `BOT_TOKEN=<token> go run ./telebot` |
| [telego](https://github.com/mymmrac/telego) | Go | `telego/` | `BOT_TOKEN=<token> go run ./telego` |
| [gotgbot](https://github.com/PaulSonOfLars/gotgbot) | Go | `gotgbot/` | `BOT_TOKEN=<token> go run ./gotgbot` |
| [grammY](https://grammy.dev) | TypeScript | `grammy/` | `BOT_TOKEN=<token> bun run grammy/bot.ts` |
| [pyTelegramBotAPI](https://github.com/eternnoir/pyTelegramBotAPI) | Python | `pytelegrambotapi/` | `BOT_TOKEN=<token> python pytelegrambotapi/bot.py` |
| [aiogram](https://aiogram.dev) | Python | `aiogram/` | `BOT_TOKEN=<token> python aiogram/bot.py` |
| **grammY (full test harness)** | TypeScript | `test-bot/` | `BOT_TOKEN=<token> bun run test-bot/index.ts` |

The Go examples share a single `go.mod` (run `go mod tidy` once in this directory).

## How each library points at the local server

- **telebot**: `telebot.Settings{URL: "http://localhost:8081"}`
- **telego**: `telego.WithAPIServer("http://localhost:8081")`
- **gotgbot**: custom `http.RoundTripper` rewrites `api.telegram.org` → `localhost:8081`
  (gotgbot has no native API-URL override)
- **grammY**: `new Bot(token, { client: { apiRoot: "http://localhost:8081" } })`
- **aiogram**: `Bot(token, session=..., base="http://localhost:8081")`

If a library works against the official `api.telegram.org`, it works identically
against this server — that is the fidelity guarantee.
