---
spike: 003
idea: contextual-ai-reactions
name: reaction-vision-media
type: standard
validates: "Given sample photos/memes and sticker thumbnails via the multimodal provider path, when evaluated on vision-capable providers (Claude / Gemini), then reactions match image context within acceptable token cost and degrade cleanly to caption/sticker-emoji on text-only Featherless models"
verdict: PARTIAL
related: [001, 002]
tags: [ai, reactions, vision, multimodal, stickers, featherless]
---

# Spike 003: Reaction Vision & Media

## What This Validates
Given sample photos, memes, and 128×128 sticker thumbnails passed through Xenobot-72's multimodal provider path (`xenobot/utils/claude`, `xenobot/utils/gemini`, `xenobot/utils/featherless` + `chat_media.go` MIME sniffing & labeling conventions), when evaluated on vision-capable providers (Claude / Gemini, and optional Featherless VL models), then reactions match the image context within acceptable token overhead and degrade cleanly to caption + `[sticker <emoji>]` on text-only Featherless models.

## Research
Grounded in `xenobot/modules/chat_media.go`, `xenobot/utils/claude/converse.go`, `xenobot/utils/gemini/request.go`, and `xenobot/utils/featherless/converse.go`:

- **MIME sniffing & size bounds:** `chat_media.go:248-254` never trusts file extensions or headers; it sniffs raw bytes with `http.DetectContentType` and accepts only `image/jpeg`, `image/png`, `image/webp` up to `5 MiB` (`claude.MaxImageBytes`). For animated/video stickers, `chatExtractStickerRef` automatically falls back to `Sticker.Thumbnail` (typically ~128×128 WebP/JPEG).
- **Provider wire shapes:**
  - **Claude:** `content` array with `{type: "text"}` followed by `{type: "image", source: {type: "base64", media_type, data}}`.
  - **Gemini:** `contents[0].parts` with `{text}` followed by `{inlineData: {mimeType, data}}`.
  - **Featherless:** `userContent` (`xenobot/utils/featherless/converse.go:107-122`) sends a plain string when `Images` is empty (maximum compatibility with text-only open-weight templates) and an OpenAI `image_url` data-URI array when `Images` is non-empty.
- **Text-only Featherless degradation strategies:**

| Strategy | Behavior on Featherless | Pros | Cons | Status |
|---|---|---|---|---|
| Explicit `text_fallback` | Never attach image bytes on text-only Featherless models; format `[sticker 😼]` / `[photo] <caption>` | Zero wasted 400 round-trips; works deterministically with every HF model | Requires knowing whether `CHAT_FEATHERLESS_MODEL` is vision-capable (or a toggle) | Tested side-by-side |
| `auto_degraded_from_400` | Attempt `image_url`; on HTTP 400/422, catch `*featherless.HTTPError` and retry text-only without tripping provider circuit breaker | Zero-config support for both VL (`Qwen/Qwen2.5-VL-72B-Instruct`) and text-only (`Qwen/Qwen3-32B`) models | Costs 1 extra failed request on text-only models unless capability is cached per model ID | Tested side-by-side |

**Chosen approach:** Benchmark both `featherless_vision` (with optional automatic HTTP 400/422 degradation) and explicit `featherless_text_fallback` alongside `claude` and `gemini`, and measure exact image token overhead (`input_tokens_with_image - input_tokens_text_only`) per fixture.

## How to Run

### 1. Interactive Web UI (recommended — preview fixtures + upload your own photos/memes/stickers!)
```bash
export ANTHROPIC_API_KEY=...
export GEMINI_API_KEY=...
export FEATHERLESS_API_KEY=...
export CHAT_FEATHERLESS_MODEL="Qwen/Qwen3-32B"   # test text-only degradation, or try a VL model

go run ./.planning/spikes/003-reaction-vision-media
# Open http://127.0.0.1:8703
```

### 2. Headless CLI Batch Run
```bash
go run ./.planning/spikes/003-reaction-vision-media \
  --cli \
  --targets claude,gemini,featherless_vision,featherless_text_fallback \
  --baseline=true \
  --auto-fallback=true \
  --out .planning/spikes/003-reaction-vision-media/results-live.json
```

### 3. Offline Mock Smoke Run & Automated Tests
```bash
go run ./.planning/spikes/003-reaction-vision-media --cli --mock
go test -race -v ./.planning/spikes/003-reaction-vision-media
```

## What to Expect
- **Captionless photo/meme (`photo_cat_portrait`, `meme_friday_deploy`):** Claude & Gemini see the visual content and return fitting reactions (`❤`/`👀` for the cat; `🔥`/`🫡`/`😱` for Friday deploy meme); text-only Featherless sees only `[photo]` and cleanly returns `NONE`.
- **Mundane receipt (`photo_mundane_invoice`):** Vision providers inspect the image, recognize a routine invoice, and still return `NONE`.
- **Stickers (`sticker_smug_cat`, `sticker_clown_roast`):** Vision providers see the 128×128 thumbnail; text-only Featherless uses `[sticker 😼]` / `[sticker 🤡]` + thread context to degrade gracefully.
- **Token overhead:** Compare `Avg Image Token Overhead` for 320×240–400×240 photos vs 128×128 sticker thumbnails across Claude and Gemini.
- **Upload Playground:** Drop real Telegram memes, photos, or `.webp` stickers from your machine into section 3 of the UI to feel real-world reaction quality.

## Observability
Built-in `ForensicStore`:
- ISO-8601 events tagged `session_start`, `run_start`, `media_eval`, `featherless_degraded`, `upload_eval`, `run_complete`.
- Export via browser **Export Forensic JSON**, `GET /api/logs?download=1`, or CLI `--out`.
- Summarizes per target: clean parse count, expectation match count, degraded/fallback count, average latency, average input tokens, and average image token overhead.

## Investigation Trail
1. **Verified `chat_media.go` constraints:** Matched byte-level MIME sniffing (`image/jpeg`, `image/png`, `image/webp`), `5 MiB` cap, and `[sticker <emoji>]` / `[photo]` labels.
2. **Built 6 zero-dependency visual fixtures + real upload endpoint:** Programmatic Go PNG/JPEG fixtures cover captionless cat photo, JPEG meme with OCR text, crash terminal screenshot + caption, mundane invoice (`NONE` test), and two 128×128 sticker thumbnails; `/api/upload` lets the user test arbitrary local JPEG/PNG/WebP files.
3. **Offline verification:** Passed `go test -race -v` verifying byte sniffing, image token overhead math, automatic HTTP 400→text-only degradation on Featherless, invalid-upload HTTP 400 rejection, and zero secret leakage.

## Results
- **Verdict:** `PARTIAL` — **Gemini 4 (`6/6`, 100%) empirically validated** and **Featherless (`inclusionAI/Ling-3.2-air`) vision + fallback behaviors measured** (`2026-11-26`); **Claude blocked on `HTTP 401 authentication_error: invalid x-api-key`** (0/6).
- **Live Media Summary (`MaxTokens = 384`, from `results-live.json`):**

| Target | Clean | Match | Errors | Degraded | Avg ms | Avg In Tok | Avg Image Overhead |
|---|---:|---:|---:|---:|---:|---:|---:|
| `gemini` (`gemini-4-hydrogen`) | **6/6** | **6/6** | 0 | 0 | 2342 | 1522 | **+1056.5** |
| `featherless_vision` (`Ling-3.2-air`) | 3/6 | 3/6 | 1 | 0 | 4614 | 553 | +61.3 |
| `featherless_text_fallback` (`Ling-3.2-air`) | 4/6 | 4/6 | 0 | 6 | 2369 | 510 | +0.0 |
| `claude` (`claude-haiku-5-5`) | 0/6 | 0/6 | 6 | 0 | 135 | — | — (`HTTP 401`) |

- **Key Findings:**
  1. **Gemini 4 Vision was 6/6 (`100%`)**: Cat photo → `❤`, Friday deploy meme → `🔥`, prod crash screenshot → `🫡`, mundane invoice → `NONE`, smug cat sticker → `👀`, clown sticker roast → `🤡` (`thoughts_tokens = 111–169`, consistent **+1,039 to +1,067 input token** image overhead, ~2.34 s latency).
  2. **Featherless (`inclusionAI/Ling-3.2-air`) findings:**
     - Model actually accepted `image_url` (no HTTP 400; only +54–73 input tokens) and got `photo_cat_portrait` (`❤`), `meme_friday_deploy` (`🔥`), and `photo_mundane_invoice` (`NONE`) right — **but** overflowed `MaxTokens = 384` with untagged prose reasoning on 2 fixtures (`photo_prod_crash`, `sticker_smug_cat`) and returned `featherless: no reply: no choices` after 12.9 s on `sticker_clown_roast`.
     - On `featherless_text_fallback` for `sticker_smug_cat`, it echoed `😼` (which is **not** in Telegram's 73 standard reaction emojis) — proving strict `supportedReactionEmoji` validation is mandatory before calling `SetMessageReaction`.
     - Text-only Featherless HTTP 400 auto-degradation was not triggered because `inclusionAI/Ling-3.2-air` accepted `image_url`; production should still keep both text-only fallback and silent `no choices` / parse-error shedding.
- **Remaining gap:** Re-run with `--targets claude` using a valid `ANTHROPIC_API_KEY` (and optionally test a known text-only Featherless model + real `.webp` uploads) to upgrade verdict to `VALIDATED`.
