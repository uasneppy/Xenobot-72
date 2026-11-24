---
status: testing
phase: 26-timed-punishments-escalation-and-warn-expiry
source: [26-VERIFICATION.md]
started: 2026-11-22T15:26:42Z
updated: 2026-11-22T15:26:42Z
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-24
  gap_snapshot: "testing::scenarios=3"
---

## Current Test

number: 1
name: PostgreSQL integrity (T-26-34)
expected: |
  Migrations apply twice, out-of-range checks fail with SQLSTATE 23514, and sixteen concurrent offences count as one.
awaiting: user response

## Tests

### 1. PostgreSQL integrity (T-26-34)
expected: With PostgreSQL and DATABASE_URL set, `make test-postgres-integrity` covers `TestTimedPunishmentMigrationDefaults`, `TestWarnExpiryMigrationDefaults`, `TestEscalationMigrationDefaults`, and `TestOffenceCountConcurrent`. Migrations apply twice, out-of-range checks fail with SQLSTATE 23514, and sixteen concurrent offences count as one.
result: [pending]

### 2. Lint and gosec
expected: `make lint` and gosec both exit 0. No new G101 or phase-26 finding.
result: [pending]

### 3. Full fuku/modules package
expected: `TZ=UTC go test -tags testtools -count=1 ./fuku/modules`, compared with commit 78648d16. Phase 26 tests stay green. Remaining failures are the pre-existing captcha expiry set (`chk_captcha_expires_at`) and `TestChatClockReadsTheGroup`, `TestApplyTimezone`, `TestSettingsTimezoneTyped`.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
