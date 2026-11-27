---
phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
plan: 05
subsystem: ui
tags: [presence, settings, inline-keyboard, staff-group, locales, agents-md]

# Dependency graph
requires:
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: Shared PRES-02 safety gate and applyPresence* helpers (31-02)
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: Complete 8-locale translations and length ceilings for Presence settings page pr and Conversations shortcut (31-03)
  - phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions
    provides: LoadPresence group-0 reaction router, StopChat tracking, 👎 retraction, and Emoji Consensus (31-04)
provides:
  - Private-only Presence Settings page pr (Order 91, Icon ✨, paired y|pr at 17 Main rows) with Emoji Consensus toggle, 2..20 threshold stepper, and 👎 Retraction toggle under settingsRightChangeInfo
  - Single ✨ Presence → shortcut row (a=op&p=pr) on Owner-only Conversations page p staying within <= 8 rows across all 8 locales
  - Race-safe 8-locale render, 64-byte callback, owner/admin/CanChangeInfo, group-menu, Staff Group, and save-failure tests plus updated Main/Staff/Chat/Phase 18-30 registry tests
  - Updated AGENTS.md documenting all Phase 31 invariants
affects:
  - phase-32-contextual-ai-emoji-reactions
  - phase-33-smarter-static-reactions-highlights-and-backup-parity

actuals:
  tokens: 12123
  tasks: 3
  commits: 3
  plan_head_before: ce7a6a2bb1d32d0064cf17b4c84f4bcb29a1c563
  plan_head_after: 374124e61cbbbc3af5a77816f5d48541a603eeef

tech-stack:
  added: []
  patterns:
    - "Private-only Settings page pr at Order 91 pairing with y (Order 89) to keep settingsMainOwnerRows = 17 across 31 registered pages"
    - "Fail-closed Staff Group defense-in-depth (Hidden + settingsPresenceStaffGate via staff.GetStaffGroupFresh) returning opaque common_callback_invalid_request without naming Staff Group"
    - "Canonical base-10 integer validator (settingsParseNumber + strconv.Itoa(n) == v + presence.ValidConsensusThreshold) rejecting leading zeros and out-of-range values before DB I/O"

key-files:
  created:
    - xenobot/modules/settings_presence.go
    - xenobot/modules/settings_presence_test.go
  modified:
    - xenobot/modules/settings_chat.go
    - xenobot/modules/settings_main_test.go
    - xenobot/modules/settings_chat_test.go
    - xenobot/modules/settings_backup_test.go
    - xenobot/modules/settings_phase18_test.go
    - xenobot/modules/settings_phase19_test.go
    - xenobot/modules/settings_phase29_test.go
    - xenobot/modules/settings_phase30_test.go
    - AGENTS.md

key-decisions:
  - "Checked settingsPresenceStaffCheck(group.Id) != settingsPresenceFree inside settingsPresenceMainState and settingsPresenceOpen (matching settingsDelMainState/settingsDelOpen) so Staff lookup errors fail closed everywhere"
  - "Updated registered page lists across Phase 18/19/29/30 code-registry tests to 31 pages (adding pr) so the full settings test suite remains green"

patterns-established:
  - "Settings pages controlling admin-facing v1.3 Presence features sit on private page pr (settingsRightChangeInfo, PrivateOnly, Hidden in Staff Groups), strictly separated from Owner-only AI consent on page p"

requirements-completed: [PRES-01, PRES-02, PRES-03]

coverage:
  - id: D1
    description: "Private Settings page pr (Order 91, y|pr at 17 Main rows, 8 locales) configures Emoji Consensus toggle, 2..20 threshold stepper, and 👎 Retraction toggle under settingsRightChangeInfo with zero omitted buttons and <= 64-byte callbacks"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceCallbackBytes"
        status: pass
      - kind: unit
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresencePageFitsEveryLocale"
        status: pass
      - kind: integration
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows"
        status: pass
      - kind: integration
        ref: "xenobot/modules/settings_main_test.go#TestSettingsMainPhase18"
        status: pass
    human_judgment: false
  - id: D2
    description: "Conversations page p remains OwnerOnly: true at <= 8 rows with the ✨ Presence → shortcut opening page pr in private"
    requirement: PRES-03
    verification:
      - kind: unit
        ref: "xenobot/modules/settings_chat_test.go#TestSettingsChatPageFitsEveryLocale"
        status: pass
      - kind: integration
        ref: "xenobot/modules/settings_chat_test.go#TestSettingsChatLog/presence_shortcut_row_opens_page_pr_in_private"
        status: pass
    human_judgment: false
  - id: D3
    description: "Group menu refuses page pr with settings_pr_group_note (0 DB reads) and invalid-request alerts on forged controls; Staff Groups hide pr from Main and fail closed without naming Staff Group"
    requirement: PRES-02
    verification:
      - kind: integration
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows/group_menu_private_only_note_and_forged_control_refusal"
        status: pass
      - kind: integration
        ref: "xenobot/modules/settings_presence_test.go#TestSettingsPresenceFlows/staff_group_hidden_refused_and_lookup_error_fails_closed"
        status: pass
    human_judgment: false
  - id: D4
    description: "AGENTS.md documents all Phase 31 invariants and full build, translation, docs, lint, and targeted -race tests pass"
    requirement: PRES-01
    verification:
      - kind: other
        ref: "CGO_ENABLED=0 go build ./... && make check-translations && make check-docs && GOTOOLCHAIN=go1.26.0 make lint"
        status: pass
      - kind: integration
        ref: "go test -tags testtools -race -count=1 ./xenobot/config ./xenobot/db/cache ./xenobot/db/migrations ./xenobot/db/presence ./xenobot/i18n ./xenobot/db/backup ./xenobot/modules"
        status: pass
    human_judgment: false

duration: 9 min
completed: 2026-11-26
status: complete
---

# Phase 31 Plan 05: Presence Settings Page (`pr`), Conversations Shortcut, and Phase 31 Finalization Summary

**Implemented the private-only `Presence` Settings page (`pr`, Order 91, paired `y|pr` keeping Main at 17 rows), added the `✨ Presence →` shortcut row on Owner-only Conversations page `p` (`<= 8` rows across all 8 locales), verified all permission/Staff Group/locale/callback invariants under `-race`, and documented Phase 31 contracts in `AGENTS.md`.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-11-26T23:11:59Z
- **Completed:** 2026-11-26T23:20:53Z
- **Tasks:** 3
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments
- Created `xenobot/modules/settings_presence.go` registering private-only Settings page `pr` (`Order: 91`, icon `✨`, `OwnerOnly: false`, `PrivateOnly: true`, `PrivateNoteKey: "settings_pr_group_note"`, `Hidden: settingsPresenceHidden`) with controls `c` (Emoji Consensus toggle), `ct` (canonical `2..20` threshold stepper), and `r` (`👎` Retraction toggle) all guarded by `settingsRightChangeInfo`, explicit `!p.private` refusal, and fail-closed `settingsPresenceStaffGate` (`staff.GetStaffGroupFresh`).
- Updated `xenobot/modules/settings_chat.go` (`renderSettingsChat`) to add a single `settings_btn_chat_presence` (`a=op&p=pr`) shortcut row right before `k.navRow(false)`, keeping Owner-only page `p` within `<= 8` rows even with all 3 AI providers configured across all 8 locales.
- Created `xenobot/modules/settings_presence_test.go` covering worst-case 14-digit chat ID callback byte lengths (`<= 64` bytes, `Omitted == 0`), 8-locale UI fit (`rows <= 8`, `UTF-16 <= 3800`, button runes `1..40`), owner toggle/stepper/bounds/idempotency/save-failure flows, admin `CanChangeInfo` allow/deny checks, group-menu private-only note (0 DB reads) and forged control refusals, forged out-of-range thresholds (`1`, `21`, `03`, `-2`, `abc`), and Staff Group hiding/refusal/lookup-error fail-closed checks.
- Updated `settings_main_test.go`, `settings_chat_test.go`, `settings_backup_test.go`, and Phase 18/19/29/30 settings registry tests for `y|pr` (17 Main rows, 31 registered pages, Staff Group state lines `len(settingsMainTitleKeys) - 8`) and the page `p` shortcut button.
- Updated `AGENTS.md` with Phase 31 invariants (`StopChat` reaction tracking with 70 s worst case unchanged, group-0 `handlers.NewReaction` and `"message_reaction"` `AllowedUpdates` rolling-deploy rule, `chat_presence` `skipLocal`, `xenobot:presence:{<chat>}:*` operational keys, `AvailableReactions` 20m cache, explicit `[]gotgbot.ReactionType{}` clear, `ActorChat.Id == Chat.Id` retraction policy, and private page `pr` / page `p` shortcut) and passed the entire build, translations, docs, lint (`0 issues`), and `-race` test gate.

## Task Commits

Each task was committed atomically:

1. **Task 1: Register private Settings page pr and add shortcut row to Conversations page p** - `efb47cac` (feat)
2. **Task 2: Add settings_presence_test.go and update Main, Chat, Backup, and Phase 18 settings tests** - `8ca17999` (test)
3. **Task 3: Update AGENTS.md for Phase 31 invariants and run final phase verification** - `374124e6` (docs)

## Files Created/Modified
- `xenobot/modules/settings_presence.go` - Private-only `Presence` Settings page `pr` with Staff Group hiding, fail-closed `settingsPresenceStaffGate`, canonical `2..20` threshold validation, pure 4-row renderer, and idempotent read-back setters.
- `xenobot/modules/settings_chat.go` - Added single `settings_btn_chat_presence` (`a=op&p=pr`) shortcut row before `k.navRow(false)`.
- `xenobot/modules/settings_presence_test.go` - Pure callback-byte and 8-locale fit tests plus end-to-end SQLite flows for owner, admin `CanChangeInfo`, group menu, forged thresholds, and Staff Group fail-closed paths.
- `xenobot/modules/settings_main_test.go` - Updated `settingsMainShapeGroup` / `settingsMainShapePrivate` (`y|pr`, keeping `settingsMainOwnerRows = 17`) and added `"settings_page_presence"` to `settingsMainTitleKeys`.
- `xenobot/modules/settings_chat_test.go` - Added `presence_shortcut_row_opens_page_pr_in_private` subtest and verified 8-locale fit & callback bytes with the new row.
- `xenobot/modules/settings_backup_test.go` - Updated Staff Group Main state-line count assertion from `-7` to `-8` (9 hidden pages including `pr`).
- `xenobot/modules/settings_phase18_test.go`, `xenobot/modules/settings_phase19_test.go`, `xenobot/modules/settings_phase29_test.go`, `xenobot/modules/settings_phase30_test.go` - Updated Staff Group state-line count (`-8`) and exact 31-page registry lists to include `"pr"`.
- `AGENTS.md` - Documented Phase 31 `StopChat`, group-0 `NewReaction`/`AllowedUpdates`, and Presence subsystem invariants.

## Decisions Made
- Applied `settingsPresenceStaffCheck(group.Id) != settingsPresenceFree` inside `settingsPresenceMainState` and `settingsPresenceOpen` (mirroring `settingsDelMainState` and `settingsDelOpen`) so a transient Staff Group DB lookup failure fails closed on Main, Open, and every control press.
- Executed `make lint` with `GOTOOLCHAIN=go1.26.0` (matching `go.mod`'s `go 1.26.0`) because Homebrew's ambient `go1.27.0` produces export data v4 unreadable by the installed `golangci-lint v2.11.4`; lint completed with `0 issues`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated exact 31-page registry assertions in Phase 18, 19, 29, and 30 settings code tests**
- **Found during:** Task 2 (settings test verification)
- **Issue:** Registering the 31st Settings page (`pr`) caused exact registered-page-list assertions in `TestSettingsPhase18Codes`, `TestSettingsPhase19Codes`, `TestPhase29Codes`, and `TestPhase30Codes` to fail expecting the previous 30 pages.
- **Fix:** Added `"pr"` to the expected registered-page lists (and updated the expected page count from 30 to 31) in `settings_phase18_test.go`, `settings_phase19_test.go`, `settings_phase29_test.go`, and `settings_phase30_test.go`.
- **Files modified:** `xenobot/modules/settings_phase18_test.go`, `xenobot/modules/settings_phase19_test.go`, `xenobot/modules/settings_phase29_test.go`, `xenobot/modules/settings_phase30_test.go`
- **Commit:** `8ca17999`

## Issues Encountered
- Ambient `/opt/homebrew/bin/go` is `go1.27.0` while `golangci-lint` (`$HOME/go/bin/golangci-lint`, v2.11.4) expects Go 1.26 export data (`go.mod` specifies `go 1.26.0`). Running `GOTOOLCHAIN=go1.26.0 make lint` used the cached `go1.26.0` toolchain and reported `0 issues`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 31 (Presence Foundation, Settings Page and Zero-AI Social Reactions) is complete across all 5 plans (`31-01` through `31-05`) with all 6 Phase 31 requirements (`PRES-01`, `PRES-02`, `PRES-03`, `PRES-05`, `REACT-04`, `REACT-05`) implemented and verified.
- Phase 32 (Contextual AI Emoji Reactions) can build directly on `chat_presence`, page `pr`, `presence.go` safety gates, `presence_react_api.go`, and `StopChat`-tracked presence handlers.
- Zero stubs, zero skipped tests, zero blockers.

---
*Phase: 31-presence-foundation-settings-page-and-zero-ai-social-reactions*
*Completed: 2026-11-26*

## Self-Check: PASSED
