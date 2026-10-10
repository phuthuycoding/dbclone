---
feature: postgres-driver
context: engines
created: 20261010_1951
kind: feature
status: archived
---
# Spec Requirement

## Feature
postgres-driver

## Objective
Add a `postgres` engine so `postgres:db` databases clone between any two endpoints —
remote profile → local, local → remote, remote → remote — with the same streaming,
parallel, retried pipeline the mongo and mysql drivers already use.

## Problem Statement
dbclone supports MongoDB and MySQL only. Users whose databases run on PostgreSQL cannot
use the tool at all: the engine list is hardwired in `main.go` and no driver implements
`driver.Driver` for Postgres. The adapter interface was designed for exactly this
extension ("adding an engine means writing one package under driver/ and registering it
in main.go"), so the gap is one driver package plus its local-container contract.

## Scope
### In Scope
- New package `internal/driver/postgres` implementing `driver.Driver`, registered in
  `main.go`'s `drivers` slice.
- Local container contract: official `postgres` image; container name `postgres`
  overridable via `DBCLONE_POSTGRES_CONTAINER`; required tools `pg_dump`, `pg_restore`,
  `psql`; local login from the container's `POSTGRES_USER`/`POSTGRES_PASSWORD` env.
- Remote profile fields `postgres.host`, `postgres.port` (default 5432), `postgres.user`,
  `postgres.password` (secret), mirroring the mysql field set.
- Full object parity with mysql: tables, views, materialized views, sequences,
  functions/procedures and triggers, across all non-system schemas (`pg_catalog`,
  `information_schema`, `pg_toast` excluded).
- Parallel clone: a whole database dumps with `pg_dump -Fd -j <w>` into a transient
  archive directory inside the tools container, then restores with
  `pg_restore -j <w>` (which cannot read an archive from stdin) and removes it.
  Dependency-safe ordering comes from pg_restore's TOC ordering (postgres has no
  non-superuser way to defer FK enforcement during load).
- Subset selection via `-only postgres:db` and `-only postgres:db.schema.table`;
  interactive object picker support through `Objects`.
- `Prepare`: create the target database when missing; `-fresh` (local target only,
  enforced by the existing guard) drops it first even with active connections.
- Retry-safe restore: re-running a stream drops and recreates its objects instead of
  failing on duplicates.
- Fast-fail on unrecoverable errors (`Permanent`): authentication, authorization,
  unknown database.
- Preflight line `postgres: local container "postgres"` with the standard fix guidance.
- `compose.yaml` gains a `postgres` service so the documented local setup covers the
  new engine.
- Unit tests for pure logic plus an integration test that clones against a real
  postgres container.

### Out of Scope
- Roles, users, grants and ownership migration — restores run with
  `--no-owner --no-privileges`; objects land owned by the target user.
- Extensions: installing `CREATE EXTENSION` prerequisites on the target is the user's
  job; failures there surface as warnings, not a hard requirement.
- Cross-engine cloning (postgres → mysql) — jobs are per-engine by design.
- Tablespaces, foreign data wrappers, publications/subscriptions and other
  server-level objects.
- Guaranteeing cross-major-version dumps beyond surfacing pg_dump's own error clearly.

## Actors
- CLI user running `dbclone` interactively or scripted (`-only`, `-all`, `-confirm`).
- Local `postgres` Docker container — hosts the tools that run every dump and restore.
- Remote PostgreSQL servers reached through named connection profiles.

## Functional Requirements
### FR-001
- Requirement: A `postgres` driver implementing every `driver.Driver` method is
  registered in `main.go`; it takes part in preflight, discovery, the picker and the
  clone pool exactly like mongo/mysql. Engine name shown in `engine:db` is `postgres`.
- Priority: must
- Notes: Registration is one line; the interface does not change.

### FR-002
- Requirement: `Local()` describes the local `postgres` container: name `postgres`
  overridable by `DBCLONE_POSTGRES_CONTAINER`; tools `pg_dump`, `pg_restore`, `psql`;
  credentials read from the container's `POSTGRES_PASSWORD` (and `POSTGRES_USER`,
  defaulting to `postgres`); `RunExample` shows a `docker run` for the official image.
- Priority: must
- Notes: `POSTGRES_USER` is optional in the image, so it cannot be a hard credential
  check — resolve it with a default at runtime instead.

### FR-003
- Requirement: `Fields()` exposes `postgres.host`, `postgres.port` (default `5432`),
  `postgres.user`, `postgres.password` (secret); `Configured()` requires host and user;
  `Address()` returns lowercased `host:port` so the same-server guard works.
- Priority: must
- Notes: Passwords reach the tools via `PGPASS`-style env vars, never on argv.

### FR-004
- Requirement: `Databases()` lists user databases on an endpoint, excluding
  `postgres`, `template0` and `template1`.
- Priority: must
- Notes: Runs `psql` inside the local container against any endpoint.

### FR-005
- Requirement: `Objects()` lists tables, views and materialized views of a database
  across non-system schemas under always-qualified `schema.name` names, each with a
  size estimate for scheduling.
- Priority: must
- Notes: Drives `-only` selection and the interactive narrow-down picker.

### FR-006
- Requirement: `Plan()` returns a whole-database stream that dumps
  `pg_dump -Fd -j <workers>` into a transient archive directory inside the tools
  container and restores it with `pg_restore -j <workers>`, deleting the directory
  afterwards. Ordering is dependency-safe by pg_restore's TOC ordering — no
  constraints, indexes, triggers or views may be created before the data they need.
  For `-only`, `Plan()` instead returns phases of plain-format
  `pg_dump | filter | psql` streams (see UC-002).
- Priority: must
- Notes: `pg_restore -j` cannot read an archive from stdin, so a pure pipe is out;
  the stage lives inside the container, is deleted after every attempt, and is the
  reason `-Fd` is used (a directory format dump parallelizes both sides).

### FR-007
- Requirement: `Prepare()` creates the target database when it does not exist; with
  `fresh` it drops the database first, terminating or forcing out active connections
  (`DROP DATABASE ... WITH (FORCE)` on PG13+, `pg_terminate_backend` fallback).
- Priority: must
- Notes: `fresh` on a non-local target stays forbidden by `guard` — no change there.

### FR-008
- Requirement: Restoring the same stream twice is safe: objects are dropped and
  recreated (e.g. `pg_restore --clean --if-exists` or equivalent), so a retry starts
  clean and never fails on already-existing objects.
- Priority: must
- Notes: Required by the engine's retry loop.

### FR-009
- Requirement: `Permanent()` matches errors no retry can fix — authentication failure,
  insufficient privilege, unknown database — so the engine fails the stream at once.
- Priority: must
- Notes: Pattern-match pq/psql error text, like the mysql `ERROR (1044|...)` regex.

### FR-010
- Requirement: Secrets never leak: remote passwords travel only as env vars into the
  container, never on the command line; tool output and errors stay scrubbed by the
  existing `docker.Redact`.
- Priority: must
- Notes: Verified — `docker.Redact`'s regex is scheme-generic and already covers
  `postgres://` URIs; field-based connections rarely print URIs anyway.

### FR-011
- Requirement: `StageFromStream`/`StageFromLog` report the object currently in flight
  from pg_dump/pg_restore output where the tools emit a usable marker; return ""
  where they do not.
- Priority: should
- Notes: pg_restore verbose output prints per-table lines; best-effort like mongo.

### FR-012
- Requirement: `compose.yaml` adds a `postgres` service (official image, env-named
  container, `POSTGRES_PASSWORD` wired to `DBCLONE_LOCAL_PASSWORD`, volume
  `postgres-data`) and README/`profiles.example.json` mention the new engine.
- Priority: should
- Notes: Keeps the documented zero-install setup honest.

## Non-Functional Requirements
- Tool version: when the local `pg_dump` major is older than a remote server, the dump
  fails — the raw tool error must surface readable in the database log (no silent
  retry loop on a version mismatch).
- Staging: whole-database clones write one transient archive directory inside the
  tools container (removed after the attempt); `-only` clones stay pure-streaming;
  nothing lands on the host besides per-database logs.
- Streams respect the global pool: a stream's `Weight` reflects the tool parallelism
  it holds (e.g. `pg_restore -j`), matching mongo's convention.
- Code passes `gofmt -l .` (clean), `go vet ./...` and `go test ./...`.
- No new Go dependencies — drivers are stdlib + `internal/docker` only.

## Main Use Cases
- UC-001 Clone a whole Postgres database remote → local
- UC-002 Clone selected objects (`-only postgres:db.schema.table`)
- UC-003 Fresh clone onto local (`-fresh` drops and recreates)
- UC-004 Local → remote / remote → remote Postgres clone
- UC-005 Preflight failure → fix guidance for the postgres container
- UC-006 Retry on a flaky link; fast-fail on auth/permission errors

## Constraints
- Every dump/restore runs inside the local `postgres` container; remote servers must
  be reachable from inside it — same model as the existing drivers.
- `pg_dump` cannot dump a server newer than its own major version; the pinned image
  tag in `RunExample`/`compose.yaml` sets the ceiling.
- Remote target users may lack `CREATEDB` and ownership rights — `Prepare` only
  creates when missing and dumps restore `--no-owner --no-privileges`.
- `database.schema.object` names: object identifiers may contain dots, so qualified
  names are `schema.object` pairs, not free-form.

## Assumptions
- Engine name is `postgres` (matching `mongo`/`mysql` full-name convention), so
  `-only` syntax reads `postgres:db.schema.table`.
- Profile fields use host/port/user/password like mysql rather than a URI like mongo.
- `RunExample` and `compose.yaml` pin a recent stable major (postgres:17); adjustable
  in planning if a different floor is wanted.
- Objects selectable via `-only` are tables/views/matviews; sequences, functions and
  triggers follow their owning tables automatically (same treatment as mysql
  routines/triggers).
- `docker.Redact` already scrubs `scheme://credentials@host` URIs generically;
  `postgres://` is assumed covered and verified during implementation.

## Acceptance Criteria
- [ ] `dbclone -check` prints a `postgres: local container "postgres"` line that goes
  green with a suitable container and lists missing tools/env with a fix command.
- [ ] Profile setup prompts for the four `postgres.*` fields; a postgres-only profile
  lists only that engine's databases in discovery.
- [ ] Cloning a database with non-public schemas, a view, a matview, a serial/sequence
  column, a function and a trigger lands every object working on the target, with
  sequence values matching the source.
- [ ] `-only postgres:db.schema.table` clones exactly that table; an unknown object
  name fails with a clear error.
- [ ] `-fresh` on local drops and recreates the database even while another session
  holds a connection to it.
- [ ] A retried stream completes without "already exists" errors; an auth failure
  fails immediately without retrying.
- [ ] Passwords appear nowhere in argv, logs or error output.
- [ ] `gofmt -l .`, `go vet ./...` and `go test ./...` are clean; the integration test
  performs a real clone against a postgres container.

## Edge Cases
- Same table name in multiple schemas — qualified `schema.table` names must keep them
  distinct in `-only` and progress labels.
- Database/object names with quotes, uppercase or unicode — validate against a
  safe-name rule or quote identically to mysql's `safeName`.
- `-fresh` while connections are open — must terminate them or fail with the real
  error, not hang.
- Source user lacking SELECT/pg_read_all_data on some objects — warn and skip like
  mysql's events, rather than aborting the whole clone.
- Extensions present on the source but not installed/allowed on the target —
  `CREATE EXTENSION` failure should not take the data down with it.
- Large objects (blobs) — included by default in `pg_dump` custom format; keep them.
- Materialized views — must exist on the target populated (pg_dump -Fc restores
  matview data; verify in the integration test).
- Remote server newer than the local container's `pg_dump` — hard error surfaced as
  the tool's own version message.
- `local` → `local` is refused by the existing guard — unchanged.

## Open Questions
- Stream mechanism: decided — `pg_restore -j` cannot read stdin, so whole-db dumps
  `pg_dump -Fd -j` to a transient container path and restores with `pg_restore -j`.
  Per-table-group parallel phases remain a future option if profiling asks for it.
- Integration-test bootstrap: reuse the `compose.yaml` `postgres` service vs an
  ephemeral container spawned by the test — decide in planning.

## Test Strategy
- Level: unit+integration
- UI Tests: n/a (CLI; the engine only reuses the existing picker/progress labels)
- Tools: `go test ./...`; integration test drives a real postgres container via
  docker (same pattern preflight already uses: `docker run`/`docker exec`)
- Coverage Target: 80%
