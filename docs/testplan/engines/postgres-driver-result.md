---
feature: "postgres-driver"
context: "engines"
tested: "20261010_2027"
execution: "2d4ebf5b-3826-40e3-b153-0e4b03b8f270"
status: PASS
---

# Testing Result

## Feature
postgres-driver

## Environment
- OS: macOS (darwin/arm64)
- Runtime: Go 1.26.0 toolchain; Docker Desktop
- Tooling: `go test`; throwaway `postgres:17-alpine` containers on a private docker
  network (`dbclone-test-pg` tools host + `dbclone-test-pg2` remote server, removed
  after the run)

## Execution Time
~9s test run (plus one-time image pull)

## Summary
| Metric | Result |
|---|---:|
| Total | 14 |
| Passed | 14 |
| Failed | 0 |
| Rejected | 0 |
| Blocked | 0 |

## Test Results

| Case / test name | Type | Status | Expected | Actual | Evidence |
|---|---|---|---|---|---|
| TC-001 Local contract | Unit | PASS | container `postgres`, env override, tools pg_dump/pg_restore/psql, only POSTGRES_PASSWORD required | all fields verified; `DBCLONE_POSTGRES_CONTAINER=mypg` honored | TestLocal |
| TC-002 Fields/Configured/Address | Unit | PASS | 4 `postgres.*` fields, secret password, port default 5432, host+user required, lowercase host:port address | verified incl. partial-profile negatives | TestFieldsConfiguredAddress |
| TC-003 safe-name validation | Unit | PASS | `schema.table` accepted; quotes/semicolons/spaces/uppercase rejected | all cases match | TestSafeName |
| TC-004 whole-db stream construction | Unit | PASS | `pg_dump -Fd -f $STAGE`; restore drains stdin → `pg_restore -j --clean --if-exists --no-owner --no-privileges` on stage → `rm -rf`; weight = workers | all flags present in generated scripts; weight 4 = workers 4 | TestWholeStream |
| TC-005 `-t` args per selected object | Unit | PASS | one `-t name` per selected qualified object plus owned sequences; unsafe names filtered | ` -t public.orders -t analytics.events -t public.orders_id_seq`; `bad"name` filtered | TestTableArgs |
| TC-006 awk FK filter | Unit | PASS | FK referencing unselected object dropped; selected FKs, indexes, triggers pass | real `awk` run over post-data text | TestAwkFilter |
| TC-007 Permanent matching | Unit | PASS | auth/permission/owner/unknown-db/version-mismatch → true; connection reset/timeout → false | all cases match | TestPermanent |
| TC-008 stage markers | Unit | PASS | `processing data for table "s"."t"` → `s.t`; `COPY s.t` chunk → `s.t` | both log and stream markers verified | TestStageMarkers |
| TC-009 preflight real container | Integration | PASS | check line OK, driver in `Ready`; bogus container → failed check with fix | TestPreflightPostgres | `preflight.Run` against testPg |
| TC-010 whole-db clone | Integration | PASS | all object kinds land on target; sequences setval correct; re-run replaces cleanly | rows/view/matview/function/trigger/index/FK/sequence verified on dst; second run clean | TestCloneWholeDatabase via `clone.Run` remote(testPg2)→local(testPg) |
| TC-011 `-only` subset clone | Integration | PASS | only `public.orders` lands with index+trigger; FK to unselected `customers` skipped + warned; customers absent | all verified; warning names `orders_customer_id_fkey` | TestCloneSelectedObjects |
| TC-012 `-fresh` drop | Integration | PASS | drop+recreate under an open `pg_sleep` connection; `fresh=false` leaves db untouched | dropped while connection held; recreated empty | TestPrepareFresh |
| TC-013 remote→remote | Integration | PASS | clone between two profile endpoints completes; index present on target | `app3` on testPg2 → `app3` on testPg via remote endpoints | TestCloneRemoteToRemote |
| TC-014 wrong password fast-fail | Integration | PASS | `Databases` fails; line matches `Permanent`; password absent from error text | auth error classified permanent, no secret in output | TestPermanentAuthFailure |

## Commands and Evidence

| Command / tool | Exit code | Evidence / output |
|---|---:|---|
| `gofmt -l .` | 0 | empty output — tree clean |
| `go vet ./...` | 0 | no findings |
| `go build ./...` | 0 | binary builds |
| `go test ./internal/driver/postgres/ -v -coverprofile` | 0 | 14/14 PASS, coverage 90.3% |
| `go test ./...` | 0 | all packages ok (dbclone, config, docker, mongo, postgres) |

## Failures and Blockers

| Case / test name | Error / blocker | Impact | Next action |
|---|---|---|---|
| — | none | — | — |

## Coverage

| Metric / scope | Target | Measured | Evidence |
|---|---:|---:|---|
| Overall code coverage (`internal/driver/postgres`) | 80% | 90.3% | `go test -coverprofile`: `coverage: 90.3% of statements` |

## Regression
`go test ./...` over the whole module passes — mongo/mysql drivers, config, docker
and root package tests unaffected; the only change outside the new package is a
one-line registration in `main.go`.

## Conclusion
- All 14 planned cases pass on execution `2d4ebf5b-3826-40e3-b153-0e4b03b8f270`;
  coverage 90.3% exceeds the 80% target. Status: PASS — ready for review.
