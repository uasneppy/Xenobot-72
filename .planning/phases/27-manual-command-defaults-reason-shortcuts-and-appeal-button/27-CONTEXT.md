# Phase 27: Manual Command Defaults, Reason Shortcuts and Appeal Button - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

As the owner, I can set how my admins' manual punishments behave by default, save short words for common reasons, and give banned people a way to appeal.

Requirements: TUNE-04, TUNE-05, TUNE-06.
Depends on: Phase 22 (ban messages and their PM copies carry the appeal button).

Planning notes from ROADMAP.md:
- Per-group commands only. Staff Group commands keep their own durations and reasons (EVT-10, and no Staff Group defaults).
- Decide and record as provisional owner confirmations (following Phase 21–26 convention): which commands the reason-required switch covers (ban, mute, kick, warn), and whether the silent and delete-first variants get the default duration.
- Reason shortcuts are a list (word, then full text) with a two-step add, like Filters on the v1.1 list engine. Expansion happens where the reason is parsed. The expanded reason shows in the group message, the PM copy, the warn reason and the log channel. Stored text is escaped when it is shown.
- The appeal link uses the editor's link check (http or https). The button is added after any custom buttons and counts toward Telegram's button limits.
- Goldens first for the manual command paths this phase changes (the duration parse and the missing-reason refusal), committed alone before production changes (MSG-11).

Success Criteria:
1. With a default duration set, `/mute` and `/ban` given without a duration apply it. An explicit duration is unchanged. With no default, they behave as today.
2. With "reason required" on, a punishment with no reason is refused with a short reply and nothing happens. With a reason, it runs. It is off by default.
3. The owner adds a reason shortcut (a word and its full text) on a page, browses the list and deletes an entry behind a two-tap card. `/ban @user spam` then shows the full text as the reason in the group message, the PM copy, the warn reason and the log post. A word that is not a shortcut is used as typed.
4. With an appeal link set, a ban message (default or custom) and its PM copy carry the button. With none set, they carry none. The link passes the editor's link check.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting (`workflow.skip_discuss=true`). Use ROADMAP phase goal, planning notes, success criteria, requirements TUNE-04..TUNE-06, and codebase conventions in `AGENTS.md` and Phases 21–26 to guide decisions. Document any provisional owner-noticeable defaults clearly so they are easy to confirm or reverse.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase pattern mapping and planning (existing manual moderation in `fuku/modules/bans.go`, `mute.go`, `warns.go`, `moderation.go`; event messages & PM copies in `fuku/modules/event_message*.go`; private settings pages & v1.1 list engine in `fuku/modules/settings_*.go`; duration parsing in `fuku/utils/extraction`; duration bounds in `fuku/db/models/punish.go`).

</code_context>

<specifics>
## Specific Ideas

No additional user specifics — discuss phase skipped. Refer to ROADMAP Phase 27 description, planning notes, and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
