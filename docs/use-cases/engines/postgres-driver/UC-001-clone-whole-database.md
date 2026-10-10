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
| ID | UC-001 |
| Name | Clone a whole Postgres database remote → local |
| Requirement reference | FR-001, FR-004, FR-006, FR-007, FR-008, FR-011 |
| Goal | A Postgres database on a remote profile lands in the local container, complete and consistent |
| Primary actor | CLI user |

## Supporting Actors
- Local `postgres` container (runs pg_dump/pg_restore/psql)
- Remote PostgreSQL server behind the source profile

## Preconditions
- Local `postgres` container is running from the official image with `POSTGRES_PASSWORD` set
- A connection profile holds `postgres.host`/`postgres.user` (+optional port/password)
- The remote server is reachable from inside the local container

## Trigger
User runs `dbclone -from <profile>` (interactive pick or `-all` / `-only postgres:db`).

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | `Databases(src)` lists user databases (excl. postgres/template0/template1) | picker shows `postgres:<db>` entries |
| 2 | user | selects `postgres:app` (whole db) | job queued for engine `postgres` |
| 3 | system | `Prepare(dst, db, fresh=false)` | target database exists (created if missing) |
| 4 | system | `Plan` returns one stream: `pg_dump -Fc` on src → `pg_restore -j <w> --clean --if-exists --no-owner --no-privileges` on dst | dump pipes into restore; restore parallelizes internally; ordering by TOC |
| 5 | system | progress shows `processing data for table "schema.name"` markers | user sees live table progress |
| 6 | system | stream exits 0 | `postgres:app` identical on local: tables incl. data/indexes/constraints, views, matviews with data, functions, triggers, sequence values |

## Alternative Flows
### A1 — Database already exists on target
- Trigger: `existing[postgres:app]` is true

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | restore still runs `--clean --if-exists` | objects named in the dump are dropped and recreated; objects only on the target are kept |

### A2 — `-fresh` on local
- Trigger: user passes `-fresh`

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A2.1 | `Prepare` drops the database first (FORCE/terminate) | target ends up identical to source |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| Remote pg_dump major < server major | tool error surfaces in the database log, stream fails | log shows the version-mismatch message |
| Source user lacks read privilege on some objects | warning recorded, clone continues | `! postgres:app: <warning>` printed |
| Stream fails mid-run | engine retries (≤3, backoff); `--clean --if-exists` makes re-run safe | second attempt starts clean |

## Postconditions
- `postgres:app` exists on the local container with all non-system-schema objects and matching sequence values
- No dump files on disk; per-database log written under the run's log dir

## Business Rules
- Restores are `--no-owner --no-privileges`: objects land owned by the target login
- `-fresh` is refused for non-local targets by the existing guard
- `pg_dump` cannot dump a server newer than its own major version

## Data
- `driver.Object`: `schema.name` qualified names, kind table/view/matview, byte size
- `driver.Plan`: one stream, `Weight = -w` (pool slots held)

## Acceptance Criteria
- [ ] Whole-database clone lands all object kinds working on the target
- [ ] Sequence `setval`s match the source
- [ ] Retried stream completes without "already exists" errors
- [ ] Password never appears in argv, logs or errors
