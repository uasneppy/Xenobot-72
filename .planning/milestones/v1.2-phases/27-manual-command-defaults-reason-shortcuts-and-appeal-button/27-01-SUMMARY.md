---
phase: 27-manual-command-defaults-reason-shortcuts-and-appeal-button
plan: 01
subsystem: testing
status: complete
tags: [golden, characterization, moderation, bans, mutes, warns, msg-11]
requires: []
provides:
  - TestLegacyPhase27ModerationRequests (17 subtests)
  - goldenP27* constants (15 exact sendMessage JSON goldens)
  - phase27GoldenRun and phase27AssertNoUntilDate helpers
affects: [27-02, 27-03, 27-04, 27-05, 27-06, 27-07]
tech-stack:
  added: []
  patterns:
    - "Characterization goldens recorded directly against unchanged code at b7393953 and committed alone before any production change (MSG-11)"
    - "Shared newModerationGoldenSite and withModerationGoldenLocales harness from moderation_legacy_request_test.go"
key-files:
  created:
    - fuku/modules/moderation_phase27_legacy_request_test.go
  modified: []
decisions:
  - "Characterized /swarn on unchanged code at b7393953 accurately: warns.go deletes command message 101 and still sends the warn event reply (queued under KindBotManual), pinned via goldenP27SwarnNoReason."
  - "Pinned stored warn reasons for no-reason /warn, /dwarn, /swarn to the actual unchanged-code slice []string{\"Причину не вказано\"} (warns_no_reason_given) via warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)."
  - "Reused newModerationGoldenSite, withModerationGoldenLocales, moderationGoldenTarget, and legacyRequestJSON without modifying moderation_legacy_request_test.go or any production file."
metrics:
  duration: 6min
  completed: 2026-11-22
actuals:
  tokens: 3864
  tasks: 2
  commits: 1
plan_head_before: 580c08e53199d0b7badd83764fded718205b823c
plan_head_after: 71dbf865f008fb493c604ce5c8078ae6a5f7159f
---

# Phase 27 Plan 01: Manual Command Characterization Goldens Summary

`TestLegacyPhase27ModerationRequests` pins, on unchanged code at `b7393953` before any Phase 27 production edit (MSG-11), the exact `sendMessage`, punishment, deletion, and stored-warn behavior across 17 manual moderation command cases in Ukrainian with real locales (`uk`): `/tban` and `/tmute` missing-duration and invalid-duration refusals (zero punishment calls and zero deletion queue rows), `/ban 2h spam` and `/mute 30m noise` treating leading duration-like text as literal reason with no `until_date`, untimed `/ban spam`, `/dban`, `/sban 42`, `/mute`, `/dmute noise`, `/smute 42` with `phase27AssertNoUntilDate` confirming absence of `until_date`, `/kick` with no reason, and `/warn`, `/warn spam`, `/dwarn`, `/swarn` with stored `warns.GetWarns` counts/reasons and exact replies. The new golden file `fuku/modules/moderation_phase27_legacy_request_test.go` was committed alone in `71dbf865`.

## Performance

- **Duration:** 6 min
- **Started:** 2026-11-22T19:40:23Z
- **Completed:** 2026-11-22T19:45:56Z
- **Tasks:** 2
- **Files created:** 1

## Accomplishments

- Created `fuku/modules/moderation_phase27_legacy_request_test.go` (`//go:build testtools`) with `TestLegacyPhase27ModerationRequests` (17 subtests), 15 `goldenP27*` exact `sendMessage` JSON constants, `phase27GoldenRun`, and `phase27AssertNoUntilDate`.
- Verified all 17 subtests of `TestLegacyPhase27ModerationRequests` and all pre-existing subtests of `TestLegacyModerationRequests` pass under `-tags testtools -race -count=1`.
- Verified `gofmt -l`, `go vet -tags testtools ./fuku/modules`, `CGO_ENABLED=0 go build ./...`, and `git diff --exit-code b7393953 -- go.mod go.sum` all succeed cleanly.
- Committed `fuku/modules/moderation_phase27_legacy_request_test.go` alone in `71dbf865` with zero production commits between `b7393953` and HEAD.

## Task Commits

1. **Tasks 1 & 2: Record Phase 27 characterization goldens and commit golden file alone** - `71dbf865` (`test(27-01): pin manual command duration parses, missing-reason and untimed replies before Phase 27`)

## Files Created/Modified

- `fuku/modules/moderation_phase27_legacy_request_test.go` - Phase 27 MSG-11 manual moderation command characterization golden suite (17 subtests, 15 `goldenP27*` constants, `phase27AssertNoUntilDate`, `phase27GoldenRun`).

## Decisions Made

- Pinned actual unchanged-code behavior at `b7393953` without editing any production file or pre-existing golden file (`moderation_legacy_request_test.go`).
- Kept the `moduleBotCall` type from `test_harness_test.go` for `phase27AssertNoUntilDate` and called `warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)` using the repository's `(userId, chatId)` signature.

## Deviations from Plan

### Characterization Adjustments to Match Unchanged Code

**1. [Rule 1 - Bug] Recorded actual unchanged-code `/swarn` reply and default no-reason warn string**
- **Found during:** Task 1 (recording goldens against unchanged `b7393953`)
- **Issue:** The plan prose assumed `/swarn` sends zero `sendMessage` and that no-reason warns store `""` in `warns.GetWarns`. On unchanged code (`fuku/modules/warns.go:414-441`), `/swarn` deletes command message `101` and still sends the warn event reply (`postEvent`, queued under `KindBotManual`), and `warns.WarnUser` stores the localized fallback string `"Причину не вказано"` (`warns_no_reason_given`) when reason is empty. Also, `warns.GetWarns` takes `(userId, chatId)` rather than `(chatId, userId)` and the recorded call type is `moduleBotCall`.
- **Fix:** Recorded `goldenP27SwarnNoReason` for `/swarn`'s actual `sendMessage` request, asserted `[]string{"Причину не вказано"}` for the three no-reason warn cases, used `warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)`, and typed `phase27AssertNoUntilDate(t *testing.T, call moduleBotCall)`.
- **Files modified:** `fuku/modules/moderation_phase27_legacy_request_test.go`
- **Commit:** `71dbf865`

## Issues Encountered

None — all tests, vet, build, formatting, and git-invariant checks passed on the first post-recording run.

## Known Stubs

None.

## Next Phase Readiness

- Phase 27 MSG-11 baseline goldens are frozen in a standalone commit (`71dbf865`) ahead of all production changes.
- Plans 27-02..27-07 can proceed and rerun both `TestLegacyPhase27ModerationRequests` and `TestLegacyModerationRequests` untouched.

## Self-Check: PASSED

- `fuku/modules/moderation_phase27_legacy_request_test.go`: FOUND
- Commit `71dbf865`: FOUND (ancestor of HEAD, 1 file changed, 0 production files modified since `b7393953`)
