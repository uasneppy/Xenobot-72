---
phase: 26-timed-punishments-escalation-and-warn-expiry
plan: 08
subsystem: db, escalation
status: complete
tags: [escalation, offences, migration, cache]
requires:
  - 26-07
provides:
  - migration 20261122140000 (chat_escalation, chat_offences)
  - models.ChatEscalation, models.ChatOffence
  - fuku/db/escalation (Get, GetFresh, SetEnabled, SetEnabledWithSteps, SetStep, SetForgetDays, CountOffence, ClearOffences, ValidStep)
  - chat_escalation: prefix in skipLocal
requirements: [TUNE-02]
---

# Plan 26-08: ladder storage and offence counter

Started from the saved `26-08-WIP.patch` (the earlier executor was cut off before verifying). The patch was reviewed
and applied, its tests were run, and the harness and backup parts it lacked were added. The patch file is removed.

## What changed
- Migration with inline named constraints, two models with matching check tags, the `escalation` repository
  (column-targeted upserts that drop the key; `CountOffence` as one `INSERT ... ON CONFLICT ... RETURNING`).
- `chat_escalation:` in `skipLocal` with its cache test; Makefile `test-postgres-integrity` lists the new tests and package.
- Both models in the `fuku/modules` harness and the `fuku/db/backup` testmain; `TestEscalationStaysOutOfBackup`.
- AGENTS.md: two Data bullets and the Testing PostgreSQL entry.

## Verification
Run: `TestEscalationSettings`, `TestOffenceCountRules` (SQLite), `TestEscalationMigrationStatements`,
`TestSkipLocal*`, `TestEscalationStaysOutOfBackup`, the backup round-trip tests, `go vet -tags testtools ./fuku/modules`,
`CGO_ENABLED=0 go build ./...`, `make check-docs`, golden diff at 92f29f1, `go.mod`/`go.sum` unchanged.

Not run on this machine (no Docker daemon, no PostgreSQL, no golangci-lint or gosec installed):
`TestEscalationMigrationDefaults` (skipped without PostgreSQL), `TestOffenceCountConcurrent` (registers its case on
PostgreSQL only, so it passed on SQLite without exercising concurrency), `make test-postgres-integrity`, the lint and
gosec gates. These must run before Phase 26 closes (plan 26-12).
