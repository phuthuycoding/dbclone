---
feature: "postgres-driver"
context: "engines"
created: "20261010_1959"
status: planning
---

# Use Case

## Overview

| Field | Value |
|---|---|
| ID | UC-006 |
| Name | Retry on a flaky link; fast-fail on auth/permission errors |
| Requirement reference | FR-008, FR-009, FR-010 |
| Goal | Transient failures retry cleanly; errors no retry can fix fail at once with a redacted message |
| Primary actor | CLI user |

## Supporting Actors
- `clone` engine (retry loop, `Permanent` check)
- Local `postgres` container tools

## Preconditions
- A postgres clone is running or planned

## Trigger
Any stream failure during `clone.Run`, or a profile with wrong credentials.

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | transient error (connection reset, remote restart) fails the stream | engine retries ≤3 with backoff |
| 2 | system | re-run executes the same `pg_dump | pg_restore` | `--clean --if-exists` drops/recreates objects — no "already exists" cascade |
| 3 | system | stream succeeds on retry | clone completes, warning-free |

## Alternative Flows
### A1 — Permanent error first
- Trigger: wrong password / no privilege / unknown database

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | tool stderr line matches `Permanent` | stream fails at once — zero retries, clear message |

### A2 — Flaky link exceeds retries
- Trigger: 3 failed attempts

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A2.1 | database marked failed, other databases continue | `✗ postgres:app …` + log tail at the end |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| Error text containing credentials | `docker.Redact` scrubs `scheme://…@` forms before logging | `postgres://***@host/…` in output |

## Postconditions
- Retry-safe streams never corrupt the target: every attempt drops and recreates its objects
- Permanent errors are reported verbatim (redacted), not masked as transient

## Business Rules
- `Permanent` covers: `password authentication failed`, `permission denied`,
  `must be owner`, `database "…" does not exist`, `relation "…" does not exist`
- Transient by absence: a line matching none of the patterns retries normally
- Retries never produce duplicate objects on the target

## Data
- `Permanent(line string) bool` — stderr line classification
- Stream retry count/backoff owned by `clone` engine (unchanged)

## Acceptance Criteria
- [ ] Auth failure fails immediately without a single retry
- [ ] A killed-then-retried stream completes without duplicate-object errors
- [ ] No password in any failure output
