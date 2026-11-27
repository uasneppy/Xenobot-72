---
phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
plan: 05
subsystem: modules
tags: [telegram, presence, highlights-archive, reactions, redis, postgres, stopchat]

# Dependency graph
requires:
  - phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity
    provides: chat_presence schema/repository, TryClaimArchived, Settings normalization (33-02) and reactions rule options (33-03)
provides:
  - Operational Highlights Archive Redis keys outside xenobot:cache: and applyPresence* writers for CatSticker and Highlights Archive settings
  - 2-step /setarchive and /unsetarchive commands (helpers.NewBotCommand) with Staff Group source/destination refusals and group-11 forward capture
  - Group 0 MessageReactionUpdated Highlights Archive tracking (SADD/SREM, author/bot/hostile/emoji-mode filtering), durable PostgreSQL dedup (TryClaimArchived), and CopyMessage/ForwardMessage delivery under StopChat
affects:
  - 33-07 (Settings UI pr & ph sub-page wiring against applyPresence* writers)
  - 33-08 (Verification & AGENTS.md / docs sync)

# Actuals (#2632)
actuals:
  tokens: 24857
  tasks: 3
  commits: 8
  plan_head_before: 872f4c79b1afb0b6410ba116d622d3177e4fbf9b
  plan_head_after: c8f9f57a741a83702bc33146aae26a8ba1c709f6

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "2-step /setarchive forward proof peeking pending marker and checking live admin/ChangeInfo + Staff Group before GETDEL consumePendingMarker"
    - "Distinct-member reactor SET tracking (SADD/SREM on qualifying NewReaction) with fast Redis done latch, lazy lockdown fail-closed check, and atomic TryClaimArchived PostgreSQL claim"
    - "Supergroup-guarded t.me/c/ jump URL construction omitting inline markup on legacy group chats"

key-files:
  created:
    - xenobot/modules/presence_archive.go
    - xenobot/modules/presence_archive_test.go
  modified:
    - xenobot/modules/presence.go
    - xenobot/modules/presence_reactions.go
    - xenobot/modules/presence_test.go
    - xenobot/modules/test_harness_test.go

key-decisions:
  - "Checked peekPendingMarker before Staff Group / admin refusal inside bindSetArchiveForward so unrelated forwards in Staff Groups or normal groups pass through captureSetArchiveForward silently, while marked forwards refuse without burning the token"
  - "Passed ArchiveThreadID=0 when binding a forwarded channel/supergroup post so a source forum group's topic thread ID never leaks into CopyMessage/ForwardMessage calls targeting an external archive channel"
  - "Verified channel bot post rights via FetchBotMember (CanPostMessages on channel administrators; member/admin/creator or restricted CanSendMessages on supergroups/groups)"

patterns-established:
  - "Highlights Archive runs inside StopChat's Group 0 MessageReactionUpdated handler (chatJobs/chatDraining/chatContext) with zero new shutdown drains"

requirements-completed: [COMM-02, COMM-03, COMM-04]

coverage:
  - id: D1
    description: "Operational archive Redis keys sit outside xenobot:cache: with {<chatID>} Cluster hash tags, and applyPresence* writers validate CatStickerEnabled and Highlights Archive bounds before DB writes."
    requirement: COMM-02
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceKeys"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_test.go#TestPresenceApply"
        status: pass
    human_judgment: false
  - id: D2
    description: "2-step /setarchive (channel forward + same-group forum topic) and /unsetarchive use helpers.NewBotCommand, check admin/ChangeInfo and bot post rights before consuming pending tokens, and refuse Staff Groups as both source and destination."
    requirement: COMM-03
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_archive_test.go#TestPresenceArchiveSetAndUnset"
        status: pass
      - kind: unit
        ref: "xenobot/utils/helpers/command_registry_test.go#TestNoRawNewCommand"
        status: pass
    human_judgment: false
  - id: D3
    description: "Group 0 Highlights Archive reaction handler tracks distinct non-author member reactions via Redis SADD/SREM, enforces positive/all and hostile filters, claims once in PostgreSQL via TryClaimArchived after lazy lockdown check, and delivers via CopyMessage (supergroup-guarded jump URL) or ForwardMessage under StopChat."
    requirement: COMM-04
    verification:
      - kind: unit
        ref: "xenobot/modules/presence_archive_test.go#TestPresenceArchiveReactions"
        status: pass
      - kind: unit
        ref: "xenobot/modules/presence_reactions_test.go#TestShutdownEndsPresenceReaction"
        status: pass
    human_judgment: false

# Metrics
duration: 8 min
completed: 2026-11-27
status: complete
---

# Phase 33 Plan 05: Highlights Archive & Presence Apply Layer Summary

**Implemented the Highlights Archive runtime (`COMM-03`, `COMM-04`) and all `applyPresence*` writers for Cat Stickers & Highlights (`COMM-02`, `COMM-03`): 2-step `/setarchive` / `/unsetarchive` proof, Group 0 distinct-member reactor tracking, durable PostgreSQL `(chat_id, message_id)` deduplication, and `CopyMessage` (with supergroup-guarded `t.me/c/` jump button) / `ForwardMessage` delivery under `StopChat`.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-11-27T06:54:39Z
- **Completed:** 2026-11-27T07:02:04Z
- **Tasks:** 3
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- Added operational Redis keys (`presenceArchiveUsersKey`, `presenceArchiveDoneKey`) outside `xenobot:cache:`, extended `presenceWrites`, and implemented `applyPresenceCatStickerEnabled`, `applyPresenceArchiveEnabled`, `applyPresenceArchiveTarget`, `applyPresenceClearArchiveTarget`, `applyPresenceArchiveThreshold`, `applyPresenceArchiveEmojiMode`, and `applyPresenceArchiveDelivery` in `xenobot/modules/presence.go` (plus `&models.ChatPresenceArchived{}` in `xenobot/modules/test_harness_test.go`).
- Built `/setarchive`, `bindSetArchiveForward`, `captureSetArchiveForward` (group 11), and `/unsetarchive` in `xenobot/modules/presence_archive.go` using `helpers.NewBotCommand`, preserving non-admin and Staff Group tokens and restoring markers on save failure.
- Implemented `presenceHandleArchive` and `presenceDeliverArchive` in `xenobot/modules/presence_archive.go` wired into `LoadPresence` and `presenceHandleReaction` in `xenobot/modules/presence_reactions.go`, with author/bot/anonymous/hostile filtering, `positive` vs `all` emoji mode, lazy lockdown check, `TryClaimArchived` deduplication, and supergroup-guarded `CopyMessage` / `ForwardMessage` delivery under `StopChat`.

## Task Commits

Each task was committed atomically on `main` (Note: `git rev-list --count plan_head_before..plan_head_after` is `8` because Wave 2 peers `33-04` and `33-06` committed 5 interleaved commits on `main` in parallel):

1. **Task 1: Extend presence.go apply layer, add ChatPresenceArchived to test harness, and build /setarchive / /unsetarchive in presence_archive.go** - `007f91e1` (feat)
2. **Task 2: Implement Highlights reactor tracking, durable Postgres dedup, Copy/Forward delivery, and LoadPresence wiring** - `8b3af1b6` (feat)
3. **Task 3: Add tests for archive keys, apply layer, /setarchive proof, reactor dedup, and delivery modes** - `c8f9f57a` (test)

## Files Created/Modified
- `xenobot/modules/presence.go` - Operational archive Redis keys, `presenceWrites` fields, and `applyPresence*` writers with bounds checks
- `xenobot/modules/presence_archive.go` - 2-step `/setarchive` / `/unsetarchive` handlers, group-11 forward capture, `presenceHandleArchive`, and `presenceDeliverArchive`
- `xenobot/modules/presence_reactions.go` - `LoadPresence` registration of `/setarchive`, `/unsetarchive`, `captureSetArchiveForward`, `SetModuleEnabled`, and `presenceHandleArchive` dispatch
- `xenobot/modules/test_harness_test.go` - Registered `&models.ChatPresenceArchived{}` in `AutoMigrate`
- `xenobot/modules/presence_test.go` - Extended `TestPresenceKeys` and `TestPresenceApply` for all Phase 33 keys and writers
- `xenobot/modules/presence_archive_test.go` - End-to-end dispatcher + miniredis + SQLite tests for `/setarchive`, `/unsetarchive`, Staff Group refusals, reactor `SADD`/`SREM`, author exclusion, emoji filtering, durable DB dedup across Redis flush, `CopyMessage` jump button guard, `ForwardMessage`, and lockdown fail-closed behavior

## Decisions Made
- Peeked `setarchivePendingKey` before Staff Group / admin checks inside `bindSetArchiveForward` so ordinary forwarded posts in group 11 return `false` silently while marked forwards to or from Staff Groups refuse without burning the marker.
- Stored `targetThreadID = 0` for forwarded destinations in `bindSetArchiveForward` (reserving non-zero `ArchiveThreadID` for direct same-group `/setarchive` inside a forum topic) so a source forum group's thread ID cannot leak into external archive deliveries.
- Checked live destination post permission via `chat_status.FetchBotMember` (`CanPostMessages` for channel admins; member/admin/creator or `CanSendMessages` when restricted for groups/supergroups).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Verification
- `go test -tags testtools -race -count=1 ./xenobot/modules -run 'TestPresence|TestShutdownEndsPresenceReaction|TestHelpModuleNamesExist'` — passed (`3.481s`)
- `go test -tags testtools -race -count=1 ./xenobot/utils/helpers -run 'TestNoRawNewCommand'` — passed (`1.896s`)
- `GOTOOLCHAIN=go1.26.0 golangci-lint run --build-tags testtools ./xenobot/modules/...` — `0 issues`

## Self-Check: PASSED

## Next Phase Readiness
- `applyPresence*` writers and `presenceHandleArchive` are ready for `33-07` (Presence Settings UI & Highlights sub-page `ph`).

---
*Phase: 33-smarter-static-reactions-highlights-archive-and-backup-parity*
*Completed: 2026-11-27*
