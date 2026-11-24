# 26-10 summary: the private Escalation page

**Done.** Page `j` (Order 43, after Warnings) sets a group's escalation ladder from the bot's private chat: on/off, up to five steps (mute, kick, ban, timed mute, timed ban with the shared duration screen), Remove last, and the forget period (1, 7, 30, 90 days).

- `fuku/modules/escalation_settings.go`: four apply functions (`applyEscalationEnabled/Step/RemoveStep/Forget`) behind the `escalationWrites` seam. They check against a fresh read, so a stale screen writes nothing. Turning on an empty ladder stores an example ladder. Turning off deletes the group's offence counts.
- `fuku/modules/settings_escalation.go`: the page, `PrivateOnly`, hidden in a Staff Group, seven page actions that all require the private owner field `c`. Writes need `settingsRightRestrict`.
- 17 locale keys in all 8 languages and their limit rows. The tests caught a 21-code-point French cell ("Retirer la dernière"), now "Retirer dernière".
- Main tests updated for the new cell (24 state lines; a Staff Group hides it).
- AGENTS.md has a bullet for the page.

**Passed:** `TestEscalationApply`, `TestSettingsEscalation{Opens,Steps,Forged,ReadsCreateNothing,CallbackBytes,PageFitsEveryLocale}`, `TestSettingsDriftEscalation`, `TestSettingsAuthorityEscalation`, the i18n locale/label tests, `make check-translations`, `make check-docs`, build, vet.

**Not clean, not caused by this plan:** `TestSettingsCaptchaToggle` (`chk_captcha_expires_at`) and `TestSettingsTimezoneTyped` fail the same way without my changes. `TestSettingsDeletionRightLine/a_429` failed once under load and passed alone twice.
