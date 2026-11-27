---
phase: 32-contextual-ai-emoji-reactions
plan: 06
subsystem: settings
tags: [settings-presence, owner-admin-gate, ui, locales, agents-md, verification]
status: complete

requires:
  - phase: 32-contextual-ai-emoji-reactions
    plan: 01
    provides: presence.AIReactModeOff/Low/Medium, presence.ValidAIReactMode, chat_presence.ai_react_mode
  - phase: 32-contextual-ai-emoji-reactions
    plan: 03
    provides: 8-locale settings_pr_ai_line, settings_pr_ai_locked_note, settings_pr_ai_need_owner, settings_btn_pr_ai_{off,low,medium}
  - phase: 32-contextual-ai-emoji-reactions
    plan: 04
    provides: applyPresenceAIReactMode writer with skipLocal cache invalidation
  - phase: 32-contextual-ai-emoji-reactions
    plan: 05
    provides: chatPresenceQueue worker lane, group 13 chatReactWatch handoff, and full contextual AI reactions runtime
provides:
  - Private Settings page pr 5-row layout with segmented [Off | Low | Medium] Contextual AI Reactions selector (t=ai, v=o/l/m)
  - Owner×Admin double-gating on page pr via fresh chatconv.GetFresh (settingsPresenceChatRead), fail-closed DB error handling, plain-text settings_pr_ai_need_owner alert when locked, unconditional Off disable, and accurate Main state counting
  - Expanded 8-locale layout, 64-byte callback, authorization, fail-closed, and forged-input tests in settings_presence_test.go
  - Updated AGENTS.md documenting all Phase 32 operational, worker-lane, Redis, Featherless vision fallback, and Settings page pr invariants
affects:
  - 33 (Smarter Static Reactions, Highlights Archive and Backup Parity extending page pr / chat_presence)

tech-stack:
  added: []
  patterns:
    - 3-button segmented [Off | Low | Medium] selector row on page pr matching settings_chat.go's segmented row convention
    - Fresh chatconv.GetFresh double-gating (settingsPresenceConvUnlocked) with zero DB reads when !chatAnyProviderConfigured(), fail-closed cannotOpen on DB read error when enabling, and unconditional Off disable
    - Accurate Main state counting (AI reactions counted in settings_state_presence_on only when AIReactMode != "off" and Owner Conversations is unlocked)

key-files:
  created: []
  modified:
    - xenobot/modules/settings_presence.go
    - xenobot/modules/settings_presence_test.go
    - AGENTS.md

key-decisions:
  - "Extracted settingsPresenceApplyStored behind settingsPresenceSetStored so settingsPresenceSetAIReact enforces group/Staff gates once before the chatconv.GetFresh unlock check and write without a duplicate Staff Group DB lookup."
  - "Kept settingsPresenceConvUnlockedForView degrading transient chatconv read errors to false (showing the Owner-lock note) during page rendering, while settingsPresenceSetAIReact fails closed with p.cannotOpen() and zero DB writes when attempting to enable Low or Medium."
  - "Short-circuited settingsPresenceConvUnlocked on !chatAnyProviderConfigured() so groups on servers without AI providers incur zero chat_conversations DB reads on page pr or Main."

patterns-established:
  - "Admin AI toggles on Settings page pr require fresh chatconv.GetFresh verification before enabling while always allowing disable to Off and leaving zero-AI toggles independent."

requirements-completed:
  - PRES-04
  - REACT-01

coverage:
  - deliverable: "Settings page pr 5-row [Off | Low | Medium] AI reactions selector with Owner×Admin double-gating and accurate Main state"
    requirement: "PRES-04"
    human_judgment: false
    verification:
      - kind: test
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows/ai_reactions_owner_admin_double_gate_and_forged_inputs"
        status: pass
      - kind: test
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceCallbackBytes"
        status: pass
      - kind: test
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresencePageFitsEveryLocale"
        status: pass
  - deliverable: "Admin Off/Low/Medium Contextual AI Reactions control and complete Phase 32 verification gate"
    requirement: "REACT-01"
    human_judgment: false
    verification:
      - kind: command
        ref: "CGO_ENABLED=0 go build ./... && make check-translations && make check-docs && GOTOOLCHAIN=go1.26.0 make lint"
        status: pass
      - kind: test
        ref: "go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/utils/chatprompt ./xenobot/i18n ./xenobot/db/backup ./xenobot/modules"
        status: pass

duration: 6 min
completed: 2026-11-27

commits: 3
plan_head_before: 28da7eab7f327d7d35da4085e9c10e93e5ddf167
plan_head_after: 0e13fc3acc8098e3a79364ce9cdc047cce93625d

actuals:
  tokens: 8879
  tasks: 3
  commits: 3
---

# Phase 32 Plan 06: Presence Page AI Reactions Control, Owner×Admin Gate & AGENTS.md Summary

**5-row private Settings page `pr` `[Off | Low | Medium]` Contextual AI Reactions selector with fresh `chatconv.GetFresh` Owner×Admin double-gating, 8-locale/64-byte/authorization `-race` test suite, `AGENTS.md` Phase 32 invariants, and green end-to-end build/translations/docs/lint/test verification gate**

## Performance

- **Duration:** 6 min (365 s)
- **Started:** 2026-11-27T00:10:04Z
- **Completed:** 2026-11-27T00:16:09Z
- **Tasks:** 3
- **Files modified:** 3 (0 created, 3 modified)

## Accomplishments

- Added `settingsPresenceControlAIReact` (`t=ai`, `v=o/l/m`) under `settingsRightChangeInfo` in `xenobot/modules/settings_presence.go` with a 3-button segmented `[Off | Low | Medium]` row (row 4 before `navRow`, 5 rows total `<= 8`), `settings_pr_ai_line` status line, and explicit `settings_pr_ai_locked_note` whenever Owner Conversations is not enabled/configured.
- Implemented fresh Owner×Admin double-gating via `settingsPresenceChatRead = chatconv.GetFresh` (`settingsPresenceConvUnlocked`): enabling `low` or `medium` refuses writes with plain-text alert `settings_pr_ai_need_owner` when Owner Conversations is off (and fails closed with `p.cannotOpen()` on DB error), disabling to `off` always succeeds even after Conversations is turned off, zero-AI Consensus and Retract controls stay independent, and `settingsPresenceMainState` counts AI reactions only when both `AIReactMode != "off"` and Owner Conversations is unlocked.
- Expanded `xenobot/modules/settings_presence_test.go` to cover 5-row callback byte limits (`1..64` bytes), all 8 locales (`uk, en, es, fr, hi, id, pt, ru`) including locked-note worst cases, group-menu and Staff-Group forged `t=ai` refusals, and `ai_reactions_owner_admin_double_gate_and_forged_inputs` covering every Owner×Admin, fail-closed, Main state, disable-when-locked, and forged-input path.
- Documented all Phase 32 invariants in `AGENTS.md` (`StopChat` `chatPresenceQueue` lane with zero new shutdown drains, group `13` unaddressed handoff to `chatReactWatch`, `chat_presence.ai_react_mode` schema & rolling-deploy cache normalization, operational vs `xenobot:cache:` Redis keys including Featherless `text_only` fallback, `chatprompt/reaction.go` stdlib leaf, non-tripping shedding, and Settings page `pr` Owner×Admin double-gating) and verified the full Phase 32 gate cleanly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add [Off | Low | Medium] AI reactions control and Owner×Admin unlock gate to settings_presence.go** - `055dbe2f` (feat)
2. **Task 2: Expand settings_presence_test.go for 5-row callbacks, 8-locale fit, and Owner×Admin double-gating flows** - `3c01b5a6` (test)
3. **Task 3: Update AGENTS.md for Phase 32 invariants and run final Phase 32 verification gate** - `0e13fc3a` (docs)

## Files Created/Modified

- `xenobot/modules/settings_presence.go` - Added `settingsPresenceControlAIReact` (`t=ai`), `settingsPresenceChatRead = chatconv.GetFresh`, `settingsPresenceConvUnlocked`, 5-row `renderSettingsPresence` with segmented `[Off | Low | Medium]` row and Owner-lock note, accurate `settingsPresenceMainState`, and fail-closed `settingsPresenceSetAIReact`.
- `xenobot/modules/settings_presence_test.go` - Updated `TestSettingsPresenceCallbackBytes` and `TestSettingsPresencePageFitsEveryLocale` for 5 rows and `convUnlocked`; added `t=ai` to forged group/Staff checks and added `ai_reactions_owner_admin_double_gate_and_forged_inputs`.
- `AGENTS.md` - Documented Phase 32 `StopChat` `chatPresenceQueue` lane, group `13` handoff, storage/Redis namespace discipline, Featherless `text_only` cache, `chatprompt/reaction.go`, non-tripping shedding, and Settings page `pr` Owner×Admin double-gating.

## Decisions Made

- Extracted `settingsPresenceApplyStored` behind `settingsPresenceSetStored` so `settingsPresenceSetAIReact` runs group/Staff checks once before checking `chatconv.GetFresh` and writing, avoiding a duplicate Staff Group DB query.
- Kept view rendering (`settingsPresenceConvUnlockedForView`) degrading transient `chatconv` read errors safely to `false` (displaying the lock note), while `settingsPresenceSetAIReact` fails closed with `p.cannotOpen()` and zero writes when enabling `low`/`medium`.
- Short-circuited `settingsPresenceConvUnlocked` on `!chatAnyProviderConfigured()` so servers without AI providers perform zero `chat_conversations` DB reads from page `pr` or Main.

## Deviations from Plan

### Minor Test Helper Correction

- **Found during:** Task 2 (`xenobot/modules/settings_presence_test.go`)
- **Issue:** Plan prose cited `withChatProviders(t, true, false, false)`, whose signature is `(t, enabled, anthropic, google)` — passing `false, false` would leave zero AI providers configured and prevent testing the Owner-unlocked enable path.
- **Fix:** Called `withChatProviders(t, true, true, false)` so `EnableChat=true` with Anthropic configured matches `chatconv.Defaults().Provider`.
- **Files modified:** `xenobot/modules/settings_presence_test.go`
- **Commit:** `3c01b5a6`

## Issues Encountered

- `make lint` initially reported `golangci-lint not found` with `/opt/homebrew/bin:/usr/local/bin` alone because `golangci-lint` v2.11.4 is installed in `$HOME/go/bin`; adding `$HOME/go/bin` to `PATH` ran `GOTOOLCHAIN=go1.26.0 make lint` cleanly with `0 issues`.

## Verification

- `CGO_ENABLED=0 go build ./...` — PASS.
- `make check-translations` — PASS (1069 translation keys across all 8 locales).
- `make check-docs` — PASS (no drift detected).
- `GOTOOLCHAIN=go1.26.0 make lint` — PASS (`0 issues`).
- `go test -tags testtools -race -count=1 ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/utils/chatprompt ./xenobot/i18n` — PASS (`1.703s`, `1.378s`, `1.233s`, `39.417s`).
- `go test -tags testtools -race -count=1 ./xenobot/db/backup -run 'TestChatConversationsStayOutOfBackup'` — PASS (`1.452s`).
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestChatReact|TestShutdownEndsChatReact|TestChatWatcher|TestChatProvidersRequest|TestChatLog|TestPresence|TestShutdownEndsPresenceReaction|TestSettingsPresence|TestSettingsChat|TestSettingsMain|TestSettingsFwdBlockMainShape|TestHelpModuleNamesExist'` — PASS (`6.725s`).

## Threat Model Coverage

- **T-32-13 (Elevation of Privilege):** Mitigated in `settingsPresenceSetAIReact` via `settingsRightChangeInfo`, `settingsPresenceGroupRefused`, `settingsPresenceStaffGate`, and fresh `chatconv.GetFresh` check (fail-closed `p.cannotOpen()` on DB error, plain-text `settings_pr_ai_need_owner` alert when locked, unconditional disable to `off`).
- **T-32-14 (Tampering):** Mitigated by validating `v` strictly against `settingsPresenceAIReactCodes` (`"o"`, `"l"`, `"m"`) in both control `Validate` and handler before any DB read or write.
- **T-32-SC (Supply Chain):** Zero new external Go packages added.

## Next Phase Readiness

- Phase 32 (Contextual AI Emoji Reactions) is complete across all 6 plans (`32-01`..`32-06`) and all 4 Phase 32 requirements (`PRES-04`, `REACT-01`, `REACT-02`, `REACT-03`).
- Ready for Phase 32 verification and transition to Phase 33 (Smarter Static Reactions, Highlights Archive and Backup Parity).
- Zero stubs, zero new shutdown drains, zero new handler groups, and zero threat flags introduced.

## Self-Check: PASSED
