# 26-11 summary: the safety and privacy sweep

**Done.** The phase's logs, texts, codes, reads, costs and Staff Group privacy are held by tests.

- `TestTimedPunishmentLogsCarryIDsOnly`: every failing path of the phase under a debug hook. It found two lines that logged the raw error text of a failed write (the blacklist action's and the warn settings page's). Both now log `escalationErrClass` only. Telegram's own refusal text is not treated as a leak: the untimed punishments have always logged it and it is server wording, never a member's. This is a judgement call, recorded in AGENTS.md.
- `TestTimedNoticeTextsFitEveryLocale`, and an Until-line check added to `TestEventPMTextsFitEveryLocale` (only added assertions).
- `TestPhase26Cost`: ladder off = no count and no row, on = exactly one. The Escalation screens cost only the pipeline's two lookups. A Warnings timed press adds only the cached bot-admin check its right already needed.
- `TestPhase26Codes`, `TestPhase26ReadsCreateNothing`.
- `settingsPrivacyPhase26Screens` and `settingsPrivacyPhase26Run` in `TestSettingsStaffPrivacy` (8 languages, group and private views, and a run by a non-creator admin of a linked group).
- AGENTS.md has a Testing paragraph for all of it. Every `applyTimedPunishment` sentence already states the 30 s HTTP client bound.

**Passed:** all the tests above, `make check-translations`, `make check-docs`, `CGO_ENABLED=0 go build ./...`. `go.mod` and `go.sum` unchanged. The guard goldens at `92f29f1` are unchanged.

**Not run (tools absent on this machine):** gosec, golangci-lint, `make generate-docs` was a no-op (no drift). The `TestEventPMCaptcha` and `TestLegacyScreeningNoticeRequests/captcha_*` failures are `chk_captcha_expires_at` and occur on the tree without my changes too.
