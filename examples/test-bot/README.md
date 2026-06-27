# mtgo-bot-api Test Bot

A simple grammY bot for testing type fidelity between mtgo-bot-api and the official Telegram Bot API.

## Setup

```bash
cd test-bot
bun install
```

## Usage

### Against local mtgo-bot-api (default):
```bash
bun run start
# or
bun run local
```

### Against official Telegram API:
```bash
bun run official
# or
API_ROOT=https://api.telegram.org bun run start
```

### With a different token:
```bash
TOKEN=123456:ABC-DEF bun run start
```

## Commands

| Command | What it does |
|---|---|
| `/start` | Help menu |
| `/me` | Dump getMe response as JSON |
| `/chat` | Dump getChat response as JSON |
| `/user` | Dump your User object |
| `/keyboard` | Test inline keyboard + callback queries |
| `/photo` | Send a photo |
| `/media` | Send media group |
| `/poll` | Send a poll |
| `/dice` | Send a dice |
| `/location` | Send location |
| `/raw` | Dump raw message JSON |
| `/compare` | Compare getMe: local vs official API |

Any text message will echo back the parsed fields (from, chat, text, date).

## Configuration

| Env var | Default | Description |
|---|---|---|
| `TOKEN` | hardcoded test token | Bot token |
| `API_ROOT` | `http://localhost:8081` | API server root URL |
