# Spike Conventions

Patterns and stack choices established across spike sessions. New spikes follow these unless the question requires otherwise.

## Stack
- **Language:** Go (root `go.mod`, Go 1.27+), importing Xenobot-72's stdlib-only leaf packages (`xenobot/utils/claude`, `xenobot/utils/gemini`, `xenobot/utils/featherless`, `xenobot/utils/chatprompt`, `xenobot/utils/xenopersona`). Never import `xenobot/config` or `xenobot/db` in a spike because their `init()` hooks connect to PostgreSQL before `main()`.
- **Frontend UI:** Single embedded dark-theme `ui.html` served via `//go:embed ui.html` from Go's `net/http` `ServeMux` — zero npm/bundler/build tooling.
- **Dual execution mode:** Every spike runs as a local interactive web UI by default and supports `--cli` (headless batch stdout + `--out <path.json>`) plus `--mock` (in-process `httptest.Server` stubs for offline verification).

## Structure
- One directory per spike: `.planning/spikes/NNN-kebab-name/` containing `main.go`, `ui.html`, `main_test.go`, `README.md` (plus optional `corpus.go` / `fixtures.go`).
- Dot-prefixed `.planning/spikes/` is ignored by `go build ./...`, `go test ./...`, and `golangci-lint`, while still allowing `go run ./.planning/spikes/NNN-...` and `go test -race -v ./.planning/spikes/NNN-...`.
- Loopback-only sequential port assignments:
  - `001`: `127.0.0.1:8701`
  - `002`: `127.0.0.1:8702`
  - `003`: `127.0.0.1:8703`

## Patterns
- **Credential hygiene:** Read keys from standard Xenobot-72 env vars (`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `FEATHERLESS_API_KEY`, `CHAT_ANTHROPIC_MODEL`, `CHAT_GEMINI_MODEL`, `CHAT_FEATHERLESS_MODEL` / `FEATHERLESS_MODEL`) with optional in-memory browser overrides. Never log keys, never echo them in `/api/config`, never write them into forensic exports; prove non-leakage in `main_test.go`.
- **Forensic observability:** Every spike maintains a mutex-guarded `ForensicStore` with RFC3339Nano ISO timestamps, category tags, `GET /api/logs` (`?download=1` for browser export), `POST /api/logs/clear`, CLI `--out` JSON export, and aggregated summary tables.
- **Prompt caching layout:** Always keep `Static` byte-identical across groups and place non-empty per-chat data (group title, language, intersected allowed reaction palette) in `Dynamic`, matching both `claude.Converse` (`ephemeral` cache control) and `gemini.Generate` / `featherless.Converse`.
- **Emoji canonicalization:** Always normalize Telegram standard reaction emojis by stripping `U+FE0F` (`strings.ReplaceAll(strings.TrimSpace(raw), "\ufe0f", "")`) before checking against the 73-entry `supportedReactionEmoji` list and per-chat `AvailableReactions` allowlist.

## Tools & Libraries
- `xenobot/utils/claude` (`claude-haiku-5-5` default, `ConverseEffort = "low"`)
- `xenobot/utils/gemini` (`gemini-4-hydrogen` default, `DefaultThinkingLevel = "low"`)
- `xenobot/utils/featherless` (OpenAI-compatible `Converse` with `<think>` stripping; requires explicit `owner/model` HF ID)
- `xenobot/utils/chatprompt` & `xenobot/utils/xenopersona`
- Go stdlib `net/http`, `net/http/httptest`, `image/png`, `image/jpeg`, `embed`
