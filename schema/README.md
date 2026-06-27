# Bot API Schema

Machine-readable description of the Telegram Bot API, maintained as part of
**mtgo-bot-api**. It is the single source of truth for which methods, types,
parameters and return types the Bot API exposes, and it is wired into our build
tooling to measure implementation coverage.

The data is **scraped from the official docs**
([core.telegram.org/bots/api](https://core.telegram.org/bots/api)) and checked
against our Go implementation — inspired by
[`PaulSonOfLars/telegram-bot-api-spec`](https://github.com/PaulSonOfLars/telegram-bot-api-spec),
but designed specifically for this project.

---

## Why this exists

| Goal | How the schema helps |
|------|----------------------|
| **Implementation coverage** | `validate` compares every schema method against the handlers registered in `internal/client` and reports what's missing. |
| **Gap detection** | Surfaced missing methods, methods with incomplete parameters, and stubbed/partial methods. |
| **SDK / client generation** | `methods.json` + `types.json` + `botapi.schema.json` give external developers a complete, stable contract to generate Bot API clients in any language. |
| **Codegen for this server** | The schema can drive generated request/response structs and HTTP handlers. |
| **Contributor orientation** | `generate` produces a grouped method index and a coverage report so contributors see exactly what is done and what remains. |

---

## Layout

```
schema/
  README.md            this file
  botapi.schema.json   JSON Schema validating the data files below       (hand-maintained)
  methods.json         every method: params, types, return type, status  (scrape-generated)
  types.json           every object type: fields, types, descriptions    (scrape-generated)
  status.json          hand-curated implementation status overrides      (hand-maintained)
  COVERAGE.md          coverage report                                    (generated)
  METHODS.md           grouped method index                               (generated)
  schema.go            Go model + loader (package `schema`)
  regen.sh             one-shot pipeline: scrape -> validate -> generate
  cmd/
    scrape/            fetch + parse official docs -> regenerate JSON
    validate/          cross-check schema vs the live implementation
    generate/          emit COVERAGE.md + METHODS.md
```

### Which files are generated?

| File | Source of truth |
|------|-----------------|
| `methods.json`, `types.json` | **Scraped** from `core.telegram.org/bots/api`. Carry a `"_generated": true` marker. **Do not hand-edit** — change the scraper or run `regen.sh`. |
| `COVERAGE.md`, `METHODS.md` | Produced by `generate`. |
| `botapi.schema.json`, `status.json` | **Hand-maintained.** The scraper never overwrites them. |
| `README.md`, `schema.go` | Source files. |

Generated files are committed so the repo self-documents its current coverage.
Regenerate them when rebasing onto a newer Bot API release.

---

## Commands

All commands run from the repository root. The recommended one-shot entry point:

```bash
./schema/regen.sh        # scrape + validate + generate
```

### Scrape — refresh the schema from the official docs

```bash
go run ./schema/cmd/scrape
```

Fetches `https://core.telegram.org/bots/api`, parses the HTML, and overwrites
`methods.json` and `types.json`. Categories are **derived from the docs' own
`<h3>` sections** — no hand-curated category source. The only thing carried over
from the previous `methods.json` is **non-official extension methods** (e.g.
`close`, `logout`) that aren't part of the official reference; `status.json` is
never touched.

Options:

```bash
go run ./schema/cmd/scrape -in saved.html      # parse a saved page instead of fetching
go run ./schema/cmd/scrape -url https://.../api # alternate source URL
go run ./schema/cmd/scrape -out ./schema        # alternate output directory
```

### Validate — schema vs implementation parity

```bash
go run ./schema/cmd/validate
```

Loads the schema and introspects the methods registered in
`internal/client`, then reports:

- **missing official methods** — in the schema but not registered (a real parity gap; exit code **1**),
- **untracked methods** — registered but not in the schema (e.g. deprecated aliases, special methods),
- **extensions** — non-official methods (mtgo additions),
- **incomplete parameter sets** — methods whose parameters aren't fully authored,
- **stubs** — entries flagged `stub`/`partial`/`unsupported` in `status.json`.

```bash
go run ./schema/cmd/validate -json                 # machine-readable output
go run ./schema/cmd/validate -fail-on-incomplete   # also fail on param gaps
```

### Generate — human-readable reports

```bash
go run ./schema/cmd/generate
```

Writes `schema/COVERAGE.md` (parity summary) and `schema/METHODS.md` (methods
grouped by category, each annotated `+` implemented / `~` extension / `-`
missing).

---

## Data model

`methods.json` and `types.json` conform to [`botapi.schema.json`](./botapi.schema.json)
(JSON Schema draft 2020-12). The Go representation lives in [`schema.go`](./schema.go).

A method entry:

```json
{
  "name": "sendmessage",
  "title": "sendMessage",
  "category": "Messaging",
  "description": "Use this method to send text messages. ...",
  "returns": "Message",
  "parameters": [
    { "name": "chat_id", "type": "Integer or String", "required": true, "description": "..." },
    { "name": "text", "type": "String", "required": true, "description": "..." }
  ],
  "params_complete": true,
  "official": true
}
```

`status.json` overlays nuance the introspection can't infer:

```json
{
  "status": {
    "kickchatmember": { "state": "implemented", "note": "Deprecated alias of banChatMember." }
  }
}
```

Allowed `state` values: `implemented`, `stub`, `partial`, `unsupported`, `n/a`.

---

## Updating the schema

1. **After a Bot API release** — run `./schema/regen.sh` (or
   `go run ./schema/cmd/scrape` then `validate`) to pull the new methods/types
   and see what our server is now missing. Categories update automatically from
   the docs.
2. **When implementing a new method** — run `validate`; the method should move
   out of *missing*. Regenerate the reports with `generate`.
3. **For mtgo-only extensions** — add them to `methods.json` with
   `"official": false`; the scraper preserves such entries across runs (it only
   re-derives official entries from the docs). For implementation-status notes
   on any method, edit `status.json`.

---

## For external developers (SDK generation)

- Treat `methods.json` + `types.json` as the input contract.
- Validate your generator's input against `botapi.schema.json`.
- The `name` field is the lowercased wire name used in HTTP requests
  (`POST /bot<token>/sendMessage` is case-insensitive, but this is the canonical
  form). The `title` field is the conventional camelCase identifier.
- Type strings use `Integer`, `Float`, `String`, `Boolean`, `InputFile`, an
  object type name (e.g. `User`), or `Type[]` for arrays. Union inputs use
  `A or B` / `A|B`.
- The schema is versioned via the top-level `api_version` field
  (e.g. `"10.1"`). Pin to a version before depending on it in production.

---

## Notes

- The scraper does a best-effort extraction of **return types** from method
  descriptions (≥99% coverage). A handful of methods whose docs phrase the
  return type unusually may have an empty `returns` field; fill those manually
  if exactness matters for your generator.
- `internal/client` is imported by `validate`/`generate` to introspect the real
  method registry; these commands therefore build the whole server and require
  the sibling `../mtgo` repo (see the root `AGENTS.md`).
