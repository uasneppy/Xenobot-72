---
phase: 29-plain-tone-and-copy-settings
plan: 05
subsystem: copy-settings-ui
tags: [copy-settings, tune-09, settings-page, redis-lua, dual-group-auth, opaque-refusal, i18n]

requires:
  - phase: 29-plain-tone-and-copy-settings
    plan: 04
    provides: Copy Settings core engine (applyCopySettings) and shared validated eventmsg writer (ReplaceAllValidated)
provides:
  - 12 Copy Settings UI locale keys across all 8 languages (en, es, fr, hi, id, pt, ru, uk) with ceilings, alert checks, and label-length guards in fuku/i18n/settings_locale_test.go
  - Private-only Copy Settings page cp (Order 82, Icon 📋, PrivateOnly true, hidden in Staff Groups, static zero-I/O MainState) with actions cg (paginate), ck (pick target), and cy (confirmed Yes) in fuku/modules/settings_copy.go
  - Live dual-group verification at both ck and cy with uniform opaque target refusal (settings_pick_unavailable) across all 6 refusal causes and P29-02 non-creator admin dual-right enforcement (can_change_info && can_restrict_members)
  - Pair-bound 96-bit 16-char base64url Redis confirmation token under fuku:settings:copy:<token> (5m TTL, SET NX, fail-closed on nil Redis) consumed atomically via Lua GET+DEL script
  - Main/Phase-19 grid test updates and comprehensive TestSettingsCopyPage test suite (6 subtests using in-process miniredis) in fuku/modules/settings_copy_test.go
affects: [29-06, 30]

tech-stack:
  added: []
  patterns:
    - "Atomic Lua GET+DEL confirmation token consumption under fuku:settings:copy:<token> (outside fuku:cache:) with peek-and-bind verification before delete so cross-user or cross-source presses cannot invalidate a valid owner's card"
    - "Uniform opaque target refusal (settings_pick_unavailable) on both ask (ck) and confirmed Yes (cy) across bot-not-in-chat 4xx, non-admin, owner-only gate, missing target write rights, non-group type, and Staff Group checks"

key-files:
  created:
    - fuku/modules/settings_copy.go
    - fuku/modules/settings_copy_test.go
  modified:
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
    - fuku/i18n/settings_locale_test.go
    - fuku/modules/settings_main_test.go
    - fuku/modules/settings_phase19_test.go

key-decisions:
  - "Filtered definite Staff Groups out of the picker candidate list in settingsCopyLoadCandidates while keeping full fresh non-Staff-Group enforcement (failing closed on DB error) inside both ck and cy live checks."
  - "Used a 16-char unpadded base64url token (12 crypto/rand bytes) under fuku:settings:copy:<token> with TTL settingsCardLifetime (5m), verifying UserID and SourceChatID via GET before executing the atomic Lua GET+DEL consume script so wrong-user/wrong-source callbacks cannot burn the real owner's card."
  - "Kept settings_btn_copy_yes within 18 runes across all 8 languages and registered it in TestSettingsLabelLengths editorButtons (<= 20) so Yes and Cancel share one clean two-column action row on the confirmation card."

patterns-established:
  - "Private-only cross-group action pattern: step-1 private-field & canonical ID/token validators (0 Telegram calls on forged/group presses) + live dual-group verification at ask and confirm + uniform opaque settings_pick_unavailable alert."
  - "In-process miniredis settings page test pattern: newPhase28SettingsEnv backed by phase28InProcessMiniredis with json.Decoder.UseNumber() title wrapper for exact int64 chat ID fidelity."

requirements-completed: [TUNE-09]

status: complete
plan_head_before: ae925b8e97a4e6a2719141b9e8975940fbe1b123
plan_head_after: 567f805965c643b90a2c2475d9a9e7c0180eed4a
commits: 3

actuals:
  tokens: 21625
  tasks: 3
  commits: 3

duration: 25min
completed: 2026-11-23
---

# Phase 29 Plan 05: Copy Settings Private Page (`cp`), Live Dual-Group Auth & 8-Locale UI Summary

**Private-only Copy Settings page `cp` (Order `82`, Icon `📋`, actions `cg`/`ck`/`cy`) with live dual-group verification at ask and Yes, uniform opaque `settings_pick_unavailable` target refusals, pair-bound 16-char base64url Redis confirm cards consumed atomically via Lua `GET+DEL`, full 8-language UI strings, updated Main/Phase-19 grid tests, and 6-subtest `-race` test coverage**

## Performance

- **Duration:** 25 min
- **Started:** 2026-11-23T14:46:07Z
- **Completed:** 2026-11-23T15:10:59Z
- **Tasks:** 3
- **Files modified:** 13 (2 created, 11 modified; `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, and `go.sum` untouched)

## Accomplishments

- **8-Language Copy Settings Locale Keys (`eb921fb2`):**
  - Added `settings_page_copy`, `settings_state_copy`, `settings_copy_group_note`, `settings_copy_intro`, `settings_copy_empty`, `settings_copy_confirm_head`, `settings_copy_confirm_areas` (explicitly naming all 6 overwritten areas), `settings_copy_confirm_keep` (explicitly naming excluded tables/content + one-time non-linking sentence), `settings_btn_copy_yes`, `settings_notice_copied`, `settings_notice_copied_skipped`, and `settings_copy_unavailable` across `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, and `uk`.
  - Registered all 12 keys in `settingsLocaleLimits`, added `settings_copy_unavailable` to `settingsAlertKeys`, added `{"📋", "settings_page_copy"}` to `TestSettingsLabelLengths` `pages`, and added `settings_btn_copy_yes` to `editorButtons`.
  - Passed `TestSettingsLocaleKeys`, `TestSettingsLabelLengths`, `TestEventMsgLocaleKeys`, `TestLocaleStyle`, `TestLocalePlaceholdersMatchOtherLanguages`, `TestSettingsReusedKeyPlaceholders`, and `make check-translations`.
- **Private Copy Settings Page `cp` & Grid Tests (`ffcbd56a`):**
  - Implemented `fuku/modules/settings_copy.go` registering page `settingsPageCopy = "cp"` at Order `82` (`Icon: "📋"`, `PrivateOnly: true`, `PrivateNoteKey: "settings_copy_group_note"`, `Hidden: settingsCopyHidden`, static zero-I/O `MainState: settingsCopyMainState`) with actions `cg` (paginate), `ck` (pick target), and `cy` (confirmed Yes).
  - Enforced step-1 private-only & canonical argument validation (`c != ""`, `1 <= v <= 999`, negative canonical `t != c`, strict 16-char base64url `k`) so forged or group-menu presses fail with 0 `getChatMember` calls.
  - Enforced full live dual-group verification on both `ck` and `cy`: fresh non-Staff-Group check on source (failing closed on DB error), live `lockdownLiveMember`, `settingsGate`, P29-02 non-creator admin dual rights (`CanChangeInfo && CanRestrictMembers`), live `GetChatWithContext` (`group`/`supergroup` only), and fresh non-Staff-Group check on target — mapping every target refusal to the uniform opaque `settings_pick_unavailable` alert.
  - Added pair-bound Redis confirmation tokens (`fuku:settings:copy:<token>`, 96-bit `crypto/rand`, 5m `settingsCardLifetime`, `SET NX`, fail-closed `settings_copy_unavailable` when Redis is nil), `GET` bind check before atomic Lua `GET+DEL` consumption, HTML-escaped group titles via `html.EscapeString`, and `applyCopySettings` execution.
  - Updated `fuku/modules/settings_main_test.go` (`settingsMainShapeGroup`, `settingsMainShapePrivate`, `settingsMainTitleKeys`, keeping `settingsMainOwnerRows = 16` and `staffOwnerRows = settingsMainOwnerRows - 3`) and `fuku/modules/settings_phase19_test.go` (adding `"cp"` to `want`).
- **Comprehensive Copy Settings Test Suite (`567f8059`):**
  - Created `fuku/modules/settings_copy_test.go` (`TestSettingsCopyPage`, backed by `phase28InProcessMiniredis`) with all 6 subtests:
    1. `open_private_and_group_note_and_staff_hidden`
    2. `confirm_card_names_all_areas_and_executes_copy`
    3. `replay_expired_wrong_user_wrong_source_and_no_redis`
    4. `live_dual_group_auth_and_uniform_opaque_refusal` (all 8 target refusal variants at both `ck` and `cy`, transient 429/500, and positive P29-02 dual-right admin check)
    5. `worst_case_64_byte_callbacks` (`math.MinInt64` across `op`, `cg`, `ck`, `cy`, `no`)
    6. `all_8_languages_screen_lengths` (64-char `&`/`😀`/`Ж` titles across all 8 locales for picker, empty picker, confirm card, and copied notices)

## Task Commits

1. **Task 1: add Copy Settings locale keys across all 8 languages and update settings_locale_test.go** - `eb921fb2` (`feat(29-05): add Copy Settings locale keys across all 8 languages`)
2. **Task 2: implement private Copy Settings page (cp) and update Main/Phase-19 grid tests** - `ffcbd56a` (`feat(29-05): implement private Copy Settings page cp and update grid tests`)
3. **Task 3: add comprehensive Copy Settings page UI, dual-group auth, opacity, replay, 64-byte, and 8-language tests** - `567f8059` (`test(29-05): add Copy Settings UI, dual-group auth, opacity, replay, and 8-locale tests`)

## Files Created/Modified

- `locales/{en,es,fr,hi,id,pt,ru,uk}.yml` — Added 12 Copy Settings keys across all 8 languages.
- `fuku/i18n/settings_locale_test.go` — Added ceilings, alert check, grid title length check, and confirm Yes button length check.
- `fuku/modules/settings_copy.go` — Private Copy Settings page `cp` (Order `82`), `cg`/`ck`/`cy` actions, live dual-group verification, uniform opaque target refusal, Redis one-time token store + Lua `GET+DEL` consume script, and pure renderers.
- `fuku/modules/settings_main_test.go`, `fuku/modules/settings_phase19_test.go` — Updated Main grid shapes (`av|cp`), `settingsMainTitleKeys`, and Phase 19 registered page list.
- `fuku/modules/settings_copy_test.go` — `TestSettingsCopyPage` (6 subtests) verifying UI, HTML escaping, 6-area mirror, Redis Lua replay/expiry/isolation, live dual-group auth & uniform opacity, 64-byte callbacks, and 8-language UTF-16 screen lengths.

## Decisions Made

- Used `fuku/utils/cache` (`cache.IsRedisAvailable()` / `cache.GetRedisClient()`) for `settingsCopyDefaultRedis` so Copy Settings shares the production Redis pool without triggering raid-detection log messages.
- Verified `rec.UserID` and `rec.SourceChatID` on `GET` before running the Lua `GET+DEL` consume script so a forged cross-user or cross-source callback can never invalidate the real owner's active confirm card.
- Used `json.NewDecoder(...).UseNumber()` in `copyTitleBotClient` inside `settings_copy_test.go` so patching custom `getChat` titles preserves exact 64-bit `int64` chat IDs without float64 rounding.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Pre-existing Main/Phase-19 tests (`TestSettingsMainPhase18`, `TestSettingsMainPhase19`) and `TestLegacyPhase29Baseline` open localhost TCP miniredis sockets, requiring loopback network access (`BypassSandbox: true`, `TZ=UTC`) during verification; `TestSettingsCopyPage` itself uses in-process `phase28InProcessMiniredis` with zero TCP dials.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Private Copy Settings page `cp` and all 8-locale UI strings are complete and verified under `-race`, ready for Plan 29-06 (phase-wide code guard, `AGENTS.md` / docs updates, and full Phase 29 verification).

## Self-Check: PASSED

- Created files exist: `fuku/modules/settings_copy.go`, `fuku/modules/settings_copy_test.go`
- Commits exist and are ancestors of `HEAD`: `eb921fb2`, `ffcbd56a`, `567f8059` (`commits: 3` matching `git rev-list --count ae925b8e97a4e6a2719141b9e8975940fbe1b123..567f805965c643b90a2c2475d9a9e7c0180eed4a`)
- Untouched invariants verified: `fuku/modules/phase29_legacy_request_test.go`, `go.mod`, `go.sum` have zero diffs
- All verification suites passed under `-race`: `./fuku/i18n` (6 tests), `make check-translations`, `TestSettingsCopyPage` (6/6 subtests), `TestSettingsMainGrid`, `TestSettingsMainPhase18`, `TestSettingsMainPhase19`, `TestSettingsMainPageFitsEveryLocale`, `TestSettingsFwdBlockMainShape`, `TestSettingsPhase19Codes`, `TestPhase28Codes`, `TestEventEngineCallers`, `TestPMCopyCallers`, `TestLegacyPhase29Baseline`, `CGO_ENABLED=0 go build ./...`, and `go vet -tags testtools ./fuku/modules ./fuku/i18n`
