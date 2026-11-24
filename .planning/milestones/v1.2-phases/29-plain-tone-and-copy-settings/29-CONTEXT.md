# Phase 29: Plain Tone and Copy Settings - Context

**Gathered:** 2026-11-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

As the owner, I can make the bot's default texts neutral, and I can copy one group's messages and tuning to another group I own.

Requirements: TUNE-08, TUNE-09.
Depends on: Phase 28 (every event and every tuning value exists before the tone covers or the copy moves them).

Planning notes from ROADMAP.md:
- **Plain tone (TUNE-08):** Each event of this milestone needs a neutral default text, group and PM, in all 8 languages (`en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`). It applies only where no custom message is set. A test renders every event in both tones and all 8 languages and holds the length limits and placeholder equality. The MSG-11 goldens are for tone off.
- **Copy settings (TUNE-09):** The source and the target must both be groups where the person may change settings, checked live at the ask and again at Yes (`getChatMember` and the owner gate). The two-tap card is bound to the pair and has an expiry. It names exactly what will be overwritten (custom messages, Silent and PM switches, each tuning area). Decide and record as a provisional default whether an empty source entry clears the target's custom message on copy (default: full mirror — empty source clears target's custom message & switches/tuning in scope so the target ends up matching the source; document clearly as provisional owner decision `P29-01`!).
- A refusal must not reveal whether the bot is in the other group, as the private pick does.
- Copy writes message rows through one shared validated write function that applies the editor's link and length checks (`content.ValidButtonURL`, 4096/1024 limits, 100-button cap). Phase 30's backup import will reuse this same function!
- Copy never moves Staff Group links, approvals, or anything outside messages and tuning.
- Scope rule: everything is configured per group in the bot's private chat; no Staff Group defaults or inheritance (EVT-10); goldens first (MSG-11).

Success Criteria:
1. The Messages page has a plain-tone switch, off by default. With it on, the bot uses neutral default texts for this milestone's events in the group's language (all 8 languages). Custom messages are untouched.
2. From a group's settings, the owner picks another group and sees a card that names exactly what will be overwritten. After Yes, the target group has the same custom messages and tuning values.
3. The person must be allowed to change settings in both groups, checked live. Otherwise the copy is refused, and the refusal does not reveal anything about the other group.
4. Copying happens once. Nothing links the groups afterwards, the source is unchanged, and a stale or replayed card changes nothing.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting (`workflow.skip_discuss=true`). Use ROADMAP phase goal, planning notes, success criteria, requirements TUNE-08 and TUNE-09, and codebase conventions in `AGENTS.md` and Phases 21–28 to guide decisions. Document all provisional owner-noticeable defaults (`P29-01..`) clearly in `AGENTS.md` and `29-VERIFICATION.md`.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase pattern mapping and planning (event messages & default builders in `fuku/modules/event_message*.go`, `fuku/modules/settings_messages.go`, `fuku/db/eventmsg/`, `fuku/db/modtuning/`, `fuku/db/advthresholds/`, `fuku/db/escalation/`, `fuku/db/warns/`, `fuku/db/antiflood/`, `fuku/db/blacklists/`, private group picker in `fuku/modules/settings_menu.go` / `settings_pages.go`, two-tap confirm card patterns, `skipLocal`, and 8 locale files).

</code_context>

<specifics>
## Specific Ideas

No additional user specifics — discuss phase skipped. Refer to ROADMAP Phase 29 description, planning notes, and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
