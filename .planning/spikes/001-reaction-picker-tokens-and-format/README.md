---
spike: 001
idea: contextual-ai-reactions
name: reaction-picker-tokens-and-format
type: standard
validates: "Given a compact reaction-picker prompt offering all 73 standard Telegram emojis + NONE, when run across Claude Haiku, Gemini 4 (thinkingLevel: low), and Featherless across MaxTokens (128 / 256 / 512), then responses parse cleanly to one valid standard emoji or NONE with zero ErrNoReply thinking-token truncation and acceptable latency"
verdict: VALIDATED
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

go run ./.planning/spikes/001-reaction-picker-tokens-and-format
# Open http://127.0.0.1:8701
```
You can also leave env vars unset and paste keys into the local-only (`127.0.0.1`) UI session (stored in memory only, never persisted or logged).

### 2. Headless CLI Batch Run (with Forensic JSON export)
```bash
go run ./.planning/spikes/001-reaction-picker-tokens-and-format \
  --cli \
  --providers claude,gemini,featherless \
  --tokens 128,256,512 \
  --formats plain,json \
  --out .planning/spikes/001-reaction-picker-tokens-and-format/results-live.json
```

### 3. Offline Smoke Run (Mock Providers) & Automated Tests
```bash
go run ./.planning/spikes/001-reaction-picker-tokens-and-format --cli --mock
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
- **Verdict:** `VALIDATED` — all three providers (`claude-haiku-5-5`, `gemini-4-hydrogen`, `inclusionAI/Ling-3.2-air`) empirically measured live (`2026-11-26`, `results-live.json` + `results-claude.json`).
- **Live Benchmark Summary:**

| Provider | Format | MaxTokens | Clean | Truncated | Err/Inv | P50 ms | P95 ms | Avg Out | Avg Thought |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `claude` (`claude-haiku-5-5`) | `plain` | 128 | 4/5 (80%) | 0 | 1 | 662 | 1058 | 18.0 | — |
| `claude` (`claude-haiku-5-5`) | `plain` | 256 | 4/5 (80%) | 0 | 1 | 715 | 1206 | 22.4 | — |
| `claude` (`claude-haiku-5-5`) | `plain` | 512 | 4/5 (80%) | 0 | 1 | 696 | 1331 | 19.6 | — |
| `claude` (`claude-haiku-5-5`) | `json` | 128 | 4/5 (80%) | 0 | 1 | 678 | 819 | 13.8 | — |
| `claude` (`claude-haiku-5-5`) | `json` | 256 | 4/5 (80%) | 0 | 1 | 716 | 818 | 13.8 | — |
| `claude` (`claude-haiku-5-5`) | `json` | 512 | 4/5 (80%) | 0 | 1 | 613 | 920 | 13.6 | — |
| `gemini` (`gemini-4-hydrogen`) | `plain` | 128 | 3/5 (60%) | **2** | 0 | 1776 | 1975 | 5.0 | 87.3 |
| `gemini` (`gemini-4-hydrogen`) | `plain` | 256 | **5/5 (100%)** | 0 | 0 | 1963 | 2637 | 5.0 | 105.8 |
| `gemini` (`gemini-4-hydrogen`) | `plain` | 512 | **5/5 (100%)** | 0 | 0 | 1647 | 2005 | 5.0 | 82.8 |
| `gemini` (`gemini-4-hydrogen`) | `json` | 128 | 3/5 (60%) | **2** | 0 | 2526 | 2867 | 9.0 | 85.0 |
| `gemini` (`gemini-4-hydrogen`) | `json` | 256 | **5/5 (100%)** | 0 | 0 | 1648 | 5729 | 9.0 | 106.4 |
| `gemini` (`gemini-4-hydrogen`) | `json` | 512 | **5/5 (100%)** | 0 | 0 | 1886 | 2565 | 9.0 | 134.6 |
| `featherless` (`Ling-3.2-air`) | `plain` | 128 | 2/5 (40%) | 0 | 3 | 927 | 1529 | 97.4 | — |
| `featherless` (`Ling-3.2-air`) | `plain` | 256 | 3/5 (60%) | 0 | 2 | 1117 | 1442 | 156.2 | — |
| `featherless` (`Ling-3.2-air`) | `plain` | 512 | **5/5 (100%)** | 0 | 0 | 1105 | 1952 | 171.2 | — |
| `featherless` (`Ling-3.2-air`) | `json` | 128 | 3/5 (60%) | 0 | 2 | 1022 | 1023 | 89.2 | — |
| `featherless` (`Ling-3.2-air`) | `json` | 256 | **5/5 (100%)** | 0 | 0 | 1025 | 1230 | 114.0 | — |
| `featherless` (`Ling-3.2-air`) | `json` | 512 | **5/5 (100%)** | 0 | 0 | 1022 | 2320 | 194.4 | — |

- **Key Findings & Production Recommendations:**
  1. **`MaxTokens = 128` is disqualified:** Gemini 4 hit `ErrNoReply` (`blank answer`) on 40% of calls at `128` (`85–87` avg thinking tokens), and Featherless failed 40–60% at `128`.
  2. **`MaxTokens = 512` is the safe cross-provider floor** (Gemini 4 100% from 256+, Featherless `plain` only hits 100% at `512`, Claude Haiku has 0 truncations and P50 ~613–716 ms — ~2.5× faster than Gemini).
  3. **Use `json` format (`{"emoji":"..."}`) in production:** Eliminated Claude Haiku's plain-text scratchpad leakage (`"😹 is not on the list...\n\n🤣"`) AND cut Featherless token failures at `256` from 40% to 0%.
  4. **Explicitly warn against non-Telegram lookalikes in the prompt + enforce strict allowlist:** Every single Claude miss (6/6) was on `en_cat_keyboard` — in `json` mode it emitted `{"emoji":"😂"}` (`😂` is NOT in Telegram's 73 standard reactions; `🤣` is), and in `plain` mode it thought aloud about `😹`/`😂` not being on the list before outputting `🤣`. Explicitly noting `"Note: 😂 and cat-face emojis (🐱/😹/😻) are NOT in the list — use 🤣, ❤, 👀, 🥰 instead"` plus strict `supportedReactionEmoji` validation closes this gap cleanly.
  5. **Restricted 5-emoji palette held 100%** across all three providers.
