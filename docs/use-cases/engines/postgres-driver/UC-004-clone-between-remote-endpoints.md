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
| ID | UC-004 |
| Name | Clone between remote endpoints (local → remote, remote → remote) |
| Requirement reference | FR-003, FR-010 |
| Goal | A Postgres database clones to a remote profile target — with or without `local` on the other side |
| Primary actor | CLI user |

## Supporting Actors
- Local `postgres` container (tools host for both directions)
- Two remote PostgreSQL servers behind profiles

## Preconditions
- Both endpoints carry `postgres.host`/`postgres.user` for the engine (`Configured`)
- Target requires `-confirm <name>` (scripted) or typed confirmation (interactive)

## Trigger
`dbclone -from local -to staging -only postgres:app -confirm staging` or
`-from staging -to prod-replica …`.

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | `Configured(src)` and `Configured(dst)` true for postgres | engine participates in discovery |
| 2 | system | `guard`: `Address(src) != Address(dst)` | same host:port combination refused |
| 3 | system | non-local target confirmed (`-confirm`/typed name) | clone authorized |
| 4 | system | UC-001 stream runs with remote conn on both sides | dump inside local container → restore to remote target |
| 5 | system | restore `--no-owner --no-privileges` | objects land owned by the target user |

## Alternative Flows
### A1 — `local` → remote
- Trigger: source is the local container

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | local source logs in via container `POSTGRES_*` env; remote dst via profile env | same stream shape |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| `Address(src) == Address(dst)` | guard refuses | `"postgres: … point at the same server"` |
| Target user lacks CREATE/ownership on existing db | real error in database log | stream fails, no retry on `Permanent` match |
| Password reaches argv or a log line | must not happen by construction | env-only credentials; `docker.Redact` scrubs URI forms |

## Postconditions
- Target database on the remote server holds the cloned objects owned by the target login

## Business Rules
- A remote target always needs its name typed/`-confirm` — `-yes` never skips it
- Remote users may lack `CREATEDB`: `Prepare` creates only when missing and surfaces
  the real error otherwise
- No superuser-only mechanism may be required on either remote side (no
  `--disable-triggers`, no `session_replication_role`)

## Data
- `Endpoint.Values`: `postgres.host`, `postgres.port`, `postgres.user`, `postgres.password`

## Acceptance Criteria
- [ ] local → remote and remote → remote postgres clones complete
- [ ] Same-address profiles are refused by the guard
- [ ] Password never appears in argv, logs or error output
