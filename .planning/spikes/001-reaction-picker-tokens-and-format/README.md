---
spike: 001
idea: contextual-ai-reactions
name: reaction-picker-tokens-and-format
type: standard
validates: "Given a compact reaction-picker prompt offering all 73 standard Telegram emojis + NONE, when run across Claude Haiku, Gemini 4 (thinkingLevel: low), and Featherless across MaxTokens (128 / 256 / 512), then responses parse cleanly to one valid standard emoji or NONE with zero ErrNoReply thinking-token truncation and acceptable latency"
verdict: PENDING
related: [002, 003]
tags: [ai, reactions, tokens, claude, gemini, featherless]
---

# Spike 001: Reaction Picker Tokens & Format

## What This Validates
Given a compact reaction-picker prompt offering all 73 standard Telegram emojis + `NONE`, when run across Claude Haiku (`claude-haiku-5-5`), Gemini 4 (`gemini-4-hydrogen`, `thinkingLevel: "low"`), and Featherless across `MaxTokens` (`128` / `256` / `512`), then responses parse cleanly to one valid standard emoji or `NONE` with zero `ErrNoReply` thinking-token truncation and acceptable latency.

## Research
Grounded directly in Xenobot-72's production stdlib-only provider leaf packages (`xenobot/utils/claude`, `xenobot/utils/gemini`, `xenobot/utils/featherless`) and `xenobot/modules/reactions.go`:

- **Thinking-token budget constraint:** `gemini-4-hydrogen` cannot turn thinking off (`DefaultThinkingLevel = "low"`, `xenobot/utils/gemini/client.go:33-35`). `ThoughtsTokens` count against `maxOutputTokens`; if thinking exhausts `MaxTokens`, `candidate.Content.Parts` contains only `thought: true` parts and `parseReply` returns `ErrNoReply: blank answer`. Similarly, open-weight reasoning models on Featherless (e.g., `Qwen/Qwen3-32B`) emit `<think>...</think>` inside `content`; if `max_completion_tokens` cuts off before `</think>`, `stripLeadingThink` returns `""` -> `featherless.ErrNoReply`.
- **Variation selector (`U+FE0F`) mismatch:** Telegram's `supportedReactionEmoji` (`xenobot/modules/reactions.go:32-40`) stores 73 standard emojis with `U+FE0F` stripped (`❤`, `❤‍🔥`, `✍`, `🕊`, `☃`, `🤷‍♂`, `🤷‍♀`). LLMs routinely emit the `U+FE0F` forms (`❤️`, `✍️`). Normalizing via `strings.ReplaceAll(strings.TrimSpace(raw), "\ufe0f", "")` prevents false rejections.
- **Prompt caching layout:** Both `claude.Converse` (`ephemeral` cache control on `Static` block) and `gemini.Generate` / `featherless.Converse` (implicit prefix caching) send `Static` first and `Dynamic` second. Keeping the persona & decision rules in `Static` (byte-identical across all groups) and placing the per-chat intersected `AvailableReactions` allowlist + format rule in `Dynamic` maximizes cache hits while honoring chat-specific reaction subsets.

### Output Format Comparison

| Approach | Wire Format | Pros | Cons | Status |
|---|---|---|---|---|
| `plain` | Bare emoji or `NONE` | Fewest output tokens (1-3 tokens); no JSON syntax overhead; trivial for small models | Occasional trailing punctuation or commentary on chatty models | Tested side-by-side |
| `json` | `{"emoji":"🔥"}` / `{"emoji":"NONE"}` | Structured contract; handles `null`/`""` unambiguously | Models sometimes wrap in ```` ```json ```` fences; adds ~8-12 output tokens | Tested side-by-side |

**Chosen approach:** Build and benchmark **both** `plain` and `json` side-by-side across `MaxTokens ∈ {128, 256, 512}` on all 3 providers using Xenobot-72's real `claude.Client`, `gemini.Client`, and `featherless.Client` structs.

## How to Run

### 1. Interactive Web UI (recommended)
```bash
export ANTHROPIC_API_KEY=...
export GEMINI_API_KEY=...
export FEATHERLESS_API_KEY=...
export CHAT_FEATHERLESS_MODEL="Qwen/Qwen3-32B"   # or your preferred Featherless model

go run .planning/spikes/001-reaction-picker-tokens-and-format
# Open http://127.0.0.1:8701
```
You can also leave env vars unset and paste keys into the local-only (`127.0.0.1`) UI session (stored in memory only, never persisted or logged).

### 2. Headless CLI Batch Run (with Forensic JSON export)
```bash
go run .planning/spikes/001-reaction-picker-tokens-and-format \
  --cli \
  --providers claude,gemini,featherless \
  --tokens 128,256,512 \
  --formats plain,json \
  --out .planning/spikes/001-reaction-picker-tokens-and-format/results-live.json
```

### 3. Offline Smoke Run (Mock Providers) & Automated Tests
```bash
go run .planning/spikes/001-reaction-picker-tokens-and-format --cli --mock
go test -race -v ./.planning/spikes/001-reaction-picker-tokens-and-format
```

## What to Expect
- **At `MaxTokens = 128`:** Watch `ErrNoReply Truncations` and `Avg Thought Tok` on Gemini 4 and reasoning Featherless models (Qwen3). Any non-zero truncation count disqualifies `128` for production.
- **At `MaxTokens = 256` vs `512`:** Verify 100% clean parses (`valid_emoji` / `valid_none`), zero `ErrNoReply` truncations, and measure P50/P95 latency delta.
- **Restricted 5-emoji chat probe:** Verify every provider selects exclusively from `👍 👎 🔥 ❤ 👏` (never emitting `🎉` or `🏆` outside the chat's `AvailableReactions`).
- **Format winner (`plain` vs `json`):** Compare clean-parse %, fence usage, and output tokens.

## Observability
Built-in forensic log layer (`ForensicStore`):
- Records ISO-8601 (`RFC3339Nano`) events tagged `session_start`, `run_start`, `parse_result`, `truncation_detected`, `single_probe`, `run_complete`.
- Export via browser **Export Forensic JSON** button, `GET /api/logs` (`?download=1`), or CLI `--out <path>`.
- Summarizes per `(provider, format, max_tokens)` bucket: clean-parse %, `ErrNoReply` truncation count, P50/P95/Max latency, average output & thoughts tokens.
- API keys are never written to events, responses, or exports (verified by `TestHTTPAPIAndNoSecretLeakInForensicLog`).

## Investigation Trail
1. **Verified production leaf clients:** Confirmed `claude`, `gemini`, `featherless`, and `chatprompt` have zero DB/config side effects and can be imported directly by spike binaries.
2. **Caught `U+FE0F` canonicalization gap:** Inspected `xenobot/modules/reactions.go:32-40,434-436` — 7 of the 73 standard emojis (`❤`, `❤‍🔥`, `✍`, `🕊`, `☃`, `🤷‍♂`, `🤷‍♀`) omit `U+FE0F`, while LLMs frequently emit `U+FE0F`. Built `NormaliseEmoji` + unit tests for all 7 forms.
3. **Caught `Dynamic` non-empty requirement:** Verified both `claude.Converse` and `gemini.Generate` emit a 2nd system block from `Dynamic`; placed per-chat intersected palette + format instruction in `Dynamic` so `Static` stays byte-identical for prompt caching.
4. **Offline harness verification:** Ran `go test -race -v` and `--cli --mock` to confirm parser edge cases (73 emojis, non-standard emoji rejection, multi-emoji rejection, restricted palette rejection, thinking-token truncation telemetry, and zero secret leakage).

## Results
- **Verdict:** `PENDING` (awaiting live run with user's Anthropic, Gemini, and Featherless API keys; harness + offline mock verification passing).
- **Offline verification evidence:** All unit + `httptest.Server` integration tests pass under `-race`; `--cli --mock` cleanly surfaces simulated `ErrNoReply` truncations at `MaxTokens=128` on Gemini/Featherless and 100% clean parses at `256`/`512`.
- **Next step:** Run with live keys via UI (`http://127.0.0.1:8701`) or `--cli --out results-live.json`, inspect the `128` vs `256` vs `512` and `plain` vs `json` table, and update this section + `MANIFEST.md` with the observed verdict.
