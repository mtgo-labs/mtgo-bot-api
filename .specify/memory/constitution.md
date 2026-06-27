<!--
Sync Impact Report
- Version change: 1.0.0 → 1.1.0 (MINOR — added Principle 7)
- Modified principles: none (Governance wording updated: "Principles 1–6" → "Principles 1–7")
- Added sections: Principle 7 — Single Source of Truth (no duplicated logic)
- Removed sections: N/A
- Templates requiring updates: plan-template.md ✅ Constitution Check row 7 added / spec-template.md ✅ aligned / tasks-template.md ✅ aligned
- Follow-up TODOs: none
-->

# Constitution: mtgo-bot-api

**Project:** mtgo-bot-api
**Ratified:** 2026-06-13
**Last amended:** 2026-06-19
**Version:** 1.1.0

## Purpose

This constitution is the non-negotiable governing agreement for every feature,
plan, and task in the mtgo-bot-api repository. All specifications (`spec.md`),
implementation plans (`plan.md`), and task lists (`tasks.md`) MUST be consistent
with these principles. When a design conflicts with a principle, the principle
wins unless an explicit, recorded exception is approved.

## Governing Principles

### Principle 1 — Bot API Fidelity (Supreme)

mtgo-bot-api MUST be behaviorally indistinguishable from the official
telegram-bot-api server for every HTTP method it implements. This means:

- Method names and parameters are exactly those of the Bot API spec.
- JSON response envelopes are identical: `{"ok":…,"result":…,"error_code":…,"description":…,"parameters":…}`.
- Error codes (HTTP + Telegram) and human-readable descriptions match the
  official `Client.cpp` translations of MTProto errors.
- Bot API types (`User`, `Chat`, `Message`, `Update`, …) use the exact field
  names, types, and snake_case JSON tags from the spec.
- Booleans never use `omitempty`; presence semantics mirror the reference.

Rationale: A bot written for the official API must run unchanged against
mtgo-bot-api. Fidelity is the product; "almost compatible" is a bug.

### Principle 2 — mtgo via the raw `tg` TL layer

All Telegram interaction MUST go through the generated `tg` package
(`github.com/mtgo-labs/mtgo/tg`) using `*tg.RPCClient` and raw TL request
structs. `telegram.Client` is used ONLY to own the connection, session, and bot
authentication. The high-level `telegram.Client` methods and the `Bound*`
methods (`BoundSend`, `BoundAnswer`, …) MUST NOT be used for Bot API logic.

Rationale: Raw TL structs are the exact MTProto schema (layer 225). This gives
1:1 control mirroring how the official server invokes TDLib `td_api` methods,
and avoids abstraction mismatches between high-level helpers and Bot API semantics.

### Principle 3 — stdlib `net/http` only

The HTTP server and webhook delivery MUST use Go's standard library
`net/http`. No third-party HTTP frameworks, routers, or middleware libraries
(e.g. gin, echo, fiber, chi). Multipart parsing uses `mime/multipart`.

Rationale: Predictable behavior, zero framework lock-in, and parity with the
reference server's own minimal HTTP handling.

### Principle 4 — TQueue-style persistent storage

The update queue MUST be persisted (like tdlib's `TQueue` + binlog) so that
updates survive process restarts and `getUpdates` resumes from the correct
offset. Storage uses SQLite via `modernc.org/sqlite` (no CGO). The storage
interface mirrors `TQueue::StorageCallback` (`push`/`pop`/`replay`/`gc`).

Rationale: Durability of updates is a hard Bot API contract. No CGO keeps the
single-binary, easy-deployment story.

### Principle 5 — No CGO, pure Go

The entire codebase MUST compile and run without CGO. The only non-stdlib
dependency of significance is `modernc.org/sqlite` for storage and
`github.com/mtgo-labs/mtgo` for MTProto.

Rationale: Static, cross-compilable single-binary deployment.

### Principle 6 — Evidence before success

No task is "done" until it is verified by `go build ./...`, `go vet ./...`, and
`go test ./...` (where applicable). Behavioral parity claims for a method MUST
include a test or a documented manual check against the reference behavior.

Rationale: A 100%-fidelity goal is meaningless without verification gates.

### Principle 7 — Single Source of Truth (no duplicated logic)

No responsibility may have two implementations. Before adding code, the author
MUST inspect the existing codebase for equivalent logic — helpers, types,
interfaces, middleware, validation, error handling, config loading, and
request/response patterns — and reuse, extend, or refactor the existing one
rather than writing a parallel form. When one responsibility is spread across
several copies, they MUST be unified into a single shared helper, interface, or
package. Dead code and obsolete alternatives MUST be removed. Naming, error
handling, and folder/package structure MUST stay consistent across the project.

Rationale: Divergent copies of the same logic drift, producing subtle parity
bugs that directly undermine Principle 1. One authoritative implementation is
auditable and testable, and mirrors the reference server's own single-path
design — so that "avoid duplication" is enforced as a design gate, not a habit.

## Governance

- **Amendment:** Any change to these principles requires a version bump
  (MAJOR for removals/redefinitions, MINOR for additions, PATCH for wording)
  and must be reflected across `spec-template.md`, `plan-template.md`, and
  `tasks-template.md`.
- **Compliance review:** Every `plan.md` includes a "Constitution Check" that
  evaluates the design against Principles 1–7. Unjustified violations are an
  ERROR and block the task-generation phase.
- **Exceptions:** A principle may be waived for a specific feature only by
  recording a dated, rationale-backed note in `research.md`.
