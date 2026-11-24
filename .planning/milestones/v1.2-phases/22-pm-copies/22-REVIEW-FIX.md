---
phase: 22-pm-copies
fixed_at: 2026-11-24T10:54:00Z
review_path: /Users/denyspetrenko/Xenobot-72/.planning/phases/22-pm-copies/22-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 22: Code Review Fix Report

**Fixed at:** 2026-11-24T10:54:00Z
**Source review:** `/Users/denyspetrenko/Xenobot-72/.planning/phases/22-pm-copies/22-REVIEW.md`
**Fix scope:** `critical_warning`
**Iteration:** 1
**Verification environment:** main checkout (`workflow.use_worktrees=false`)

**Summary:**
- Findings in scope: 4 (`WR-01`..`WR-04`; Info items `IN-01`..`IN-03` out of scope)
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `pmCopySend` discards the bounded timeout context from `pmCopyPaced`

**Files modified:** `fuku/utils/media/sender.go`, `fuku/utils/media/sender_test.go`, `fuku/modules/event_message_pm.go`
**Commit:** `0a501041`
**Applied fix:** Added `media.SendWithContext` and `media.SendGreetingMessageWithContext` routing all Telegram send calls through gotgbot's `*WithContext` methods, updated `pmCopySend` to pass `callCtx` from `pmCopyPaced`, and added `TestSendGreetingMessageWithContextPropagatesContext`.

### WR-02: Member requirements PM copies disagree on media support between Settings UI, DB validation, and send path

**Files modified:** `fuku/modules/settings_messages.go`, `fuku/modules/copy_settings_test.go`
**Commit:** `f9f52f43`
**Applied fix:** Updated `settingsMsgPMEditDef` to always include `settingsEditMediaPart` (`[]settingsEditPart{settingsMsgTextPart(fam), settingsEditMediaPart, settingsEditButtonsPart}`) so Settings UI matches `eventmsg.NoMedia(event, DestPM)` (`false`) and `composeEventPM`, and added a `NoMedia(ev.Key, eventmsg.DestPM)` parity assertion to `TestEventMsgValidateWriteMatchesSettingsPage`.

### WR-03: `pmCopyDetailsDeepLink` returns silence when sending a stored custom copy fails

**Files modified:** `fuku/modules/event_message_pm.go`, `fuku/modules/event_message_pm_details.go`, `fuku/modules/event_message_pm_details_test.go`
**Commit:** `45b5b940`
**Applied fix:** Stored `Fallback` (`fb`) and `AppealRow` (`ar`) on `pmCopyDetailsRecord` when an unvalidated custom copy is saved (clearing `o.Custom` in `runEventPM` if fallback was already tried), updated `pmCopyDetailsDeepLink` to retry once with the default fallback text/appeal row on `eventMessageRefused(err)` and return `neutral()` on failure instead of silence, and added in-process test `TestEventPMDetailsDeepLinkFallbackAndNeutral`.

### WR-04: `pmCopyAddDetails` can overwrite later keyboard updates and resurrect resolved action buttons

**Files modified:** `fuku/modules/event_message_pm_details.go`, `fuku/modules/bans.go`, `fuku/modules/warns.go`, `fuku/modules/memberreq_fixed.go`, `fuku/modules/event_message_pm_details_test.go`
**Commit:** `9d04a3ca`
**Applied fix:** Added bounded in-process + Redis resolved-action tracking (`pmCopyMarkGroupActionResolved` / `pmCopyGroupActionResolved`), called it before editing the group message in the three interactive action handlers (`unrestrict` in `bans.go`, `rmWarn` in `warns.go`, and `memberReqFixedRun` in `memberreq_fixed.go`), skipped `pmCopyAddDetails` both before storing the token and right before `EditMessageReplyMarkupWithContext` when `o.GroupRows` contains interactive callback buttons and the message's action has already resolved, and added in-process test `TestEventPMDetailsSkipsResolvedInteractiveMessage`.

## Verification

Ran in the main checkout (`TZ=UTC`):
- `go vet -tags testtools ./...` — PASS
- `CGO_ENABLED=0 go build ./...` — PASS
- `make check-translations` — PASS (1068 keys across all 8 locales)
- `make check-docs` — PASS (no drift)
- `make test-shard-check` — PASS
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/utils/media` — PASS
- `TZ=UTC go test -tags testtools -race -count=1 -run '^TestEventMsgValidateWriteMatchesSettingsPage$|^TestEventEngineCallers$|^TestPMCopyCallers$|^TestEventPMDetailsDeepLinkFallbackAndNeutral$|^TestEventPMDetailsSkipsResolvedInteractiveMessage$|^TestLegacyPhase29Baseline$' ./fuku/modules` — PASS
- Note: Existing legacy tests under `TestEventPM*` / `TestSettingsMessages*` that dial `miniredis.RunT` over `127.0.0.1` could not run inside this sandbox because localhost TCP dials are blocked (`connect: operation not permitted`); all new tests use `phase28InProcessMiniredis(t)` and pass with `-race`.

---

_Fixed: 2026-11-24T10:54:00Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
