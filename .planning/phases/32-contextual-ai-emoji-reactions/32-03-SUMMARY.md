---
phase: 32-contextual-ai-emoji-reactions
plan: 03
subsystem: i18n
tags: [locales, i18n, settings-ui, presence, ai-reactions, validation]
status: complete

requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: Presence settings page locale block across all 8 locales and settings_locale_test.go validation harness
provides:
  - 6 Presence AI reaction UI keys across all 8 supported locales (en, uk, es, fr, hi, id, pt, ru)
  - Strict ceiling limits in settingsLocaleLimits, plain-text alert registration in settingsAlertKeys for settings_pr_ai_need_owner, and 3-wide segmented button length guard (<= 16 runes with "✓ " prefix) in TestSettingsLabelLengths
affects:
  - 32-05 (Presence settings page pr rendering the [Off | Low | Medium] segmented control, status line, Owner-lock note, and plain-text refusal alert)

tech-stack:
  added: []
  patterns:
    - 8-locale parity with terminology matched to each locale's existing settings_page_chat Conversations term
    - Plain-text <= 200-rune Telegram callback alert enforcement via settingsAlertKeys
    - 3-wide mobile segmented button length guard ("✓ " + label <= 16 runes) in TestSettingsLabelLengths

key-files:
  created: []
  modified:
    - locales/en.yml
    - locales/uk.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - xenobot/i18n/settings_locale_test.go

key-decisions:
  - "Matched each locale's existing settings_page_chat term for Conversations (Conversations, Розмови, Conversaciones, बातचीत, Percakapan, Conversas, Разговоры) and settings_state_off_plain Off label across all 8 locales."
  - "Used idiomatic frequency adverbs in Ukrainian (Рідко / Помірно) and Russian (Редко / Умеренно) so settings_pr_ai_line renders naturally with plural reactions while staying well under the 12-rune button budget."

patterns-established:
  - "Presence AI reaction segmented buttons are guarded in TestSettingsLabelLengths with \"✓ \" prefix <= 16 code points across all 8 locales."

requirements-completed:
  - PRES-04
  - REACT-01

coverage:
  - id: D1
    description: "All 8 locale files (en, uk, es, fr, hi, id, pt, ru) define the 6 Presence AI reaction settings keys with 100% parity and locale-style.md compliance."
    requirement: "PRES-04"
    verification:
      - kind: unit
        ref: "make check-translations"
        status: pass
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestSettingsLocaleKeys,TestLocaleStyle"
        status: pass
    human_judgment: false
  - id: D2
    description: "settings_pr_ai_need_owner is registered in settingsAlertKeys (plain text, zero HTML tags, <= 200 runes) and all 3 segmented mode buttons stay <= 16 code points with \"✓ \" prefix across all 8 locales."
    requirement: "REACT-01"
    verification:
      - kind: unit
        ref: "go test -tags testtools -race -count=1 ./xenobot/i18n/...#TestSettingsLocaleKeys,TestSettingsLabelLengths"
        status: pass
    human_judgment: false

duration: 3 min
completed: 2026-11-27

commits: 3
plan_head_before: fad5848d7a659295c6d9d3a36e2ed30fd9fc418b
plan_head_after: 837beea1da27128974bf58c406f917bea741cf92

actuals:
  tokens: 2169
  tasks: 3
  commits: 3
---

# Phase 32 Plan 03: Presence AI Reaction Locales & Test Ceilings Summary

**8-locale Presence Contextual AI Reactions UI strings (`settings_pr_ai_line`, `settings_pr_ai_locked_note`, `settings_pr_ai_need_owner`, `settings_btn_pr_ai_{off,low,medium}`) validated by `settingsLocaleLimits`, `settingsAlertKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, and `make check-translations`**

## Performance

- **Duration:** 3 min (162 s)
- **Started:** 2026-11-26T23:50:19Z
- **Completed:** 2026-11-26T23:53:01Z
- **Tasks:** 3
- **Files modified:** 9 (0 created, 9 modified)

## Accomplishments

- Added all 6 Presence AI reaction keys (`settings_pr_ai_line`, `settings_pr_ai_locked_note`, `settings_pr_ai_need_owner`, `settings_btn_pr_ai_off`, `settings_btn_pr_ai_low`, `settings_btn_pr_ai_medium`) across all 8 supported locale files (`en`, `uk`, `es`, `fr`, `hi`, `id`, `pt`, `ru`), aligned with each locale's existing `settings_page_chat` Conversations terminology and `locale-style.md` conventions.
- Registered all 6 keys in `settingsLocaleLimits` in `xenobot/i18n/settings_locale_test.go` with tight `{enMax, otherMax}` bounds and registered `settings_pr_ai_need_owner` in `settingsAlertKeys` to enforce plain-text, tag-free `<= 200`-rune Telegram alerts across all 8 locales (mitigating T-32-06).
- Added a 3-wide segmented button length guard in `TestSettingsLabelLengths` verifying `"✓ " + label <= 16` code points across all 8 locales so the `[Off | Low | Medium]` row never clips on narrow mobile screens.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Presence AI reaction keys to en, uk, es, and fr locales** - `721c9233` (feat)
2. **Task 2: Add matching Presence AI reaction keys to hi, id, pt, and ru locales** - `094eaacd` (feat)
3. **Task 3: Register Presence AI reaction limits, alert check, and segmented button caps in settings_locale_test.go** - `837beea1` (test)

## Files Created/Modified

- `locales/en.yml` - English Presence AI reaction status line, Owner-lock note, plain-text refusal alert, and Off/Low/Medium labels.
- `locales/uk.yml` - Ukrainian Presence AI reaction strings following `locale-style.md` (`«Розмови»`, `власник групи`, `ШІ-реакції`, `Вимк.`/`Рідко`/`Помірно`).
- `locales/es.yml` - Spanish Presence AI reaction strings (`Conversaciones`, `dueño del grupo`, `Apagado`/`Bajo`/`Medio`).
- `locales/fr.yml` - French Presence AI reaction strings (`Conversations`, `propriétaire du groupe`, `Off`/`Faible`/`Moyen`).
- `locales/hi.yml` - Hindi Presence AI reaction strings (`बातचीत`, `ग्रुप के मालिक`, `बंद`/`कम`/`मध्यम`).
- `locales/id.yml` - Indonesian Presence AI reaction strings (`Percakapan`, `pemilik grup`, `Mati`/`Rendah`/`Sedang`).
- `locales/pt.yml` - Portuguese Presence AI reaction strings (`Conversas`, `dono do grupo`, `Off`/`Baixo`/`Médio`).
- `locales/ru.yml` - Russian Presence AI reaction strings (`«Разговоры»`, `владелец группы`, `Выкл`/`Редко`/`Умеренно`).
- `xenobot/i18n/settings_locale_test.go` - Added 6 `settingsLocaleLimits` rows, `settings_pr_ai_need_owner` in `settingsAlertKeys`, and `"✓ "` `<= 16`-rune segmented button check in `TestSettingsLabelLengths`.

## Decisions Made

- Matched each locale's existing `settings_page_chat` term (`Conversations`, `Розмови`, `Conversaciones`, `बातचीत`, `Percakapan`, `Conversas`, `Разговоры`) and `settings_state_off_plain` label so UI wording stays consistent across Conversations and Presence pages.
- Used idiomatic frequency adverbs in Ukrainian (`Рідко` / `Помірно`) and Russian (`Редко` / `Умеренно`) so `settings_pr_ai_line` reads grammatically with plural reactions while keeping segmented labels well within the 12-rune cap.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Verification

- `make check-translations` — PASS (1069 translation keys, all 8 locales complete).
- `go test -tags testtools -race -count=1 ./xenobot/i18n/...` — PASS (`ok github.com/uasneppy/Xenobot-72/xenobot/i18n 36.271s`, covering `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestLocaleStyle`, and all locale parity guards).

## Next Phase Readiness

- All 6 Presence AI reaction UI keys and locale safety checks are in place for `settings_presence.go` (Plan `32-05`) to render the `[Off | Low | Medium]` control and Owner-lock notice/alert across all 8 locales.
- No stubs, no new dependencies, and no threat flags introduced.

## Self-Check: PASSED
