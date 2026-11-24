---
phase: quick-261124-l1n
plan: 01
subsystem: ci
tags: [lint, golangci-lint, errcheck, gocyclo, unused, ci]
key-files:
  modified:
    - xenobot/db/backup/backup.go
    - xenobot/db/backup/backup_test.go
    - xenobot/db/channels/optimized.go
    - xenobot/db/connections/repository_test.go
    - xenobot/db/notes/repository.go
    - xenobot/db/warns/repository.go
    - xenobot/db/warns/repository_test.go
    - xenobot/modules/antiraid_retired_test.go
    - xenobot/utils/formatting/formatting_test.go
duration: 1 session
completed: 2026-11-24
---

# Quick task 261124-l1n summary

Cleared the 38 findings of CI run 45487686026 (the Code Quality failure on 6ce7e378) without touching
`only-new-issues` or the linter configuration.

- **errcheck (33 + 2):** every flagged setup call in the five test files now fails the test when it errors. Two more
  unchecked miniredis `Set` calls in `antiraid_retired_test.go` that the run did not list were fixed the same way.
- **unused (3):** `getChannelSettingsRaw`, `getAllChatNotes` and `checkWarns` had no caller (`git grep`: definitions and
  log strings only; the `checkWarns` hits in `xenobot/modules` are test-case struct fields) and were deleted.
- **gocyclo (2):** `ExportModuleData` and `clearModuleData` dispatch through `moduleExporters` and `moduleClearers`
  maps. A generic `exporter` adapter keeps each export function's typed return. Behaviour is unchanged: an unknown
  module returns `unknown module: <name>`, and `clearModuleData` still ensures the chat row first.

## Verification

- `make lint`: 0 issues. `golangci-lint run --new=false` (v2.13.1) reports none of the run's findings and nothing new in
  the touched files; the repository still has other pre-existing findings outside them, unchanged by this task.
- `make test`: passed locally (exit 0, no failing test; `xenobot/modules` 445 s).

## Deviations

None. The rest of the repository's pre-existing findings (`golangci-lint run --new=false` lists 163 in all before this
task) are out of scope.
