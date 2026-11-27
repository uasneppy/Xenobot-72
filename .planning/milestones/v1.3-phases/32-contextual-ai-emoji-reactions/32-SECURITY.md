---
phase: 32
slug: contextual-ai-emoji-reactions
status: secured
threats_open: 0
asvs_level: 1
block_on: high
---
# Phase 32 — Security

## Threat Verification
| ID | Category | Sev | Disp | Status | Evidence |
|----|----------|-----|------|--------|----------|
| T-32-01 | Tampering | medium | mitigate | CLOSED | `xenobot/db/presence/repository.go:63-65` `ValidAIReactMode`. `SetAIReactMode` (:139-142) returns ErrInvalidValue before `upsert`. DB CHECK: `migrations/20261128120000_add_chat_presence_ai_react_mode.sql` `chk_chat_presence_ai_react_mode`, plus a gorm `check:` tag at `xenobot/db/models/chat_presence.go:16`. |
| T-32-02 | Integrity | medium | mitigate | CLOSED | `repository.go:68-72` `normalizeAIReactMode` is applied in `Get` (:89) and `GetFresh` (:110). Key `chat_presence:<chat>` is built by `CacheKey`. `upsert` calls `cache.DeleteCache(CacheKey)` after every write. |
| T-32-03 | EoP | high | mitigate | CLOSED | `ai_react_mode` exists only in `chat_presence`: the migration and model alter no `chat_conversations`. grep for `ai_react` outside presence files finds nothing. |
| T-32-04 | Tampering (prompt injection) | medium | mitigate | CLOSED | `xenobot/utils/chatprompt/reaction.go`: `ReactionDynamic` (:66-71) uses `Clean` on title and language. `ReactionUser` (:112-135) uses `line()` and `Clean` on every untrusted field. It takes no IDs or usernames. It ends with an untrusted-data reminder (:135). |
| T-32-05 | Tampering | medium | mitigate | CLOSED | `reaction.go:144-186`, `ParseReactionJSON`. It requires a leading `{` and a single JSON object, and rejects trailing tokens. It requires a non-null string `emoji`. It strips U+FE0F (:239). `containsNormalizedEmoji` checks exact allowlist membership. |
| T-32-06 | Tampering/DoS | low | mitigate | CLOSED | `xenobot/i18n/settings_locale_test.go:948-953` has limits for the 6 keys. `settings_pr_ai_need_owner` is also in `settingsAlertKeys` (:987). All 8 locale files contain the 3 `settings_pr_ai_*` keys. |
| T-32-07 | Info disclosure | medium | mitigate | CLOSED | `chat_log.go:148-175` `chatLogRecentThread`: `if entry.Thread != threadID \|\| entry.MessageID >= beforeMsgID { continue }`. |
| T-32-08 | DoS | low | mitigate | CLOSED | `chat_log.go:40` `chatLogRecentScanLimit=50`, and `ZRevRange` is bounded by `chatLogReadTimeout`. `presence_react_api.go:20` `presenceAvailReactTTL=20m`. `presenceAllowedPalette` (:93-126) returns `slices.Clone` and fails closed `(nil,false)` on error or an empty custom-only list. |
| T-32-09 | DoS | high | mitigate | CLOSED | `chat_react.go`: the Lua gap/cooldown gate `chatReactTakeGate` fails closed on error. Probability roll via `chatReactRoll`. Hourly budget `chatReserveWith(...,"react:b",...)` (:481). `chatPresenceQueue` is separate from `chatQueue`, cap 32 (:34,161). `chatPresenceEnqueue` sheds when full (:193-198). |
| T-32-10 | DoS | high | mitigate | CLOSED | `chat_react.go:477` only calls `isPaused`. grep of `chat_react.go` finds no `chatRecordFailure`, `countFailure` or `pauseProvider`. Errors are shed at Debug (:454,515). |
| T-32-11 | Info disclosure | high | mitigate | CLOSED | `chatReactWatch` calls `presenceMessageAllowed` (:263). That function (`presence.go:114-134`) allows only group/supergroup, rejects slash text and bots, and rejects Staff Groups. `chatReactProcessJob` calls `presenceIsLockedDown` (:473). Prompts carry no IDs. Logs carry only chat_id, user_id and an error class. |
| T-32-12 | EoP | high | mitigate | CLOSED | `chat_react.go:255-281`: `chatAnyProviderConfigured` (which requires `cfg.EnableChat`), then `AIReactMode != off`, then `conv.Enabled && chatProviderConfigured(conv.Provider)`. All run before enqueue. |
| T-32-13 | EoP | high | mitigate | CLOSED | `settings_presence.go:409-440` `settingsPresenceSetAIReact` checks `settingsPresenceGroupRefused` and `settingsPresenceStaffGate` first. For low/medium it checks `settingsPresenceConvUnlocked` (:140-148), which reads `chatconv.GetFresh`. On a DB error it calls `cannotOpen`. When Owner Conversations is off it alerts `settings_pr_ai_need_owner`. `off` is always allowed. The control declares `Right: settingsRightChangeInfo`. |
| T-32-14 | Tampering | medium | mitigate | CLOSED | `settings_presence.go:37-39` `settingsPresenceAIReactCodes`. The control's `Validate` checks `v` against that map. The handler re-checks it (:413-416) before any read or write. |
| T-32-SC | Tampering (supply chain) | high | accept | CLOSED (accepted) | Accepted-risk entry below. No `go.mod`/`go.sum` change in Phase 32. |

## Accepted Risks Log
| ID | Risk | Rationale |
|----|------|-----------|
| T-32-SC | New Go dependencies | Plans 32-01, 32-05 and 32-06 add zero new external packages. They use stdlib, GORM, go-redis, and internal packages only. |

## Unregistered Flags
None found. The auditor did not re-read the SUMMARY files for `## Threat Flags`.

## Notes (non-blocking)
- The auditor read code only and ran no tests. The verifier separately ran the race tests (including `TestShutdownEndsChatReact`), lint and translations.
- `chatReactCallProvider`, `chatReactRecentTurns` and `chatReactDownloadVision` were not read line by line; `chat_react.go` was grepped for breaker-mutating calls and none were found.
