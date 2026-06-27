# AGENTS.md

## Project: mtgo-bot-api

A Go reimplementation of [`telegram-bot-api`](https://github.com/tdlib/telegram-bot-api)
(the official Telegram Bot API server built on TDLib). Instead of TDLib, this server
is built on top of **mtgo** (`github.com/mtgo-labs/mtgo`), a pure-Go MTProto client.

The goal is 100% behavioral fidelity to the official Bot API: identical HTTP API
(method names, parameters, response envelopes, error codes/messages), identical
Bot API types, and an update queue (TQueue) that persists across restarts.

## Agent Tools — Use First

- **AgentMemory MCP** is available. At session start run `memory_recall` for prior
  decisions (type gotchas, storage layout, error mapping). After major decisions run
  `memory_save`.
- **CodeGraph** is initialized (`.codegraph/`); prefer its MCP tools
  (`codegraph_explore`, `codegraph_search`, `codegraph_node`, `codegraph_callers`)
  over grep/Read for "how does X work", architecture, and where-is-X questions.

## Build & Verify

```bash
go build ./...          # compile everything
go vet ./...            # static analysis
go test ./...           # run all tests
go run ./cmd/mtgo-bot-api --help   # run the server
```

Requirements: Go 1.26+. Sibling repos `../mtgo` (and its `../storage`) must exist
locally because `go.mod` uses `replace github.com/mtgo-labs/mtgo => ../mtgo`.

## Monorepo Layout

```
mtgo-labs/
├── mtgo/            ← MTProto client (dependency)
├── storage/         ← mtgo storage interfaces (mtgo dependency)
├── mtgo-bot-api/    ← THIS repo — Bot API HTTP server
└── telegram-bot-api/← reference: official C++ source (read-only reference)
```

## Package Map (target)

| Path | Role |
|------|------|
| `cmd/mtgo-bot-api/` | Entry point: CLI flag parsing, HTTP server bootstrap |
| `internal/server/` | Raw `net/http` server, multipart/query parsing (mirrors HttpServer/Query) |
| `internal/manager/` | ClientManager: multi-bot lifecycle, token routing, flood control (mirrors ClientManager) |
| `internal/client/` | Per-bot Client: all Bot API method handlers (mirrors Client.cpp) |
| `internal/tqueue/` | TQueue: monotonic update queue + storage callback (mirrors TQueue) |
| `internal/storage/` | SQLite persistence for TQueue, webhooks, peers (mirrors TQueueBinlog) |
| `internal/webhook/` | Outgoing webhook delivery + setWebhook/deleteWebhook/getWebhookInfo |
| `internal/types/` | Bot API types matching the spec (User, Chat, Message, Update, …) |
| `internal/convert/` | mtgo `tg`/`telegram` types ↔ Bot API JSON types |
| `internal/fileid/` | file_id encode/decode |
| `internal/response/` | JSON response envelope: `{ok,result,error_code,description,parameters}` |
| `internal/stats/` | Per-bot and global request/update statistics |

## Conventions

- **Go 1.26+**, stdlib `net/http` only (no HTTP frameworks).
- **No CGO** — SQLite via `modernc.org/sqlite`.
- **Commit style:** Conventional Commits with scope: `feat(client):`, `fix(tqueue):`, etc.
- **Error mapping:** MTProto errors are translated to Bot API error messages exactly as in
  the official `Client.cpp` (see `internal/client/` error table).
- **Types:** JSON field names use snake_case matching the Bot API spec; structs use Go
  naming with `json:"..."` tags. Booleans never use `omitempty`.

## Code Quality Rules

_These operationalize constitution Principle 7 (`.specify/memory/constitution.md`); keep both in sync._

- Do not duplicate logic with different styles.
- Search the codebase before implementing new functionality.
- Reuse existing helpers, interfaces, types, middleware, and patterns.
- If the same logic appears more than once, consider extracting it.
- Follow existing architecture, naming conventions, and folder structures. Do not introduce new patterns unless clearly necessary.
- Prefer boring, simple, maintainable code over clever code.
- Do not over-engineer. Do not add unnecessary interfaces, factories, abstractions, or layers.
- For every important behavior change, add or update tests. Bug fixes must include regression tests when possible.
- Do not break public APIs, config formats, database schemas, or existing behavior unless explicitly requested. If a breaking change is required, explain it first.
- Use the project's existing error handling style. Do not introduce inconsistent error types or panic-based flow unless the project already uses that pattern.
- Avoid tautological or impossible conditions. Never write checks that are always true or always false (e.g. `nil == nil`, `x == x`, `true == true`, or a `nil` guard against a value that can never be nil). Fix any `tautological condition` / `impossible condition` diagnostic before considering the task complete — either make the condition meaningful or delete the dead branch.
- Before finishing any modified file, check for unused code and remove it. Never leave behind unused functions, variables, imports, constants, types, helpers, or dead code. Fix diagnostics such as `function "X" is unused [default]`. Do not keep speculative helper functions "for later". Do not ignore unused-code warnings. Required behavior: after editing a file, run the relevant diagnostics/static analysis; for Go projects run `go vet ./...` and `go test ./...`, plus editor/LSP diagnostics or the project linter (`golangci-lint`) to catch unused symbols. A task is not complete while the edited files still contain unused-code diagnostics.

## CRITICAL: Use raw `tg` TL layer, NOT high-level/bound methods

mtgo-bot-api talks to MTProto via the **generated `tg` package**
(`github.com/mtgo-labs/mtgo/tg`), NOT mtgo's high-level `telegram.Client`
convenience methods or its `Bound*` methods (`BoundSend`, `BoundAnswer`, …).

The pattern:

```go
// telegram.Client is used ONLY to own the connection/session/bot auth.
cl, _ := telegram.NewClient(apiID, apiHash, &telegram.Config{BotToken: tok})

// Then take the raw RPC client and invoke generated TL request structs directly:
rpc := cl.RPC() // returns *tg.RPCClient

result, err := rpc.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
    Peer:     ...,
    Message:  text,
    RandomID: ...,
    ReplyTo:  ...,
})
```

Why:
- Precise 1:1 control over MTProto calls, mirroring how the official telegram-bot-api
  invokes TDLib `td_api` methods directly rather than through opinionated wrappers.
- The high-level `telegram.Client` methods add abstractions/opinions that don't always
  map cleanly onto Bot API semantics.
- Generated request structs (`tg.MessagesSendMessageRequest`, `tg.BotsSetBotInfoRequest`,
  …) are the exact TL schema (layer 225).

Rule of thumb: build a `tg.RPCClient` once per bot, construct TL request structs in
`internal/client/`, decode the returned `tg.TLObject` in `internal/convert/`. Do **not**
import high-level `telegram` helpers that re-wrap these calls.

## Spec-Kit Workflow

This repo uses [spec-kit](https://github.com/github/spec-kit) skills. Specs live under
`specs/<N>-<short-name>/` (`spec.md`, `plan.md`, `tasks.md`, `research.md`, …). The
constitution is at `.specify/memory/constitution.md`. See `docs/architecture.md` and
`docs/roadmap.md` for the big picture.

<!-- SPECKIT START -->
**Active spec/plan:** `specs/7-principle7-plan-enforcement/` — `spec.md`, `plan.md` (Principle 7 Plan Enforcement: evidence-based Constitution Check row 7).
<!-- SPECKIT END -->

## Known mtgo/tg Gotchas (from prior sessions)

- `tg.Vector` is an empty struct that does NOT decode contents — avoid `UsersGetUsers`;
  use `telegram.Client.Me()` for getMe.
- `types.User.LanguageCode` field is actually `Language`.
- `BotsGetBotName`/`BotsSetBotName` don't exist — use `BotsSetBotInfo`/`BotsGetBotInfo`.
- `ChannelsEditAbout`/`ChannelsExportInvite` don't exist — use
  `MessagesEditChatAbout`/`MessagesExportChatInvite`.
- Forum-topic TL methods may not be generated — fall back to high-level mtgo methods.
- `MessagesGetFullChat` returns `ChatFullClass`; actual response is `*MessagesChatFull`
  (which implements `ChatFullClass`). Type-assert to `*MessagesChatFull` to get Chats/Users.
- `ChannelsGetFullChannel` same pattern — assert to `*MessagesChatFull`.
- `ChannelsGetParticipants` returns `*ChannelsChannelParticipants` (has Participants/Users/Chats).
- `ChannelsGetParticipant` returns `*ChannelsChannelParticipant` (wrapper with Participant/Users/Chats).
- `InputStickerSetShortName` field is `ShortName` (not `Name`).
- `BotsSetCustomVerification`: Enabled flag is bit 1 (not 0); Bot is bit 0.
- `ChatBannedRights` inversion: Bot API "can_send_messages" = true → TL `SendMessages` = false.
