# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — PM Settings Hub & Member Screening

**Shipped:** 2026-11-13
**Phases:** 10 | **Plans:** 126 | **Sessions:** not tracked (the autonomous run spanned many context windows, each continued from a summary)

### What Was Built
- A private-chat settings hub gated by the group's owner, with a roster of manageable groups, a Main grid of 18 pages and one press pipeline (live `getChatMember`, owner gate, per-control right) that every page and every typed reply goes through.
- A message editor for Welcome, Goodbye and Rules (text, media, buttons, previews, media under text, greet once) on a Redis-backed typed-reply door.
- Member requirements: name, photo, channel and AI spam-name rules with per-rule punishments, one notice per episode and an "I fixed it" button.
- A durable deletion queue for service messages, the bot's own messages, members' commands, edits and self-destruct.
- Pages for warnings, switches, language, log channel, anonymous admins and locks, then blacklist, filters and notes on one list engine with a generic two-step add.

### What Worked
- One apply function per setting, with a characterization golden committed alone before each extraction, plus parity, drift and authority-parity tests: no page ever drifted from its command, and the goldens were never re-recorded.
- Gating every plan boundary myself (build, gofmt, vet, focused tests under the CI environment and bare, the lint stand-in, a skip grep, `go.mod`) caught what executor reports missed: a hard-coded Main-entry count, a settings callback that bypassed its apply function (Phase 18), a pager code that collided with a Deletion page code (Phase 19).
- Executors that commit after every sub-step survived two container restarts mid-plan; the work was resumed from the working tree.
- Provisional owner defaults recorded as owner confirmations, each owner-visible command change in its own revertable commit, and every rule written into AGENTS.md in the same commit as the code.
- A separate audit pass per phase (security, verification, then an integration check across phases) found nothing blocking and one low finding that was fixed.

### What Was Inefficient
- Test time: `fuku/modules` grew from about 600 s to about 1,900 s locally and 28 to 30 minutes on CI against a 40 minute limit; every plan added 2 to 16 s and had to be argued down.
- Plan text went stale against the code (counts of codes, apply functions and keys, a pager code); executors and I had to count from the registry each time.
- Each push cancels the in-flight CI run, so CI verified plan boundaries only, and a cancelled Test Suite repeatedly produced a "CI Success failure" notice that had to be checked by hand.
- Sandbox limits: `make lint` and gosec cannot run here (golangci-lint is built with Go 1.25), disk filled once, and the write guard's documented milestone-reset sentinel did not work because the guard is registered twice.
- Ten phases of Telegram behaviour were verified only on a faked client; the live checks all sit with the owner.

### Patterns Established
- Characterization golden first, apply function second, owner-visible change third, each its own commit.
- Unknown never punishes and never deletes; lookups that cannot be answered fail closed; caches hold positives only.
- Menus are stateless, authority is a live `getChatMember` on every press, and a group menu never reads private data.
- Counts and codes are asserted from the registry, never from prose; sweeps keep only the worst few of a notice and the full product only where the keyboard changes.
- Deferred live checks are written as a numbered `NN-SPIKE.md` per phase and indexed in STATE.md.

### Key Lessons
1. Run a short live-check pass after every two or three phases of the next milestone instead of deferring all of them; unverified Telegram behaviour compounds across phases.
2. Give every plan a test-time budget up front and measure it per plan, because the cost of the last 10 percent of a 40 minute limit is a whole class of work.
3. Have the orchestrator re-verify an executor's numbers (counts, codes, byte sizes) against the code rather than the plan or the report.
4. Commit after each sub-step and keep executor briefs short and explicit about what never to push or amend.
5. Treat guards that are registered in two places (a hook in both the user and project settings) as one broken hatch and fix the registration before relying on a single-use sentinel.

### Cost Observations
- Model mix: not measured. Planning and execution ran on the balanced profile for most phases and on the budget profile (sonnet executors, haiku verifier and integration checker) from Phase 19.
- Sessions: not tracked.
- Notable: the whole-package test run (about 33 minutes) dominates wall-clock time per plan; executors spent most of their time waiting on it.

---

## Milestone: v1.2 — Conversations, Custom Messages and Tuning

**Shipped:** 2026-11-24
**Phases:** 11 | **Plans:** 87 | **Sessions:** not tracked (spanned Claude Code & Antigravity sessions)

### What Was Built
- Group Conversations (Phase 20): owner-enabled in-character AI replies in groups via Anthropic or Google, never in private chat.
- Per-group Messages engine & Messages page (Phases 21, 23–25): one shared renderer with default fallback, Silent switch, and MSG-11 characterization goldens covering manual punishments, warn/warn-limit, antiflood, blacklist, forward blocklist, failed captcha, rate-limited AI spam notice, member requirements, and lockdowns.
- Optional fleet-paced PM copies (Phase 22): best-effort private copies with a per-group automatic cap, Details deep-link fallback, and `#PM_COPY` log line, excluded from raid-scale removals and Staff Group actions.
- Punishment & threshold tuning (Phases 26–29): timed bans/mutes for warn limit, blacklist and antiflood; escalation ladder; warn expiry; manual command defaults, reason shortcuts, and ban appeal button; 12 advanced thresholds inside safe bounds; per-group plain tone across all 8 languages; and live-checked Copy Settings between owned groups.
- Backup page & custom messages in backups (Phase 30): private-only Backup page reusing `/export`, `/import`, and `/reset` behind two-tap cards, with custom messages validated by shared `eventmsg.ValidateWrite`.

### What Worked
- MSG-11 golden-first discipline: freezing every baseline send site in its own commit before touching production code kept unconfigured chats byte-identical across all 11 phases.
- Sharded CI (`fuku/modules` split 1/4..4/4) kept full `-race` test runs under ~5 minutes per shard instead of hitting the 40-minute ceiling from v1.1.
- Running full code reviews (`21-REVIEW.md`..`30-REVIEW.md`) and fixing every actionable warning before merging PR #23 caught subtle cross-phase bugs (shared `eventmsg.ValidateWrite` alignment, media sender dedup, Details deep-link fallback logging, action-code collisions).
- Keeping `go.mod` unchanged (zero new dependencies) and keeping `AGENTS.md` updated in the same commits as the code.

### What Was Inefficient
- Code-review fixes across Phases 21–28 were applied in bulk at the end of the milestone rather than immediately after each phase, which caused a few CI regressions (`dupl`, `errcheck`, and two outdated test assertions) that had to be reconciled before PR #23 went green.
- Live Telegram checks for Phases 21–27 were again deferred (`human_needed`), carrying the same accumulation risk noted in v1.1.

### Patterns Established
- Shared pure-Go validation packages (`fuku/db/eventmsg/validate_write.go`) consumed identically by settings editors, Copy Settings, and Backup imports with zero circular dependencies.
- One-time cross-replica Redis confirmation tokens consumed atomically via Lua `GET+DEL` for destructive or cross-group operations (Copy Settings, Backup Import/Reset).
- Parallel phase code reviews + targeted fix passes followed by full PR CI verification before merging to `main`.

### Key Lessons
1. Run `/gsd-code-review` + `--fix` right after each phase (while its tests and context are fresh) instead of batching 8 phases at the end of a milestone.
2. When a review fix changes allowed behavior (e.g., enabling Media on requirement PM copies), grep safety/forged-press tests in the same commit so CI never sees stale assertions.
3. Keep live Telegram check passes scheduled after every 2–3 phases so deferred checklists don't grow across milestones.

### Cost Observations
- Model mix: budget/balanced profiles across planning, execution, verification, security audits, and code reviews.
- Sessions: not tracked.
- Notable: sharded CI and in-process `net.Pipe()` miniredis tests cut local and CI iteration time significantly compared to v1.1.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | not tracked | 9 | Staff Group, lockdowns, Turnstile captcha, first settings menu |
| v1.1 | not tracked | 10 | Golden-first apply functions, per-boundary orchestrator gates, deferred live checks as SPIKE files |
| v1.2 | not tracked | 11 | Sharded CI (`fuku/modules` 4x), MSG-11 goldens per phase, full security + code-review-fix pass before merge |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | not recorded | not recorded | 0 |
| v1.1 | about 1,840 top-level tests in `fuku/modules` plus the repository packages (about 201,000 lines of test code) | not measured here | 0 (`go.mod` unchanged against the v1.0 closing commit) |
| v1.2 | ~265,900 lines of Go test code across 14/14 green CI checks (4 sharded `fuku/modules` suites + other packages + gosec + golangci-lint) | not measured here | 0 (`go.mod` unchanged) |

### Top Lessons (Verified Across Milestones)

1. Live checks deferred to the owner accumulate; both milestones ended with a long checklist (v1.0: 10 of 11 UAT checks, v1.1: 10 SPIKE files).
2. A rulebook kept in the same commits as the code (`AGENTS.md`) stayed true enough to audit against; it is the cheapest guard against drift.
