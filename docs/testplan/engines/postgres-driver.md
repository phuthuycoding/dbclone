---
feature: "postgres-driver"
context: "engines"
created: "20261010_1959"
status: planning
---

# Test Plan

Test Strategy from `phase-1-spec-requirement.md` decides the depth:
`unit` → Unit; `unit+integration` → Unit + Integration; `full` → Unit + Integration + UI/E2E.

## Feature Test Summary

| Field | Value |
|---|---|
| Feature | postgres-driver |
| Context | engines |
| Test level | unit+integration |
| UI scope | n/a (CLI; no new UI components) |
| Tools / commands | `go test ./...`; docker for integration cases |
| Coverage target | 80% |

## Overall Case Counts

| Test type | Planned | Must pass | Notes |
|---|---:|---:|---|
| Unit | 8 | 8 | pure logic: flags, naming, filters, regexes |
| Integration | 6 | 6 | real `postgres` container; skipped when docker is absent |
| UI / E2E | 0 | 0 | test level is unit+integration; CLI reuses existing UI |
| **Total** | **14** | **14** | — |

## Use Case Coverage Matrix

| Use case | Requirement(s) | Test cases | Planned | Pass criteria |
|---|---|---|---:|---|
| UC-001 | FR-001, FR-004, FR-006, FR-007, FR-008, FR-011 | TC-004, TC-008, TC-010 | 3 | whole-db clone lands every object kind |
| UC-002 | FR-005, FR-006, FR-008 | TC-003, TC-005, TC-006, TC-011 | 4 | subset clone exact; dangling FK skipped |
| UC-003 | FR-007 | TC-012 | 1 | drop+recreate under open connections |
| UC-004 | FR-003, FR-010 | TC-002, TC-013 | 2 | remote↔remote works; guard refuses same address |
| UC-005 | FR-002, FR-012 | TC-001, TC-009 | 2 | preflight line accurate, fix actionable |
| UC-006 | FR-008, FR-009, FR-010 | TC-007, TC-014 | 2 | permanent fails at once; retry clean |

## Requirement Coverage Matrix

| Requirement | Use case(s) | Test case(s) | Covered? | Gap / note |
|---|---|---|---|---|
| FR-001 | UC-001 | TC-010 | yes | registration proven by a running clone |
| FR-002 | UC-005 | TC-001, TC-009 | yes | Local() contract + real container check |
| FR-003 | UC-004 | TC-002, TC-013 | yes | fields, defaults, address guard |
| FR-004 | UC-001 | TC-010 | yes | system dbs excluded on real server |
| FR-005 | UC-002 | TC-003, TC-005, TC-011 | yes | qualified names in listing and `-only` |
| FR-006 | UC-001, UC-002 | TC-004, TC-010, TC-011 | yes | ordering + parallelism verified on real clone |
| FR-007 | UC-003 | TC-012 | yes | fresh drop with open connections |
| FR-008 | UC-001, UC-002, UC-006 | TC-010, TC-011 | yes | `--clean --if-exists` re-run safety |
| FR-009 | UC-006 | TC-007, TC-014 | yes | regex unit + real auth failure |
| FR-010 | UC-004, UC-006 | TC-013, TC-014 | yes | argv/log inspection in integration |
| FR-011 | UC-001 | TC-008 | yes | marker parsing unit test |
| FR-012 | UC-005 | TC-009 | yes | compose service inspected by test + docs diff |

## TC-001

| Field | Detail |
|---|---|
| Test case ID | TC-001 |
| Requirement reference | FR-002 |
| Use case reference | UC-005 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | `postgres.New().Local()` with and without `DBCLONE_POSTGRES_CONTAINER` |
| Steps | See steps table below |
| Expected outcome | Container defaults `postgres`, env override honored; Tools = pg_dump/pg_restore/psql; Credentials contains `POSTGRES_PASSWORD` (not `POSTGRES_USER`); RunExample runs the official image |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | read `Local()` defaults | Container `postgres`, env var name `DBCLONE_POSTGRES_CONTAINER` |
| 2 | set `DBCLONE_POSTGRES_CONTAINER=mypg` | Container `mypg` |
| 3 | inspect Tools/Credentials/RunExample | required tools listed; only `POSTGRES_PASSWORD` required; run example uses `postgres:` image |

## TC-002

| Field | Detail |
|---|---|
| Test case ID | TC-002 |
| Requirement reference | FR-003 |
| Use case reference | UC-004 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | `Fields()`, `Configured()`, `Address()` on local and remote endpoints |
| Steps | See steps table below |
| Expected outcome | four `postgres.*` fields, password marked secret, port default `5432`; Configured needs host+user (local always true); Address is lowercased `host:port` |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | list `Fields()` | host/port/user/password keys with `postgres.` prefix; password `Secret: true` |
| 2 | `Configured` on partial profiles | false when host or user missing; true for local |
| 3 | `Address` of `HOST:UPPER` vs `host:upper` | identical lowercase `host:port` strings |

## TC-003

| Field | Detail |
|---|---|
| Test case ID | TC-003 |
| Requirement reference | FR-005 |
| Use case reference | UC-002 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | qualified object names through the safe-name validator |
| Steps | See steps table below |
| Expected outcome | `schema.table` accepted (incl. dots in both parts per rule); names with quotes/semicolons/spaces rejected |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | validate `public.orders`, `analytics.events_2` | accepted |
| 2 | validate `weird"name`, `a;b`, `a b` | rejected with the unsupported-characters error |

## TC-004

| Field | Detail |
|---|---|
| Test case ID | TC-004 |
| Requirement reference | FR-006 |
| Use case reference | UC-001 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | `Plan()` on a whole database (no `only`) — command construction inspected via the returned stream funcs |
| Steps | See steps table below |
| Expected outcome | one stream per database; `Weight == workers`; dump is `pg_dump -Fd -j <w> -f <unique container path>` with env-based auth; restore drains stdin then runs `pg_restore -j <w> --clean --if-exists --no-owner --no-privileges` on that path and removes it |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | build plan for db with tables/views/matviews | single phase, one stream, weight = `-w` |
| 2 | inspect dump command script | `pg_dump -Fd -j`, `--no-password`, `-f` stage path inside the container, host/user via env |
| 3 | inspect restore command script | stdin drained first; `pg_restore -j` on the stage path with clean/if-exists/no-owner/no-privileges; `rm -rf` of the stage |

## TC-005

| Field | Detail |
|---|---|
| Test case ID | TC-005 |
| Requirement reference | FR-005 |
| Use case reference | UC-002 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | `Plan()` with `only = ["public.orders","analytics.events"]`, plus an unknown name |
| Steps | See steps table below |
| Expected outcome | `-t` argument emitted once per selected qualified object; unknown name → `not found on the source` before any stream runs |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | plan with two selected objects | `-t` args cover exactly those names |
| 2 | plan with `["public.ghost"]` | error naming the missing object |

## TC-006

| Field | Detail |
|---|---|
| Test case ID | TC-006 |
| Requirement reference | FR-006 |
| Use case reference | UC-002 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | post-data SQL text: FK to selected table, FK to unselected table, index, trigger, comment containing `;` |
| Steps | See steps table below |
| Expected outcome | statements referencing unselected objects are dropped; selected-object FKs, indexes, triggers pass through |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | filter FK `REFERENCES other.customers` (unselected) | statement absent from output |
| 2 | filter FK `REFERENCES public.customers` (selected) | statement present |
| 3 | filter `CREATE INDEX` / `CREATE TRIGGER` | present |

## TC-007

| Field | Detail |
|---|---|
| Test case ID | TC-007 |
| Requirement reference | FR-009 |
| Use case reference | UC-006 |
| Test type | Unit |
| Priority | High |
| Preconditions | none |
| Input | `Permanent()` over psql/pg_restore error lines |
| Steps | See steps table below |
| Expected outcome | auth failure, permission denied, must-be-owner, unknown database/relation → true; `server closed the connection`, `timeout` → false |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | `FATAL:  password authentication failed for user "r"` | permanent |
| 2 | `ERROR:  permission denied for table orders` | permanent |
| 3 | `server closed the connection unexpectedly` | not permanent (retriable) |

## TC-008

| Field | Detail |
|---|---|
| Test case ID | TC-008 |
| Requirement reference | FR-011 |
| Use case reference | UC-001 |
| Test type | Unit |
| Priority | Medium |
| Preconditions | none |
| Input | `StageFromLog` on `pg_restore --verbose` lines; `StageFromStream` on plain-dump `COPY` lines |
| Steps | See steps table below |
| Expected outcome | `processing data for table "s.t"` → `s.t`; `COPY s.t (...)` chunk → `s.t`; unmatched → `""` |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | feed `processing data for table "public.orders"` | `public.orders` |
| 2 | feed unrelated output | `""` |

## TC-009

| Field | Detail |
|---|---|
| Test case ID | TC-009 |
| Requirement reference | FR-002, FR-012 |
| Use case reference | UC-005 |
| Test type | Integration |
| Priority | High |
| Preconditions | docker available; test postgres container `dbclone-test-pg` running with `POSTGRES_PASSWORD` |
| Input | `preflight.Run(ctx, []driver.Driver{postgres.New()})` with `DBCLONE_POSTGRES_CONTAINER=dbclone-test-pg` |
| Steps | See steps table below |
| Expected outcome | report `Ready` contains the postgres driver; check line is OK |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | run preflight against the test container | `postgres` check OK, driver in `Ready` |
| 2 | repeat with a bogus container name | check fails with `not found` + RunExample fix |

## TC-010

| Field | Detail |
|---|---|
| Test case ID | TC-010 |
| Requirement reference | FR-001, FR-004, FR-006, FR-008 |
| Use case reference | UC-001 |
| Test type | Integration |
| Priority | High |
| Preconditions | test container; source db seeded: non-public schema, table + serial PK, second table + FK, view, matview, function, trigger |
| Input | `clone.Run` with one `postgres` job, src = localhost-profile endpoint, dst = local endpoint, stub reporter |
| Steps | See steps table below |
| Expected outcome | every seeded object works on the target: rows identical, matview populated, sequence `setval` correct, function/trigger fire; run twice — second run replaces cleanly |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | seed source database | objects present in two schemas |
| 2 | run clone via `clone.Run` | job result has no error |
| 3 | query target | data identical; matview has rows; `setval` matches; trigger fires on insert |
| 4 | run clone again | replaces cleanly, no duplicate-object errors |

## TC-011

| Field | Detail |
|---|---|
| Requirement reference | FR-005, FR-006, FR-008 |
| Use case reference | UC-002 |
| Test case ID | TC-011 |
| Test type | Integration |
| Priority | High |
| Preconditions | seeded source: `public.orders` FK → `public.customers`, index and trigger on `orders` |
| Input | `Plan`/`clone.Run` with `only = ["public.orders"]` |
| Steps | See steps table below |
| Expected outcome | only `orders` lands (with index + trigger); `customers` untouched/absent; warning names the skipped FK |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | plan/run `only public.orders` | clone succeeds |
| 2 | inspect target | `orders` has index and trigger; no `customers` created by the clone |
| 3 | check warnings | FK `orders_customer_id_fkey` listed as skipped |

## TC-012

| Field | Detail |
|---|---|
| Test case ID | TC-012 |
| Requirement reference | FR-007 |
| Use case reference | UC-003 |
| Test type | Integration |
| Priority | High |
| Preconditions | target db exists on the container with an idle open connection |
| Input | `Prepare(dst, db, fresh=true)` |
| Steps | See steps table below |
| Expected outcome | database dropped and recreated despite the open connection; `fresh=false` never drops |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | hold an open connection to target db | connection idle |
| 2 | `Prepare(fresh=true)` | returns nil; db recreated empty |
| 3 | `Prepare(fresh=false)` on existing db | no drop, returns nil |

## TC-013

| Field | Detail |
|---|---|
| Test case ID | TC-013 |
| Requirement reference | FR-003, FR-010 |
| Use case reference | UC-004 |
| Test type | Integration |
| Priority | Medium |
| Preconditions | test container published on two host ports (e.g. 5432/5433) so two profile addresses differ |
| Input | src = profileA endpoint, dst = profileB endpoint (distinct `host:port` strings, same container) |
| Steps | See steps table below |
| Expected outcome | remote→remote clone completes; guard refuses when both addresses are identical; no password on argv or in logs |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | clone db via two profile endpoints | data lands on the target database |
| 2 | attempt same `host:port` on both sides | `Address` equality → guard refusal (unit-covered, sanity-checked) |
| 3 | inspect `docker exec` argv of the stream commands | credentials only as `-e VAR` names; no literal password |

## TC-014

| Field | Detail |
|---|---|
| Test case ID | TC-014 |
| Requirement reference | FR-009, FR-010 |
| Use case reference | UC-006 |
| Test type | Integration |
| Priority | High |
| Preconditions | test container; profile with a deliberately wrong password |
| Input | `Databases(src)` / a clone stream with bad credentials |
| Steps | See steps table below |
| Expected outcome | `Permanent` matches the auth line → engine fails the stream on first attempt (single attempt in log); error output contains no password |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | run stream with wrong password | fails once, no retry attempts |
| 2 | grep the database log for the password | absent |
