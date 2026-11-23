# Phase 28: Advanced Thresholds - Context

**Gathered:** 2026-11-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

As the owner, I can tune the detection and screening limits that are fixed in code today, per group, inside safe bounds.

Requirements: TUNE-07.
Depends on: Phase 27.

Planning notes from ROADMAP.md:
- The values and their constants today (from `AGENTS.md`):
  - join surge window 60 s
  - flood window 30 s with the 60% new-member share, the 3-account floor and the 24 h newness age
  - AI burst window 120 s
  - `raidAIImageBar` 0.85
  - `memberReqAIBar` 0.9
  - the member requirement cap of 10 a minute and its 10-minute pause
  - `lockdownReminderEvery` of 1 hour
  - a captcha timeout above today's 10-minute cap
- Hot paths: the flood watcher reads its settings on every message and the Redis Lua scripts fix their windows, caps and expiries. A window change must keep the script arguments, caps and TTLs consistent, and it must add no database round trip per message. Each value needs Go-side bounds and a default equal to today's constant, with a test that fails if the two drift.
- A change must act on every replica at the next event, so decide `skipLocal` for the new key.
- Goldens first for the behaviors whose constants move (MSG-11).
- Scope rule: everything is set group by group in that group's own private `/settings` chat; no Staff Group defaults or inheritance (EVT-10).

Success Criteria:
1. A private Advanced page lists the areas (raid windows, new-member share and age, AI spam bars, member requirement cap and pause, lockdown reminder interval, captcha timeout). Each value shows its current setting, its default and its allowed range.
2. A value can only be set inside its bounds. An out-of-range, stale or forged press changes nothing. Reset to default puts today's constant back, for one value or one area.
3. A changed value acts at the next relevant event on every replica. For example, a new flood window changes what counts as a flood, a new reminder interval changes when the next lockdown reminder posts, and a captcha timeout above 10 minutes applies to the next joiner.
4. A group that changes nothing behaves exactly as today.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting (`workflow.skip_discuss=true`). Use ROADMAP phase goal, planning notes, success criteria, requirement TUNE-07, and codebase conventions in `AGENTS.md` and Phases 21–27 to guide decisions. Record any provisional owner-noticeable defaults (`P28-01..`) clearly in `AGENTS.md` and `28-VERIFICATION.md`.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase pattern mapping and planning (raid detection windows & Lua scripts in `fuku/modules/raid*.go`, AI bars in `fuku/modules/raid_ai*.go` and `fuku/modules/memberreq_ai*.go`, member requirement cap/pause in `fuku/modules/memberreq*.go`, lockdown reminder interval in `fuku/modules/lockdown*.go`, captcha timeout in `fuku/modules/captcha*.go` and `settings_captcha.go`, private settings pages and `skipLocal` cache keys).

</code_context>

<specifics>
## Specific Ideas

No additional user specifics — discuss phase skipped. Refer to ROADMAP Phase 28 description, planning notes, and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
