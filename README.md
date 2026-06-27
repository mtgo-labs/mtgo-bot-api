<div align="center">

# mtgo-bot-api

<p>A drop-in Go reimplementation of the official <a href="https://github.com/tdlib/telegram-bot-api">Telegram Bot API server</a>, built on <a href="https://github.com/mtgo-labs/mtgo">mtgo</a> — a pure-Go MTProto client.</p>

<p>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://core.telegram.org/bots/api"><img src="https://img.shields.io/badge/Bot%20API-10.1-26A5E4?logo=telegram&logoColor=white" alt="Bot API"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License"></a>
  <img src="https://img.shields.io/badge/CGO-None-success" alt="No CGO">
</p>

<p><b>100% behavioral fidelity</b> to the official Bot API server — identical method names, parameters, JSON response envelopes, error codes, and a persistent update queue (TQueue). If it works against <code>api.telegram.org</code>, it works here.</p>

</div>

---

## Table of Contents

- [Why?](#why)
- [Status](#status)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Supported Clients](#supported-clients)
- [Architecture](#architecture)
- [Feature Coverage](#feature-coverage)
- [Update Delivery](#update-delivery)
- [Webhooks](#webhooks)
- [File Handling](#file-handling)
- [Security](#security)
- [Testing & Verification](#testing--verification)
- [Project Layout](#project-layout)
- [Examples](#examples)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Why?

The official `telegram-bot-api` server is a C++ binary that links TDLib. It requires a C++ toolchain,
gRPC, and OpenSSL to build — and TDLib's actor model adds overhead and complexity that isn't always
needed.

**mtgo-bot-api** is a single statically-linked Go binary with:

- **Zero CGO** — SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go).
- **No HTTP frameworks** — stdlib `net/http` only.
- **One binary, one process** — no external dependencies beyond the Telegram network.
- **Direct MTProto** — talks the raw TL layer (`tg.RPCClient`) for precise 1:1 control over every
  RPC call, mirroring how the official server invokes `td_api` methods directly.
- **Multi-bot by design** — route any number of bot tokens through one server instance.

---

## Status

| Area | Status |
|------|--------|
| **Bot API version** | **10.1** |
| **HTTP methods** | **180** registered — **100% of official Bot API** (schema-certified, see [`schema/COVERAGE.md`](schema/COVERAGE.md)) |
| **Update types** | **22/22** (100%) |
| **Response field parity** | **26/26** verified vs official `api.telegram.org` |
| **file_id / file_unique_id** | Byte-identical parity verified end-to-end |
| **Test files** | 89 |
| **Lines of Go** | ~24,000 production (~40,000 incl. tests) |
| **Avg test coverage** (all packages) | **~82%** — full suite green |

> Server version defaults to `0.1.0`; release builds inject the git tag via
> `-ldflags` (see [`internal/version`](internal/version/version.go) and the
> `Release` workflow).

---

## Installation

### Option 1: Install script (recommended)

Download and install the latest release binary to `~/.local/bin`:

```bash
# Using curl
curl -fsSL https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | sh

# Using wget
wget -qO- https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | sh
```

The script auto-detects your OS and architecture, downloads the matching
prebuilt binary from GitHub Releases, and installs it to `~/.local/bin`.
If no prebuilt binary is available, it falls back to `go install`.

Customize the install location or version:

```bash
curl -fsSL https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | INSTALL_DIR=/usr/local/bin sh
curl -fsSL https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | VERSION=v0.2.0 sh
```

### Option 2: go install

If you have Go 1.26+ installed:

```bash
go install github.com/mtgo-labs/mtgo-bot-api/cmd/mtgo-bot-api@latest
```

The binary lands in `$(go env GOPATH)/bin`. Ensure that directory is in your
`PATH`.

### Option 3: Build from source

```bash
# Clone — the mtgo sibling repo is required (go.mod uses a local replace)
git clone https://github.com/mtgo-labs/mtgo-bot-api.git
cd mtgo-bot-api

# Build a static binary (no CGO)
CGO_ENABLED=0 go build -o mtgo-bot-api ./cmd/mtgo-bot-api

# Or run directly
go run ./cmd/mtgo-bot-api --version
# → mtgo-bot-api v0.1.0
# → Bot API 10.1
```

> **Sibling repo:** `go.mod` currently uses `replace github.com/mtgo-labs/mtgo => ../mtgo`
> for local development. The `../mtgo` repo must exist alongside this repo:
>
> ```
> mtgo-labs/
> ├── mtgo/              ← MTProto client (required locally)
> ├── storage/           ← mtgo storage interfaces
> └── mtgo-bot-api/      ← this repo
> ```

---

## Quick Start

Start the server with your API credentials:

```bash
mtgo-bot-api --api-id <API_ID> --api-hash <API_HASH>
# → listens on :8081
```

Point any client at `http://localhost:8081`:

```bash
# getMe — verify the bot connects
curl http://localhost:8081/bot<TOKEN>/getMe

# Send a message
curl http://localhost:8081/bot<TOKEN>/sendMessage \
  -d chat_id=<CHAT_ID> \
  -d text="Hello from mtgo-bot-api"
```

The URL format — `http://localhost:8081/bot<TOKEN>/<METHOD>` — matches the
official server exactly.


## Configuration

All flags can also be set via environment variables (`$TELEGRAM_API_ID`, `$TELEGRAM_API_HASH`).

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--api-id` | `TELEGRAM_API_ID` | **required** | Telegram application API ID |
| `--api-hash` | `TELEGRAM_API_HASH` | **required** | Telegram application API hash |
| `--local` | — | `false` | Local Bot API mode (removes 20 MB file download limit) |
| `--http-port` | — | `8081` | HTTP listening port |
| `--http-stat-port` | — | `0` (off) | Separate port for the statistics endpoint |
| `--http-ip-address` | — | `""` (all) | Local IP address to bind HTTP on |
| `--dir` | — | `.mtgo-bot-api` | Working directory (SQLite DBs, sessions, file cache) |
| `--temp-dir` | — | `os.TempDir()` | Directory for temporary file uploads |
| `--max-webhook-connections` | — | `100` | Default max concurrent webhook connections per bot |
| `--filter` | — | `""` | `<remainder>/<modulo>` — only serve bots where `bot_user_id % modulo == remainder` |
| `--proxy` | — | `""` | HTTP proxy for outgoing webhook requests (`http://host:port`) |
| `--verbosity` | — | `1` | Log verbosity: `0`=FATAL, `1`=+ERROR, `2`=+WARN, `3`=+INFO, `4`=+DEBUG |
| `--version` | — | — | Print version and exit |

### Examples

```bash
# Local mode, custom port, debug logging
go run ./cmd/mtgo-bot-api --api-id 12345 --api-hash abcdef --local --http-port 8082 --verbosity 4

# Stats endpoint on a separate port
go run ./cmd/mtgo-bot-api --api-id 12345 --api-hash abcdef --http-stat-port 9090

# Bind to localhost only
go run ./cmd/mtgo-bot-api --api-id 12345 --api-hash abcdef --http-ip-address 127.0.0.1

# Sharding: serve only even-numbered bot user IDs
go run ./cmd/mtgo-bot-api --api-id 12345 --api-hash abcdef --filter 0/2
```

---

## Supported Clients

**Every Bot API client that works against `api.telegram.org` works identically
against this server** — that is the fidelity guarantee. No SDK changes needed;
just point the base URL at your local instance.

In Go, this includes all libraries built on the official `telegram-bot-api`
HTTP contract — [telebot](https://github.com/tucnak/telebot),
[telego](https://github.com/mymmrac/telego),
[gotgbot](https://github.com/PaulSonOfLars/gotgbot),
[telebot.v3](https://gopkg.in/telebot.v3), and any custom client using the
standard Bot API JSON schema.

Example bots (Go, TypeScript, Python) are in [`examples/`](examples/).

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Request                             │
│              POST /bot<TOKEN>/<method>                          │
└──────────┬──────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────┐    parses multipart/form-data + query params
│  internal/server     │──────────────────────────────┐
│  (net/http, no fwk)  │                               │
└──────────┬───────────┘                               ▼
           │                              ┌─────────────────────┐
           │  routes by bot token         │  internal/response  │
           ▼                              │  JSON envelope:     │
┌──────────────────────┐                  │  {ok,result,error}  │
│  internal/manager     │◄─────────────────┤                     │
│  ClientManager        │                  └─────────────────────┘
│  • token routing      │
│  • per-bot lifecycle  │
│  • flood control      │
└──────────┬───────────┘
           │  creates/reuses one Client per bot
           ▼
┌──────────────────────┐    raw tg.RPCClient ────► Telegram MTProto
│  internal/client     │                  ▲
│  Per-bot Client      │    ┌─────────────┴──────────────┐
│  • 180 handlers      │
│  • dispatch table    │    │  tg types ↔ Bot API JSON   │
└──┬───────┬───────┬───┘    └────────────────────────────┘
   │       │       │
   │       │       └──────────────────────────────────────────┐
   │       ▼                                                  ▼
   │ ┌────────────────┐                          ┌──────────────────┐
   │ │ internal/tqueue│                          │ internal/webhook │
   │ │ Update queue   │                          │ Outgoing webhooks│
   │ │ (monotonic IDs)│                          │ • SSRF protection│
   └───────┬────────┘                            │ • flood gates    │
           │                                      │ • retry/Retry-After│
           │ persists                             └──────────────────┘
           ▼
┌──────────────────────┐
│  internal/storage    │
│  SQLite (pure Go)    │
│  • update log        │
│  • webhook config    │
│  • peer cache        │
└──────────────────────┘
```

### Key Design Principles

1. **Raw TL layer, not high-level wrappers.** The per-bot `Client` constructs `tg.*Request` structs
   and invokes them via `tg.RPCClient` — the exact TL schema (layer 225). No opinionated convenience
   methods. This mirrors how the official server calls `td_api` methods directly rather than through
   wrappers.

2. **Per-bot isolation.** Each bot gets its own `Client`, connection, session, and SQLite database
   (`.mtgo-bot-api/<bot_id>/bot.db`). No shared state between bots.

3. **Persistent update queue.** Updates survive restarts. The TQueue stores events with monotonic
   IDs and replays them on `getUpdates`. The storage callback persists every push to SQLite.

4. **Exact error mapping.** MTProto RPC errors are translated to Bot API error messages and HTTP
   codes exactly as the official `Client.cpp` does.

---

## Feature Coverage

### Implemented Bot API Method Categories

| Category | Methods | Notes |
|----------|---------|-------|
| **Getting updates** | `getUpdates` | Long polling, webhook conflict detection, 409 on concurrent calls, negative offset, byte-budget truncation |
| **Webhooks** | `setWebhook`, `deleteWebhook`, `getWebhookInfo` | HTTPS validation, secret token, TCP/TLS readiness probe, max_connections |
| **Messages** | `sendMessage`, `editMessageText/Caption/Media/ReplyMarkup`, `deleteMessage(s)`, `forwardMessage(s)`, `copyMessage(s)` | HTML + MarkdownV2 + plain text |
| **Media** | `sendPhoto`, `sendDocument`, `sendAudio`, `sendVideo`, `sendAnimation`, `sendVoice`, `sendVideoNote`, `sendSticker`, `sendMediaGroup` | Upload + re-send by file_id |
| **Interactive** | `sendContact`, `sendVenue`, `sendLocation`, `sendDice`, `sendPoll`, `stopPoll`, `sendPaidMedia`, `setMessageReaction` | |
| **Chat management** | `getChat`, `getChatMember(s)`, `leaveChat`, `setChatTitle/Description/Photo/Permissions/StickerSet/MenuButton` | |
| **Admin** | `promoteChatMember`, `restrictChatMember`, `banChatMember`, `unbanChatMember`, `banChatSenderChat` | `ChatBannedRights` inversion handled |
| **Forum topics** | `createForumTopic`, `editForumTopic`, `close/reopen/deleteForumTopic`, `edit/close/reopen/hide/unhideGeneralForumTopic` | 12 methods |
| **Sticker sets** | `getStickerSet`, `createNewStickerSet`, `addStickerToSet`, `setStickerSetTitle/Thumbnail/EmojiList`, `deleteStickerSet` | 16 methods, full CRUD |
| **Inline mode** | `answerInlineQuery`, `answerCallbackQuery`, `setGameScore`, `getGameHighScores`, `answerWebAppQuery`, `answerGuestQuery`, `savePreparedInlineMessage/KeyboardButton`, `sendCustomRequest`, `answerCustomQuery` | |
| **Bot profile** | `getMyCommands/setMyCommands/deleteMyCommands`, `getMyName/setMyName`, `getMyDescription/setMyDescription`, `getMyShortDescription/setMyShortDescription` | Via `BotsGetBotInfo`/`BotsSetBotInfo` |
| **Payments** | `sendInvoice`, `createInvoiceLink`, `answerShippingQuery`, `answerPreCheckoutQuery`, `refundStarPayment`, `getMyStarBalance`, `getStarTransactions`, `sendGift`, `giftPremiumSubscription`, `getAvailableGifts`, `getUserGifts`, `getChatGifts`, `transferGift`, `upgradeGift`, `convertGiftToStars`, `editUserStarSubscription`, `getUserChatBoosts` | Stars, invoices, gifts, subscriptions |
| **Business** | `getBusinessConnection`, `setBusinessAccountName/Bio/Username`, `readBusinessMessage`, `deleteBusinessMessages`, `setBusinessAccountProfilePhoto`, `getBusinessAccountStarBalance`, `transferBusinessAccountStars`, `getBusinessAccountGifts` | 12 methods |
| **Stories** | `postStory`, `editStory`, `deleteStory`, `repostStory`, `sendLivePhoto` | |
| **Rich messages** | `sendRichMessage`, `sendRichMessageDraft`, `sendMessageDraft`, `sendChecklist`, `editMessageChecklist` | RichBlock (21 types) + RichText (27 types) |
| **File operations** | `getFile`, `uploadStickerFile` | Download via `upload.getFile`, file_id parity |
| **Verification** | `verifyChat`, `verifyUser`, `removeChatVerification`, `removeUserVerification` | |
| **Lifecycle** | `getMe`, `logout`, `close` | |
| **Invite links** | `exportChatInviteLink`, `createChatInviteLink`, `editChatInviteLink`, `revokeChatInviteLink`, `createChatSubscriptionInviteLink`, `approve/declineChatJoinRequest`, `answerChatJoinRequestQuery` | |
| **Moderation** | `approveSuggestedPost`, `declineSuggestedPost`, `deleteMessageReaction`, `deleteAllMessageReactions` | |

### Update Types

All **22** Bot API update types are delivered via `getUpdates` and webhooks:

```
message                    deleted_business_messages   message_reaction_count
edited_message             callback_query              poll
channel_post               inline_query                poll_answer
edited_channel_post        chosen_inline_result        business_connection
business_message           chat_join_request           chat_boost
edited_business_message    chat_member                 removed_chat_boost
my_chat_member             shipping_query
                           pre_checkout_query
```

`allowed_updates` filtering is fully implemented:
- **Default** (nil/empty): excludes `chat_member`, `message_reaction`, `message_reaction_count`,
  `chat_boost`, `removed_chat_boost`.
- **Explicit** (non-nil array): only the listed types are delivered.
- Filtering applies at push time (TQueue) and is persisted per webhook.

---

## Update Delivery

Updates flow through a **monotonic update queue (TQueue)** with these guarantees:

- **Persistent**: Every event is written to SQLite on push. On restart, the queue replays
  undelivered events.
- **Monotonic IDs**: `update_id` values are strictly increasing per bot. Clients acknowledge by
  passing `offset = last_update_id + 1`.
- **Long polling**: `getUpdates` blocks for up to `timeout` seconds (default 0), shaped exactly like
  the official server — including the 3-second delayed conflict response and 50-second hard cap.
- **Conflict detection**: A second concurrent `getUpdates` (with an older offset) gets HTTP `409
  Conflict`. Webhook-active bots get `409` on any `getUpdates` call.
- **Negative offset**: Passing `offset < 0` clears the queue but keeps the newest events.
- **Size budget**: A single `getUpdates` response is capped at ~4 MB (`1 << 22` bytes), truncating
  the event list if it overflows.

---

## Webhooks

```bash
# Set a webhook
curl http://localhost:8081/bot<TOKEN>/setWebhook \
  -d 'url=https://example.com/webhook' \
  -d 'secret_token=mysecret'

# Check webhook status
curl http://localhost:8081/bot<TOKEN>/getWebhookInfo

# Delete (switch back to polling)
curl http://localhost:8081/bot<TOKEN>/deleteWebhook
```

Webhook delivery mirrors the official `WebhookActor` semantics:

- **Verification**: `setWebhook` returns only after the server confirms outbound TCP/TLS readiness
  to the webhook URL.
- **Concurrency**: Respects `max_connections` per bot (configurable, default 100).
- **Ordering**: Events within a single queue are delivered in order.
- **Retry**: Exponential backoff with `Retry-After` header clamping. Randomized resend cap.
- **Expiry**: Events past their TTL are dropped (not retried forever).
- **Sustained 410**: Repeated `410 Gone` responses close the webhook.
- **DNS/IP cache**: Resolved endpoints are cached to avoid redundant lookups.
- **Flood gates**: Active and pending connection counts are flood-controlled.

A webhook receiver for testing is in [`examples/webhook/receiver.ts`](examples/webhook/receiver.ts).

---

## File Handling

### Uploads

Files are uploaded via `multipart/form-data` and staged in `--temp-dir`, then uploaded to Telegram
in 512 KB chunks (`upload.saveFilePart` / `upload.saveBigFilePart` for files > 10 MB). Large file
uploads include flood-control awareness.

In `--local` mode, the 20 MB file download size limit is removed.

### Downloads

`getFile` resolves a `file_id` to a download path. The server streams the file via
`upload.getFile` in 1 MB chunks. In `--local` mode, the file is served directly from the working
directory.

### file_id / file_unique_id Parity

File ID encoding and decoding is **byte-for-byte identical** to the official Bot API server.
Verified end-to-end against live `api.telegram.org`:

- All **28 `FileType` constants** (0–27) with correct type-class mapping.
- **Legacy constructors**: `InputPeerPhotoFileLocationLegacy`, `InputStickerSetThumbLegacy`,
  `DialogPhotoSmallLegacy`, `DialogPhotoBigLegacy`, `StickerSetThumbLegacy`.
- **Web remote** file IDs via `InputWebFileLocation` → `upload.getWebFile`.
- **Generated** file IDs (map tiles, audio thumbnails) with full validation.
- **RLE encoding**, version bytes, and the exact serialization TDLib uses.

See [`internal/fileid/`](internal/fileid/) for the full encoder/decoder.

---

## Security

- **SSRF protection**: Outgoing webhook requests are validated against an IP blocklist (loopback,
  link-local, private ranges) to prevent server-side request forgery.
- **Symlink escape prevention**: File downloads cannot traverse symlinks outside the working
  directory.
- **HTTP timeouts**: All outgoing webhook requests have connect, read, and write timeouts.
- **Token validation**: Bot tokens are validated for format (shape + checksum) before client
  creation, returning official `401`/`421` envelopes for malformed tokens.
- **No secrets in logs**: Token values are never logged.

---

## Testing & Verification

```bash
go build ./...       # compile everything
go vet ./...         # static analysis
go test ./...        # run all unit tests
go test -cover ./... # with coverage
```

### Parity Verification

Field-shape parity is verified with a **grammY-based comparison harness** in
[`examples/test-bot/`](examples/test-bot/). The `/testall` command runs 26 Bot API methods against
both this server and `api.telegram.org` simultaneously, comparing every top-level field:

```bash
# Start the server, then run the comparison bot
BOT_TOKEN=<token> bun run examples/test-bot/index.ts
# Send /testall to the bot in Telegram
```

**Result**: 26/26 methods produce identical top-level field sets between this server and the
official Bot API.

### Schema Certification

Method parity is **continuously certified** against the official Bot API reference. The
[`schema/`](schema/) toolchain scrapes [core.telegram.org/bots/api](https://core.telegram.org/bots/api) into
machine-readable JSON (`methods.json`, `types.json`), then a `validate` command cross-checks every official
method against the handlers registered in `internal/client`:

```bash
go run ./schema/cmd/validate     # schema vs implementation parity (exit 1 on gaps)
./schema/regen.sh                # scrape + validate + generate reports
```

**Result: 180/180 official methods registered (100%).** The generated
[`schema/COVERAGE.md`](schema/COVERAGE.md) lists any gaps, deprecated aliases, and mtgo-only extensions
(`close`, `logout`). See [`schema/README.md`](schema/README.md) for the full toolchain.

### Test Coverage by Package

| Package | Coverage |
|---------|----------|
| `internal/stats` | 100% — counters, snapshots, concurrency |
| `internal/response` | 98% — JSON envelope serialization |
| `internal/tqueue` | 98% — queue push/pop/replay/GC |
| `internal/types` | 97% — marshal, float, rich types |
| `internal/log` | 88% — verbosity, formatting |
| `internal/manager` | 86% — client lifecycle, token routing |
| `internal/fileid` | 84% — encode/decode/round-trip fixtures |
| `cmd/mtgo-bot-api` | 83% — CLI flags, bootstrap, shutdown |
| `internal/convert` | 80% — entity, media, chat, rich conversion |
| `internal/webhook` | 75% — SSRF, delivery, lifecycle |
| `internal/server` | 74% — request parsing, multipart, query |
| `internal/storage` | 74% — SQLite persistence |
| `internal/client` | 24% — parity, params, behavioral, dispatch (growing) |

> Average across all 13 packages: **~82%** (`go test -cover ./...`; full suite green). `internal/client`
> is the large god-package mirroring `Client.cpp`; its 180-handler surface is exercised primarily
> through the live parity harness and `schema/cmd/validate` certification rather than unit mocks.

---

## Project Layout

```
mtgo-bot-api/
├── cmd/
│   └── mtgo-bot-api/          # Entry point: CLI flags, HTTP server bootstrap, graceful shutdown
│
├── internal/
│   ├── server/                # Raw net/http server, multipart/query parsing (mirrors HttpServer/Query)
│   ├── manager/               # ClientManager: multi-bot lifecycle, token routing, flood control
│   ├── client/                # Per-bot Client: all 183 Bot API method handlers (raw tg.RPCClient)
│   ├── tqueue/                # TQueue: monotonic update queue + storage callback
│   ├── storage/               # SQLite persistence (TQueue log, webhooks, peers) — pure Go, no CGO
│   ├── webhook/               # Outgoing webhook delivery + SSRF protection
│   ├── convert/               # MTProto tg types ↔ Bot API JSON types
│   ├── types/                 # Bot API type structs (User, Chat, Message, Update, …)
│   ├── fileid/                # file_id / file_unique_id encode/decode (parity with TDLib)
│   ├── log/                   # TDLib-compatible stderr logger (ANSI colors, verbosity 0–4)
│   ├── response/              # JSON response envelope: {ok, result, error_code, description, parameters}
│   ├── stats/                 # Per-bot and global request/update statistics
│   └── version/               # Static build version + Bot API spec version
│
├── examples/                  # Multi-library example bots (Go, TypeScript, Python)
├── schema/                   # Scraped Bot API schema + parity certification (scrape/validate/generate)
└── go.mod
```

---

## Examples

The [`examples/`](examples/) directory contains minimal bots that connect to a local mtgo-bot-api
server and verify compatibility:

| Example | Language | Run |
|---------|----------|-----|
| [telebot](examples/telebot) | Go | `BOT_TOKEN=<token> go run ./examples/telebot` |
| [telego](examples/telego) | Go | `BOT_TOKEN=<token> go run ./examples/telego` |
| [gotgbot](examples/gotgbot) | Go | `BOT_TOKEN=<token> go run ./examples/gotgbot` |
| [grammY](examples/grammy) | TypeScript | `BOT_TOKEN=<token> bun run examples/grammy/bot.ts` |
| [pyTelegramBotAPI](examples/pytelegrambotapi) | Python | `BOT_TOKEN=<token> python examples/pytelegrambotapi/bot.py` |
| [aiogram](examples/aiogram) | Python | `BOT_TOKEN=<token> python examples/aiogram/bot.py` |
| **[test-bot](examples/test-bot)** | TypeScript | `BOT_TOKEN=<token> bun run examples/test-bot/index.ts` |

The Go examples share a single `go.mod` in the `examples/` directory — run `go mod tidy` there first.

The **test-bot** is a grammY-based comparison harness: the `/testall` command fires 26 Bot API
methods at both this server and `api.telegram.org`, then reports any field-shape differences.

---

## Development

### Build & Verify

```bash
go build ./...                            # compile
go vet ./...                              # static analysis
go test ./...                             # all tests
go test -race ./internal/tqueue/...       # race detector on concurrency-sensitive packages
go test -cover ./... 2>&1 | grep coverage # coverage summary
```

### Code Style

- **Go 1.26+**, stdlib `net/http` only (no HTTP frameworks).
- **No CGO** — SQLite via `modernc.org/sqlite`.
- **Commit style:** Conventional Commits with scope: `feat(client):`, `fix(tqueue):`, etc.
- **JSON tags:** snake_case matching the Bot API spec. Booleans never use `omitempty`.
- **Error mapping:** MTProto errors → Bot API error messages, matching `Client.cpp` exactly.
- **Logging:** TDLib-compatible stderr output via `internal/log` (no stdlib `log`).

### Key Convention: Raw `tg` Layer

mtgo-bot-api talks to MTProto via the **generated `tg` package**
(`github.com/mtgo-labs/mtgo/tg`), not mtgo's high-level `telegram.Client` convenience methods:

```go
cl, _ := telegram.NewClient(apiID, apiHash, &telegram.Config{BotToken: tok})
rpc := cl.RPC()  // *tg.RPCClient

result, err := rpc.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
    Peer:     inputPeer,
    Message:  text,
    RandomID: randID,
})
```

Build a `tg.RPCClient` once per bot, construct TL request structs in `internal/client/`, decode the
returned `tg.TLObject` in `internal/convert/`. See [`AGENTS.md`](AGENTS.md) for the full convention
guide and known mtgo/tg gotchas.

### Agent Tools

This repo is set up for AI-assisted development:

- **[CodeGraph](https://github.com/...)** index in `.codegraph/` — semantic code search.
- **AgentMemory MCP** — persistent cross-session memory for decisions and gotchas.
- **[spec-kit](https://github.com/github/spec-kit)** specs under `specs/`.

---

## Contributing

1. Read [`AGENTS.md`](AGENTS.md) for architecture conventions and known gotchas.
2. Verify parity against the official server before claiming a feature complete — use the
   `examples/test-bot/` harness or diff responses manually.
3. Follow existing patterns: per-method files in `internal/client/`, conversion in `internal/convert/`.
4. Add tests for behavioral changes. Bug fixes should include regression tests.
5. Use Conventional Commits: `feat(client): add sendVoice`, `fix(webhook): handle 410 correctly`.

---

## License

MIT — see [LICENSE](LICENSE).

---

<details>
<summary><b>References</b></summary>

- **Official Bot API server (reference):** [tdlib/telegram-bot-api](https://github.com/tdlib/telegram-bot-api)
- **MTProto client (dependency):** [mtgo-labs/mtgo](https://github.com/mtgo-labs/mtgo)
- **Bot API documentation:** [core.telegram.org/bots/api](https://core.telegram.org/bots/api)
- **Schema & parity certification:** [`schema/`](schema/) — scraped official reference + coverage reports

</details>
