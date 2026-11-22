# 26-11: partial, not finished

Done and committed: `TestPhase26Codes`, `TestPhase26ReadsCreateNothing` (`settings_phase26_test.go`), `TestTimedNoticeTextsFitEveryLocale` (`timed_punishment_safety_test.go`). All pass.

Still to do for 26-11:
- `TestTimedPunishmentLogsCarryIDsOnly` (the log sweep over every failing path).
- `TestPhase26Cost` (no `chat_offences` statement with the ladder off, exactly one with it on; no extra Telegram request when drawing the Escalation screens).
- The `TestEventPMTextsFitEveryLocale` extension for the Until line.
- `settingsPrivacyPhase26Screens` and `settingsPrivacyPhase26Run` in `TestSettingsStaffPrivacy`.
- The AGENTS.md read-through, and the gosec and golangci-lint gates, which need tools this machine does not have.

Then 26-12 (end-to-end test and the full gates, which also need PostgreSQL).
