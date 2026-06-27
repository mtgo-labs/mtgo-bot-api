# Implementation Plan: [FEATURE NAME]

## Technical Context

**Language(s):**      [e.g. Go 1.26]
**Primary framework:** [e.g. net/http stdlib]
**Storage:**           [e.g. modernc.org/sqlite]
**Key dependencies:**  [e.g. github.com/mtgo-labs/mtgo/tg]
**Project structure:** [key internal/ package map]
**Deployment:**        [e.g. single static binary]

**NEEDS CLARIFICATION:**
- [unknown-1: ...]
- [unknown-2: ...]

## Constitution Check

Reference: `.specify/memory/constitution.md`

| Principle | Status | Notes |
|-----------|--------|-------|
| 1. Bot API Fidelity | ✅ / ⚠ / ❌ | [how design upholds it] |
| 2. mtgo via raw `tg` TL layer | ✅ / ⚠ / ❌ | [evidence] |
| 3. stdlib net/http only | ✅ / ⚠ / ❌ | [evidence] |
| 4. TQueue-style persistent storage | ✅ / ⚠ / ❌ | [evidence] |
| 5. No CGO, pure Go | ✅ / ⚠ / ❌ | [evidence] |
| 6. Evidence before success | ✅ / ⚠ / ❌ | [verification plan] |
| 7. Single Source of Truth | ✅ / ⚠ / ❌ | [see **Principle 7 Evidence** below] |

ERROR on unjustified violations. Record exceptions in research.md.

**Principle 7 — Single Source of Truth** is gated on *evidence*, not a status
mark. The acceptance bar is defined once in `AGENTS.md` §Code Quality Rules
(which operationalizes constitution Principle 7). To pass, the
`### Principle 7 Evidence` sub-section below must state, **per piece of new
logic**, either (a) the specific existing implementation(s) reused or extended
(file path / symbol), or (b) a duplication-register entry in `research.md` with
a justification and a named unification task.

**Block condition:** a Principle 7 entry that is a bare status mark, or whose
evidence does not meet the bar, **blocks the plan from the task-generation
phase**.

### Principle 7 Evidence

<!-- Per piece of new/changed logic: cite the existing implementation reused
     (file path / symbol), or reference a research.md duplication-register entry
     (justification + unification task). A bare ✅ with nothing here fails the check. -->

- [logic 1]: ...

## Phase 0: Outline & Research

See `research.md` for resolved NEEDS CLARIFICATION items, with Decision /
Rationale / Alternatives considered for each.

## Phase 1: Design & Contracts

- Data model: see `data-model.md`
- API/HTTP contracts: see `contracts/`
- Quickstart / test scenarios: see `quickstart.md`

### Architecture (mirrors telegram-bot-api)

```
cmd/mtgo-bot-api/      Entry point: CLI flags, server bootstrap
internal/server/       Raw net/http server + multipart/query parsing
internal/manager/      ClientManager: multi-bot lifecycle, token routing, flood control
internal/client/       Per-bot Client: all Bot API method handlers (raw tg.RPCClient)
internal/tqueue/       TQueue: monotonic update queue + storage callback
internal/storage/      SQLite persistence (TQueue, webhooks, peers)
internal/webhook/      Outgoing webhook delivery + setWebhook/deleteWebhook/getWebhookInfo
internal/types/        Bot API types matching the spec
internal/convert/      mtgo tg.TLObject ↔ Bot API JSON types
internal/fileid/       file_id encode/decode
internal/response/     JSON response envelope
internal/stats/        Per-bot and global statistics
```

### Data flow

```
HTTP request ─▶ server ─▶ manager (token route) ─▶ client
                                                      │
                              raw tg.RPCClient ◀──────┤ (generated TL request structs)
                                      │
                          MTProto result ─▶ convert ─▶ response envelope ─▶ HTTP
```

Updates flow in the opposite direction: MTProto updates → client → tqueue (persisted)
→ getUpdates (long poll) or webhook delivery.

## Open Questions

- [any unresolved items after research]
