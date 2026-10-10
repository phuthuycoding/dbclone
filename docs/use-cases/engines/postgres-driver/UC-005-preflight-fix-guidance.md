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
| ID | UC-005 |
| Name | Preflight failure → fix guidance for the postgres container |
| Requirement reference | FR-002, FR-012 |
| Goal | A missing or misconfigured local postgres container is reported with an exact fix, and only that engine is disabled |
| Primary actor | CLI user |

## Supporting Actors
- `preflight.Run` / `checkLocal`
- Docker daemon

## Preconditions
- Docker is usable; the `postgres` container is absent, stopped, from the wrong image, or missing `POSTGRES_PASSWORD`

## Trigger
`dbclone -check` or any run (preflight executes at every start).

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | `checkLocal("postgres", driver.Local())` inspects the container | one `postgres: local container "postgres"` report line |
| 2 | system | container absent → `not found` + `docker run` example; stopped → `docker start`; wrong tools/env → `not usable` + recreate example | user gets the exact command to fix |
| 3 | system | engine excluded from `report.Ready` | mongo/mysql remain usable; postgres endpoints are skipped |

## Alternative Flows
### A1 — Custom container name
- Trigger: `DBCLONE_POSTGRES_CONTAINER=mypg`

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | check inspects `mypg`; fix lines mention the env override | `or use an existing container: DBCLONE_POSTGRES_CONTAINER=<name> dbclone` |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| Docker itself unusable | `ErrNoDocker`, all engines dead | existing fatal path, unchanged |

## Postconditions
- User can fix the container from the printed commands without reading docs

## Business Rules
- `POSTGRES_USER` is optional in the image — not a checked credential; runtime default is `postgres`
- Required tools: `pg_dump`, `pg_restore`, `psql`; required env: `POSTGRES_PASSWORD`
- A missing postgres container never blocks the other engines

## Data
- `driver.Local`: Container, ContainerEnv, Tools, Credentials, RunExample

## Acceptance Criteria
- [ ] `dbclone -check` shows the postgres line green on a correct container
- [ ] Missing tools/env produce the recreate hint with `RunExample`
- [ ] A broken postgres container leaves mongo/mysql usable
