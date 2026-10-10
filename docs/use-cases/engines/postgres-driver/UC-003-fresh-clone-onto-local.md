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
| ID | UC-003 |
| Name | Fresh clone onto local (`-fresh`) |
| Requirement reference | FR-007 |
| Goal | The local target database is dropped and recreated so it ends up identical to the source |
| Primary actor | CLI user |

## Supporting Actors
- Local `postgres` container

## Preconditions
- Target endpoint is `local` (guard refuses `-fresh` otherwise)
- The target database may exist and may have open connections

## Trigger
`dbclone -fresh -from staging -only postgres:app` or interactive pick with `-fresh`.

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | `Prepare(dst, db, fresh=true)` | drop phase starts |
| 2 | system | `DROP DATABASE "app" WITH (FORCE)` (PG13+) — open connections terminated by the server | database gone |
| 3 | system | `CREATE DATABASE "app"` | empty database ready |
| 4 | system | UC-001 clone flow runs | target ends identical to source |

## Alternative Flows
### A1 — Server older than PG13
- Trigger: `WITH (FORCE)` rejected by the server

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '<db>'` then plain `DROP DATABASE` | same outcome |

### A2 — Database does not exist yet
- Trigger: fresh on a new database

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A2.1 | `DROP DATABASE IF EXISTS` — no-op | `CREATE DATABASE` proceeds |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| `-fresh` with a non-local target | existing `guard` refuses before any work | `-fresh drops whole databases and is only allowed when the target is "local"` |
| User lacks DROP/CREATE privilege (remote target — unreachable in practice since fresh is local-only) | error surfaces | real server error, no silent skip |

## Postconditions
- Local `postgres` container holds a database byte-identical in content to the source selection

## Business Rules
- Fresh applies per database in the job set; unrelated local databases are untouched
- The drop must not hang on open connections — force or terminate, never wait

## Data
- `fresh bool` reaches `Prepare` from `clone.Options.Fresh`

## Acceptance Criteria
- [ ] `-fresh` drops and recreates the database even while another session is connected
- [ ] Post-clone content matches the source exactly
- [ ] `-fresh` to a remote target is refused by the guard
