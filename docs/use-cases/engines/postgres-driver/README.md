---
feature: "postgres-driver"
context: "engines"
created: "20261010_1959"
status: planning
---

# Use Case Index

Every use case is its own file under `use-cases/`, named `UC-###-<slug>.md` (e.g. `UC-001-create-task.md`). Do not write a combined narrative here.

## Use Case Files

| ID | Name | File | Primary Actor | Status |
|---|---|---|---|---|
| UC-001 | Clone a whole Postgres database remote → local | [UC-001](UC-001-clone-whole-database.md) | CLI user | planned |
| UC-002 | Clone selected objects (`-only postgres:db.schema.table`) | [UC-002](UC-002-clone-selected-objects.md) | CLI user | planned |
| UC-003 | Fresh clone onto local (`-fresh`) | [UC-003](UC-003-fresh-clone-onto-local.md) | CLI user | planned |
| UC-004 | Clone between remote endpoints | [UC-004](UC-004-clone-between-remote-endpoints.md) | CLI user | planned |
| UC-005 | Preflight failure → fix guidance | [UC-005](UC-005-preflight-fix-guidance.md) | CLI user | planned |
| UC-006 | Retry on a flaky link; fast-fail on auth/permission errors | [UC-006](UC-006-retry-and-fast-fail.md) | CLI user | planned |

## Use Case Coverage

| UC ID | FR references | TC references | Acceptance coverage |
|---|---|---|---|
| UC-001 | FR-001, FR-004, FR-006, FR-007, FR-008, FR-011 | TC-004, TC-008, TC-010 | object parity, retry-safe restore, sequence values, stage markers |
| UC-002 | FR-005, FR-006, FR-008 | TC-003, TC-005, TC-006, TC-011 | qualified names, `-t` selection, dangling-FK skip, index retention |
| UC-003 | FR-007 | TC-012 | drop under open connections, create-if-missing, guard refusal |
| UC-004 | FR-003, FR-010 | TC-002, TC-013 | same-address guard, env-only secrets, non-superuser restores |
| UC-005 | FR-002, FR-012 | TC-001, TC-009 | container contract, fix guidance, engine isolation |
| UC-006 | FR-008, FR-009, FR-010 | TC-007, TC-014 | permanent-error matching, retry cleanliness, redaction |

## Totals

| Metric | Total |
|---|---:|
| Use cases | 6 |
| Actors | 3 (CLI user; local postgres container; remote PostgreSQL server) |
| Functional requirements covered | 12 (FR-001..FR-012) |
| Test cases linked | 14 |
