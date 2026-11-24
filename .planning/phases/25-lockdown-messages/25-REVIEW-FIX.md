---
phase: 25-lockdown-messages
fixed_at: 2026-11-24T14:33:10Z
review_path: .planning/phases/25-lockdown-messages/25-REVIEW.md
fix_scope: critical_warning
iteration: 1
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 25: Code Review Fix Report

**Fixed at:** 2026-11-24T14:33:10Z
**Source review:** `.planning/phases/25-lockdown-messages/25-REVIEW.md`
**Fix scope:** `critical_warning`
**Iteration:** 1

**Summary:**
- Findings in scope: 1 (`WR-01`; `WR-02` is tagged `necessity` and `IN-01`/`IN-02` are `Info`, all out of scope for `critical_warning`)
- Fixed: 1
- Skipped: 0

## Fixed Issues

### WR-01: Sticker media on lockdown events silently drops custom text and mandatory technical notes

**Files modified:** `fuku/db/eventmsg/validate_write.go`, `fuku/db/eventmsg/validate_write_test.go`, `fuku/modules/settings_edit_check.go`, `fuku/modules/settings_edit_check_test.go`, `fuku/modules/event_message.go`, `fuku/modules/event_message_test.go`, `fuku/modules/settings_messages_lockdown_test.go`, `fuku/modules/event_message_lockdown_test.go`
**Commit:** `c3b663b6`
**Applied fix:**
1. Rejected captionless `db.STICKER` media in `eventmsg.validateMessageMedia` whenever `ReservedTextFor(msg.Event, msg.Dest) > 0` (returns `"invalid_media"`, consistent with `db.VIDEO_NOTE`), covering Copy Settings and backup import.
2. Rejected `db.STICKER` in `settingsEditCheckMedia` whenever `in.Def.ReservedText > 0` (returns `settings_edit_refuse_need_media`), preventing admins from attaching stickers to Lockdown events in the Messages settings editor while keeping caption-capable media (photo, video, GIF, document, audio, voice) allowed.
3. Added a runtime fallback in `composeEventMessage` so any pre-existing stored `db.STICKER` row on an event with `r.Note != ""` or `eventmsg.ReservedTextFor(r.Event, eventmsg.DestGroup) > 0` falls back to `db.TEXT` (clearing `FileID`), ensuring custom text and mandatory technical notes are never dropped at send time.
4. Added unit tests covering validator rejection (`validate_write_test.go`), editor rejection (`settings_edit_check_test.go`, `settings_messages_lockdown_test.go`), and runtime text fallback (`event_message_test.go`, `event_message_lockdown_test.go`).

## Verification

**Environment:** Main checkout (`workflow.use_worktrees: false`).

- `go vet -tags testtools ./...` — passed
- `CGO_ENABLED=0 go build ./...` — passed
- `make check-translations` — passed (1068 keys across all 8 locales)
- `make check-docs` — passed (no drift)
- `make test-shard-check` — passed
- `TZ=UTC go test -tags testtools -race -count=1 ./fuku/db/eventmsg ./fuku/db/backup` — passed
- `TZ=UTC go test -tags testtools -race -count=1 -v -run '^(TestEventMsgValidateWriteMatchesSettingsPage|TestEventEngineCallers|TestPMCopyCallers|TestEventMessageCompose|TestSettingsEditMediaKinds|TestSettingsEditMediaTextLong|TestSettingsEditReservedText|TestLockdownNoteFitsReserve)$' ./fuku/modules` — passed
- Note: Full `TestSettingsMessagesLockdownFamily` and `TestEventMessageLockdown*` integration suites use TCP `miniredis` on `127.0.0.1`, which is blocked by the sandbox (`connect: operation not permitted`); their pure unit equivalents (`TestSettingsEditMediaKinds`, `TestEventMessageCompose`, `TestEventMsgValidateWriteMatchesSettingsPage`, `TestEventMsgValidateAndReplaceAll`) ran and passed with `-race`, and the new subtests in `settings_messages_lockdown_test.go` and `event_message_lockdown_test.go` compile and vet cleanly under `-tags testtools`.

---

_Fixed: 2026-11-24T14:33:10Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
