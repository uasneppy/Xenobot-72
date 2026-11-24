# Quick Task 261124-v7y: Context & Locked Decisions

**Task:** Replace Fuku and Alita mentions everywhere with Xenobot-72 and cleanup the repo

## Locked Decisions (from user clarification)

### 1. Rename Scope: Full Clean-Slate Rename Everywhere
- Rename Go module in `go.mod` (and any nested `go.mod`, e.g. `scripts/check_translations/go.mod`) from `github.com/uasneppy/fuku_robot` to `github.com/uasneppy/Xenobot-72` and update all imports across the entire repo.
- Rename top-level Go package directory `fuku/` → `xenobot/` and update all package/path references (`fukuerrors` → `xenoerrors`, `fuku/config` → `xenobot/config`, etc.).
- Rename all Redis key prefixes from `fuku:` → `xenobot:` (clean slate; user confirmed OK to flush Redis on next deploy).
- Rename all environment variables and defaults (`FUKU_TEST_DATABASE` → `XENOBOT_TEST_DATABASE`, `FUKU_TEST_MIGRATION_CHAIN` → `XENOBOT_TEST_MIGRATION_CHAIN`, `OTEL_SERVICE_NAME` default `fuku_robot` → `xenobot-72`, and any other `FUKU_*` env vars).
- Rename locale key `help_button_add_fuku` → `help_button_add_xenobot` across all 8 `locales/*.yml` files and all Go callers/tests.
- Rename binary, Docker images/containers/services/DB defaults, GitHub URLs, CI workflows, GoReleaser config, Makefile targets/names, docs site (`docs/`), `README.md`, `README.uk.md`, `sample.env`, `AGENTS.md`, `formal/`, and scripts from `Fuku` / `Fuku_Robot` / `fuku_robot` / `fuku-robot` / `FukuRobot` to `Xenobot-72` / `xenobot-72` / `xenobot` as contextually appropriate.

### 2. Repository Cleanup Scope (All 4 Selected)
1. **Remove one-off research spikes:** Delete `scripts/chat-spike/` and `scripts/claude-spike/` and clean up any references to them in tests, CI, Makefile, docs, or `AGENTS.md`.
2. **Remove unused PaaS deploy files:** Delete `Procfile`, `app.json`, `nixpacks.toml`, `railway.toml`, `render.yaml`, and `debug.docker-compose.yml`, and remove references to them from docs/README/Makefile/AGENTS.md.
3. **Remove `.claude/` and prune archived `.planning/` history:**
   - Delete the checked-in `.claude/` directory (~15 MB) and update `.planning/config.json` (`claude_md_path`) if it points to `./.claude/CLAUDE.md`.
   - Prune archived/completed `.planning/` clutter (e.g., `.planning/milestones/`, old `.planning/quick/` tasks other than `261124-v7y-*`, `.planning/quick-batches/`, `.planning/debug/`, `.planning/research/`, `.planning/ui-reviews/`, `.planning/codebase/`, `MILESTONES.md`, `RETROSPECTIVE.md`), **while preserving active GSD tracking files** (`.planning/STATE.md`, `.planning/ROADMAP.md`, `.planning/PROJECT.md`, `.planning/config.json`, `.planning/state.json`, and `.planning/quick/261124-v7y-replace-fuku-and-alita-mentions-everywhe/`) so GSD workflows continue working cleanly. Verify no tests depend on pruned `.planning/` paths.
4. **Tidy stale files, dead code, and outdated refs:** Clean up stale references, broken links, and outdated paths across `docs/`, `Makefile`, `.github/workflows/`, `README.md`, `README.uk.md`, and `AGENTS.md`. Run `make generate-docs` / `make check-docs` and `make check-translations` as applicable.

### 3. Legal Attribution, SQL Migrations, and Compatibility Tests
- **Keep ONLY `LICENSE`** untouched (MIT copyright notice).
- **Scrub all other `Fuku` and `Alita` references everywhere** in the repo (including removing the `Alita_Robot` fork attribution in `README.md` and `README.uk.md`, replacing `FukuRobot` backup test fixtures/compat checks, and scrubbing `fuku`/`alita` from `xenopersona_test.go`).
- **SQL migrations:** Add a new migration to replace the `'alita:staff_role:'` PostgreSQL advisory lock key prefix in the staff role exclusivity trigger function with `'xenobot:staff_role:'`. Inspect the migration runner (`schema_migrations` / checksum behavior) to see whether the historical migration files (`20261004130000_add_staff_role_exclusivity_trigger.sql` and `20261120110000_add_chat_event_messages.sql`) use checksums; if no checksum validation exists and editing them is safe (or update comments + add forward migration), ensure both fresh installs and upgraded DBs have `'xenobot:staff_role:'` and zero `fuku`/`alita` mentions remain outside `LICENSE` (document clearly if checksums prohibit editing old SQL files).

## Verification Expectations
- `PATH="/opt/homebrew/bin:$PATH"` is needed for `go` and `node` in non-login shells.
- Compile check: `CGO_ENABLED=0 go build ./...`
- Translations check: `make check-translations`
- Docs drift check: `make check-docs` (run `make generate-docs` if needed)
- Targeted Go unit tests across renamed/touched packages (`go test -tags testtools -race ...`, `main_test.go`, `scripts/...`, `xenobot/db/...`, `xenobot/utils/...`, and key `xenobot/modules` tests — note full `make test` takes 20-40 min per `AGENTS.md`, so run fast focused verification + check if full test suite or shards are feasible).
- Final `git grep -i -E '(^|[^a-zA-Z])(fuku|alita)'` verification confirming zero remaining `fuku`/`alita` mentions outside `LICENSE` (and active `.planning/` historical logs if any, or scrub those too if appropriate).
