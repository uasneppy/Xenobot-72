---
phase: quick-261124-v7y
plan: 01
subsystem: infra
tags: [go-module, rename, redis-keys, prometheus, sql-migrations, docker, ci, docs, cleanup]

requires:
  - phase: 30-backup-page-and-custom-messages-in-backups
    provides: Completed v1.2 Xenobot-72 codebase with 8 locales, Blume docs, and append-only checksummed SQL migrations
provides:
  - Go module renamed to github.com/uasneppy/Xenobot-72 (and github.com/uasneppy/Xenobot-72/scripts/check_translations) with all packages under xenobot/
  - Clean-slate Xenobot-72 naming across Redis keys (xenobot:*), GORM metric callbacks, PostgreSQL advisory locks, Prometheus metrics (xenobot_*), test env vars (XENOBOT_TEST_*), OTEL default service name (xenobot-72), and locale key help_button_add_xenobot
  - Append-only idempotent forward SQL migration migrations/20261124120000_rename_staff_role_lock_prefix.sql switching staff_role_exclusivity_guard() to 'xenobot:staff_role:' while preserving historical checksummed migrations byte-for-byte
  - Repository cleanup removing research spikes, unused PaaS manifests, debug.docker-compose.yml, .claude/, and archived .planning/ history while preserving active GSD state
  - Synchronized Dockerfiles, docker-compose.yml, .goreleaser.yaml, CI workflows, Makefile, scripts, Blume docs, README.md, README.uk.md, and AGENTS.md
affects: [deployments, ci, docs, database-migrations, redis-cache]

tech-stack:
  added: []
  patterns:
    - "Append-only forward migration 20261124120000_rename_staff_role_lock_prefix.sql preserves SHA-256 checksum immutability of already-applied migrations"
    - "Unified xenobot: namespace across Redis cache/operational keys, GORM callbacks, and PostgreSQL advisory locks"

key-files:
  created:
    - migrations/20261124120000_rename_staff_role_lock_prefix.sql
  modified:
    - go.mod
    - main.go
    - main_test.go
    - xenobot/main.go
    - xenobot/main_test.go
    - xenobot/db/migrations/runner.go
    - xenobot/db/metrics.go
    - xenobot/utils/metrics/metrics.go
    - xenobot/utils/tracing/tracing.go
    - xenobot/modules/help.go
    - locales/en.yml
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml
    - locales/id.yml
    - locales/pt.yml
    - locales/ru.yml
    - locales/uk.yml
    - scripts/check_translations/go.mod
    - scripts/check_translations/main.go
    - scripts/check_test_results/main.go
    - scripts/generate_docs/main.go
    - scripts/bump_version.sh
    - scripts/migrate_psql.sh
    - Makefile
    - .goreleaser.yaml
    - .golangci.yml
    - .gitignore
    - sample.env
    - docker-compose.yml
    - docker/alpine
    - docker/alpine.debug
    - docker/goreleaser
    - docker/pr-build
    - .github/workflows/ci.yml
    - .github/workflows/docs.yml
    - .github/workflows/release.yml
    - formal/antiflood/FloodReset.tla
    - docs/blume.config.ts
    - docs/package.json
    - docs/wrangler.jsonc
    - README.md
    - README.uk.md
    - AGENTS.md
    - .planning/config.json
    - .planning/PROJECT.md
    - .planning/STATE.md

key-decisions:
  - "Preserved LICENSE and historical SQL migrations 20261004130000_add_staff_role_exclusivity_trigger.sql and 20261120110000_add_chat_event_messages.sql byte-for-byte so schema_migrations SHA-256 verification never breaks upgraded databases"
  - "Shipped idempotent forward SQL migration 20261124120000_rename_staff_role_lock_prefix.sql replacing staff_role_exclusivity_guard() in place with 'xenobot:staff_role:' so both fresh installs and upgraded DBs share the new lock prefix"
  - "Pointed .planning/config.json claude_md_path to ./AGENTS.md after removing the checked-in .claude/ directory"

patterns-established:
  - "All Go packages live under github.com/uasneppy/Xenobot-72/xenobot/ with xenoerrors import alias"
  - "All Redis keys, GORM metric callbacks, and PostgreSQL advisory locks use the xenobot: prefix; Prometheus metrics use xenobot_*"

requirements-completed:
  - QUICK-261124-V7Y

commits: 3
plan_head_before: 206942b0803ecf523f0747c872e732ad97d56ea7
plan_head_after: 97b708a5ba092d99302f733c30791975a1b227d7
duration: 17min
completed: 2026-11-24
status: complete
---

# Quick Task 261124-v7y: Replace Fuku and Alita Mentions Everywhere Summary

**Full clean-slate rename of the Go module (`github.com/uasneppy/Xenobot-72`), `xenobot/` package tree, `xenobot:` Redis/lock/callback namespace, `xenobot_*` Prometheus metrics, `XENOBOT_TEST_*` env vars, `help_button_add_xenobot` locale key, forward SQL trigger migration, Docker/CI/GoReleaser/docs manifests, and repository cleanup removing spikes, unused PaaS files, `.claude/`, and archived `.planning/` clutter**

## Performance

- **Duration:** 17 min
- **Started:** 2026-11-24T20:36:15Z
- **Completed:** 2026-11-24T20:53:15Z
- **Tasks:** 3
- **Files modified:** 3,122 across 3 atomic commits (1,894 in Task 1 cleanup, 1,181 in Task 2 codebase/locale/migration rename, 47 in Task 3 infra/CI/docs/rulebook updates)

## Accomplishments

- **Task 1 — Repository cleanup (`7b26e86b`):** Removed `scripts/chat-spike/`, `scripts/claude-spike/`, `Procfile`, `app.json`, `nixpacks.toml`, `railway.toml`, `render.yaml`, `debug.docker-compose.yml`, `.claude/`, and archived `.planning/` directories/files while preserving active GSD state (`STATE.md`, `ROADMAP.md`, `PROJECT.md`, `config.json`, `state.json`, and the active `261124-v7y` quick directory). Updated `.planning/config.json` (`claude_md_path: "./AGENTS.md"`) and scrubbed legacy paths from `.planning/PROJECT.md` and `.planning/STATE.md`.
- **Task 2 — Codebase, locales, scripts, and forward SQL migration (`85d697cd`):** Renamed `fuku/` -> `xenobot/`, updated `go.mod` and `scripts/check_translations/go.mod` to `github.com/uasneppy/Xenobot-72` with zero dependency changes (`go.sum` untouched), renamed `fukuerrors` -> `xenoerrors`, migrated all Redis prefixes, GORM metric callbacks, and PostgreSQL migration advisory lock to `xenobot:`, renamed Prometheus metrics to `xenobot_*`, renamed `FUKU_TEST_*` env vars to `XENOBOT_TEST_*`, set default `OTEL_SERVICE_NAME` to `xenobot-72`, renamed `help_button_add_fuku` -> `help_button_add_xenobot` across all 8 locales, scrubbed legacy test fixtures and spike comments, and added `migrations/20261124120000_rename_staff_role_lock_prefix.sql` while keeping `migrations/20261004130000_add_staff_role_exclusivity_trigger.sql` and `migrations/20261120110000_add_chat_event_messages.sql` byte-for-byte immutable.
- **Task 3 — Infra, Docker, CI, docs, READMEs, and `AGENTS.md` (`97b708a5`):** Updated `Makefile`, `.goreleaser.yaml`, `.golangci.yml`, `.gitignore`, `sample.env`, `docker-compose.yml`, all four `docker/*` Dockerfiles (preserving non-root UID/GID `65532:65532` and `USER nonroot`), `.github/workflows/{ci,docs,release}.yml`, `scripts/bump_version.sh`, `scripts/migrate_psql.sh`, `formal/antiflood/FloodReset.tla`, Blume docs config & all 21 hand-maintained docs pages, `README.md`, `README.uk.md`, and `AGENTS.md`. Regenerated docs with `make generate-docs` and verified zero drift via `make check-docs`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Clean up research spikes, unused PaaS manifests, .claude/, and archived .planning/ history** - `7b26e86b` (chore)
2. **Task 2: Rename Go module, fuku/ -> xenobot/, Redis keys, metrics, env vars, locales, and add forward SQL migration** - `85d697cd` (refactor)
3. **Task 3: Rename infra, Docker, CI, GoReleaser, Makefile, docs, READMEs, AGENTS.md, regenerate docs, and run final audit** - `97b708a5` (docs)

_Note: `.planning/STATE.md` (legacy path scrub) and `.planning/quick/261124-v7y-replace-fuku-and-alita-mentions-everywhe/` files are intentionally left unstaged for the orchestrator's final quick-task state update and docs commit._

## Verification Summary

- `CGO_ENABLED=0 go build ./...` — PASS
- `make check-translations` — PASS (1,068 translation keys verified across all 8 locales: `en`, `es`, `fr`, `hi`, `id`, `pt`, `ru`, `uk`)
- `make check-docs` — PASS (zero drift after `make generate-docs`)
- `make test-shard-check` — PASS (exact match on `./xenobot/modules`)
- `TZ=UTC go test -tags testtools -race -count=1 . ./xenobot ./xenobot/config ./xenobot/db/... ./xenobot/i18n ./xenobot/utils/... ./scripts/...` and `(cd scripts/check_translations && go test -race -count=1 ./...)` — PASS (all packages green)
- `node --test scripts/captcha-page/captcha-page.test.mjs` — PASS (24/24 tests)
- Targeted `./xenobot/modules` suites (`TestSettingsDeletionPhase16ScreensUnchanged`, `TestBotCommandNameMatchesGotgbot`, `TestRaidFloodItemKey`, `TestHelp*`, `TestLegacy*`, `TestPunishmentNotice*`, `TestRulesLegacy*`, `TestBlacklistsLegacy*`, `TestFiltersLegacy*`, `TestNotesLegacy*`, `TestWarnsLegacy*`, `TestModTuning*`, `TestSettingsEditPreview*`, `TestStaffPicker*`, `TestStaffPanel*`, `TestStaffUnlink*`, `TestAntiRaidRetired*`, `TestSettingsBackupPage*`, `TestSettingsCopy*`, `TestCaptchaWeb*`, `TestPhase30Codes*`) — PASS
- `bash -n scripts/bump_version.sh scripts/migrate_psql.sh` — PASS
- Byte-for-byte immutability check (`git diff 206942b0 -- LICENSE migrations/20261004130000_add_staff_role_exclusivity_trigger.sql migrations/20261120110000_add_chat_event_messages.sql`) — empty diff (untouched)
- Clean-slate grep audit (`git grep -i -e 'fuku' -e 'alita' -- ':!LICENSE' ':!migrations/20261004130000_add_staff_role_exclusivity_trigger.sql' ':!migrations/20261120110000_add_chat_event_messages.sql' ':!.planning/quick/261124-v7y-replace-fuku-and-alita-mentions-everywhe/'`) — exit code 1 (0 matches; zero natural-language false positives)

## Decisions Made

None - followed locked user decisions in `261124-v7y-CONTEXT.md` and plan specifications exactly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated bot_command entity lengths in command-deletion and raid-flood tests after bot username rename**
- **Found during:** Task 2 (codebase rename)
- **Issue:** Renaming `/rules@FukuTestBot` (18 chars) to `/rules@XenobotTestBot` (21 chars) in `xenobot/modules/command_deletion_test.go` and `/cmd@fuku_bot` to `/cmd@xenobot_bot` (16 chars) in `xenobot/modules/raid_flood_test.go` invalidated hardcoded Telegram `bot_command` entity lengths (`botCmd(18)` and `14`).
- **Fix:** Updated entity lengths to `botCmd(21)` and `16` respectively.
- **Files modified:** `xenobot/modules/command_deletion_test.go`, `xenobot/modules/raid_flood_test.go`
- **Verification:** `TestBotCommandNameMatchesGotgbot` and `TestRaidFloodItemKey` pass under `-tags testtools -race`.
- **Committed in:** `85d697cd` (Task 2 commit)

**2. [Rule 1 - Bug] Recomputed Phase 16 deletion golden screen SHA-256 digests for renamed fixture chat title**
- **Found during:** Task 2 (module golden test verification)
- **Issue:** `TestSettingsDeletionPhase16ScreensUnchanged` hashes the rendered screen text including the fixture group title (`"Fuku Test Group"` -> `"Xenobot Test Group"`), causing all 76 screen digests in `settingsDelGoldenDigests` to change even though renderer output is otherwise identical.
- **Fix:** Recomputed and updated the 76 SHA-256 digests in `xenobot/modules/settings_deletion_golden_test.go` using the test's own sorted digest output for `"Xenobot Test Group"`.
- **Files modified:** `xenobot/modules/settings_deletion_golden_test.go`
- **Verification:** `TestSettingsDeletionPhase16ScreensUnchanged` passes under `-tags testtools -race`.
- **Committed in:** `85d697cd` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 test fixture consistency fixes)
**Impact on plan:** Both fixes were strictly required by the test fixture renames in Task 2; zero production renderer or logic drift.

## Issues Encountered

- Running `./xenobot/db/captcha` in local macOS timezone (`+02:00`) triggered a pre-existing SQLite `CHECK constraint failed: chk_captcha_expires_at` failure (SQLite `CURRENT_TIMESTAMP` is UTC whereas local-zone timestamp comparisons differ by 2 hours). Running with `TZ=UTC` (matching CI runners) passed all captcha DB tests cleanly in 1.25s.

## User Setup Required

- On next deploy, flush Redis (or allow old keys to expire) because all Redis key prefixes moved from `fuku:` to `xenobot:` (pre-confirmed by user in `261124-v7y-CONTEXT.md`).
- Update any external Docker/Compose overrides, Prometheus scrape/alert rules, systemd/launchd service configs, or CI secrets/scripts that still reference `ghcr.io/uasneppy/fuku_robot`, `fuku_robot`, `fuku_*` metrics, or `FUKU_TEST_*` environment variables.

## Next Phase Readiness

- Repository builds, tests, translates, and generates docs cleanly under `github.com/uasneppy/Xenobot-72` with zero remaining legacy brand mentions outside `LICENSE`, the two checksum-verified historical SQL migrations, and this quick task's planning directory.
- Ready for orchestrator Step 7 (`STATE.md` quick-task table append) and Step 8 (quick-task docs commit).

---
*Phase: quick-261124-v7y*
*Completed: 2026-11-24*

## Self-Check: PASSED

- All key files verified present on disk (`go.mod`, `xenobot/main.go`, `migrations/20261124120000_rename_staff_role_lock_prefix.sql`, `.planning/config.json`, `AGENTS.md`, `261124-v7y-SUMMARY.md`).
- All 3 task commits verified in `HEAD` ancestry (`7b26e86b`, `85d697cd`, `97b708a5`).
- `LICENSE`, `migrations/20261004130000_add_staff_role_exclusivity_trigger.sql`, and `migrations/20261120110000_add_chat_event_messages.sql` confirmed byte-for-byte untouched.
- Working tree contains only `.planning/STATE.md` (legacy path scrub) and `.planning/quick/261124-v7y-replace-fuku-and-alita-mentions-everywhe/` ready for orchestrator Step 7/Step 8.
