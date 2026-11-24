# Phase 30: Backup Page and Custom Messages in Backups - Context

**Gathered:** 2026-11-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

As the owner, I can export, import and reset a group's settings from the bot's private chat, and the backup carries the group's custom messages.

Requirements: MSG-10, BKP-01, BKP-02, BKP-03.
Depends on: Phase 29 (the messages table, its event keys and the shared validated message write function `eventmsg.ReplaceAllValidated` are stable).

Planning notes from ROADMAP.md:
- **Existing commands & parity (`BKP-01`, `BKP-02`, `BKP-03`):** The commands already exist: `/export` (live admin and bot admin, the file goes to the admin's private chat, `AcquireExport`), `/import` (live owner, reply to the file, a confirm card, `AcquireImport`) and `/reset` (live owner, a confirm card, `AcquireReset`). The page calls one apply function per action, and the commands call the same ones and keep their replies. Parity tests prove the page and the command leave the same result. The rights fit the existing composites (`settingsRightAdminBotAdmin` for export, `settingsRightOwnerBotAdmin` for import and reset).
- **Cross-replica pending import and reset state (`BKP-02`, `BKP-03`):** The command's pending import and reset tokens live in one replica's memory (`pendingImports`, `pendingResets`). The page's cards must work on any replica. Decide where the parsed file waits between the file and the Yes (Redis with a size cap, or a stored `file_id` that is downloaded and parsed again at Yes — record the provisional decision as `P30-01`). A document in private needs a new part of the typed-reply door (`settings_input_door.go`).
- **Custom messages in backup (`MSG-10`):** Custom messages become a backup module: `AllExportableModules`, export, import and reset. Import applies the editor's link and length checks via Phase 29's shared `eventmsg.ValidateMessage` / `eventmsg.ReplaceAllValidated` (`fuku/db/eventmsg/validate_write.go`), counts what it leaves out in `ImportReport.Skipped`, skips and counts event keys this build does not know, and imports an old export without the module as before (the `TestOldGreetingsExportDecodesToZeros` pattern). Because messages are a backup module, `/reset` of all modules clears them (`chat_event_messages` and `chat_event_message_options`, dropping `event_messages:<chat>` cache) — record this clearly as provisional owner decision `P30-02`.
- **Tuning values stay out of backup:** Tuning values (`chat_escalation`, `chat_offences`, `chat_manual_defaults`, `chat_reason_shortcuts`, `chat_advanced_thresholds`, `chat_conversations`) are not in the requirements and stay out of backup by default, like the staff tables (record as `P30-03`). Update `fuku/db/backup/event_messages_backup_test.go` (which guarded Phases 21–29 before Phase 30) so it verifies custom messages ARE in backup while tuning and staff tables stay out.
- **Private-only (`BKP-01`):** The file is never posted in a group. The page is `PrivateOnly`.
- **Scope rule:** Everything is configured per group in the bot's private chat; no Staff Group defaults or inheritance (EVT-10); goldens first (MSG-11).

Success Criteria:
1. The Backup page opens only in the bot's private chat (a group menu shows the "open in private" note), for a person who passes the live admin check and the owner gate.
2. The owner or an allowed admin picks all modules or some and presses Export. The bot sends the backup file to them in the private chat, never in the group. It is the same file `/export` makes, custom messages included, under the same rate limit and the same admin right.
3. The person presses Import and sends the file in the private chat. The bot shows what will be imported, and a two-tap card confirms. It runs the same import as `/import`, with the same checks, the same report of what was left out and the same rate limit, and only for the group's live owner. Custom messages are restored with the editor's checks, and an old export without them imports as before.
4. The owner resets all or chosen modules to their defaults behind a two-tap card, as `/reset` does, with its rate limit. Anyone who is not the live owner is refused and nothing changes.
5. A stale, expired or forged card or press changes nothing, and the page and the commands leave the same stored result.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting (`workflow.skip_discuss=true`). Use ROADMAP phase goal, planning notes, success criteria, requirements MSG-10, BKP-01, BKP-02, BKP-03, and codebase conventions in `AGENTS.md` and Phases 21–29 to guide decisions. Document all provisional owner-noticeable defaults (`P30-01..`) clearly in `AGENTS.md` and `30-VERIFICATION.md`.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase pattern mapping and planning (`fuku/modules/backup.go`, `fuku/db/backup/`, `fuku/db/eventmsg/validate_write.go`, `fuku/modules/settings_input_door.go`, `fuku/modules/settings_menu.go`, `fuku/modules/settings_copy.go` Redis Lua one-time token pattern, and 8 locale files).

</code_context>

<specifics>
## Specific Ideas

No additional user specifics — discuss phase skipped. Refer to ROADMAP Phase 30 description, planning notes, and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
