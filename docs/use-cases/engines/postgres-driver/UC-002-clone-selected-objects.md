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
| ID | UC-002 |
| Name | Clone selected objects (`-only postgres:db.schema.table`) |
| Requirement reference | FR-005, FR-006, FR-008 |
| Goal | Only the named objects of a Postgres database are cloned to the target |
| Primary actor | CLI user |

## Supporting Actors
- Local `postgres` container
- Remote PostgreSQL server behind the source profile

## Preconditions
- Same as UC-001; the named objects exist on the source database

## Trigger
`dbclone -only postgres:app.public.orders,postgres:app.analytics.events -from staging`
or the interactive narrow-down picker on a `postgres` job.

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | system | `Objects(src, db)` lists `schema.name` entries | picker / `-only` resolves object names |
| 2 | system | `driver.Select` matches every requested name | unknown name → error before any write |
| 3 | system | `Plan` builds the subset plan | two streams: pre-data+data, then post-data |
| 4 | system | stream 1: `pg_dump -Fp -t sel…` (pre-data+data) → `psql` | tables land with rows, serial columns wired |
| 5 | system | stream 2: `pg_dump -Fp --section=post-data -t sel…` → awk filter → `psql` | indexes/triggers land; FK constraints referencing unselected objects are dropped from the stream |
| 6 | system | warnings record each skipped FK | `! postgres:app: FK <name> → <unselected> skipped` |

## Alternative Flows
### A1 — Selected objects have no outbound FKs
- Trigger: no `REFERENCES` target outside the selection

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | awk filter passes every statement | identical to main flow, no warnings |

### A2 — Object named in a non-public schema only
- Trigger: `-only postgres:app.analytics.events`

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A2.1 | `schema.name` matching keeps `analytics.events` distinct from any `public.events` | only the analytics table clones |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| `-only` names an object that does not exist | `driver.Select` errors | `"%s" not found on the source`, nothing written |
| Object name fails the safe-name rule | `Plan` errors | clear error, no partial clone |

## Postconditions
- Only the selected objects exist (or are replaced) on the target; unselected target objects are untouched
- Selected tables keep indexes/triggers; FKs into unselected objects are absent and reported

## Business Rules
- Object names are always schema-qualified: `public.orders`, `analytics.events`
- A skipped outbound FK is a warning, never a silent drop and never a clone failure
- Selected objects are restored drop-and-recreate (`--clean --if-exists`), so re-running is safe

## Data
- `only []string` in `Plan`: exact `schema.name` matches from `Objects`

## Acceptance Criteria
- [ ] `-only postgres:db.schema.table` clones exactly the named objects
- [ ] Selected tables land with indexes and triggers
- [ ] FK referencing an unselected object is skipped with a warning, clone succeeds
- [ ] Unknown object name fails before any write
