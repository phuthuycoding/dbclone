# dbclone

Clone MongoDB and MySQL databases from a remote (staging) server into your local Docker
containers — whole databases or just the tables/collections you pick — with parallel
streaming, retries and a live progress view in the terminal.

```
⠹ mongo:shop   orders ⇉2         [==========>-------------]  42%  1.2 GiB / ~2.9 GiB  18.4 MiB/s  01:07
✓ mysql:app    done              [========================] 100%  235.6 MiB           57.2 MiB/s  00:04
· mysql:crm    waiting for slot  [------------------------]   0%    0.0 b / ~80.0 MiB
```

## Why

- **Nothing to install on the host.** Dump and restore run with the tools that already ship in
  the official `mongo` and `mysql` images, inside your local containers, with client versions
  that match the local server.
- **Streams, never stages.** Each dump is piped straight into the restore; no dump files on disk.
- **Fast.** One global pool of streams shared by all databases, biggest work first. Big Mongo
  collections and groups of MySQL tables each get their own stream.
- **Survives flaky links.** Every stream is retried (3 attempts, backoff); restores
  drop-and-recreate, so a retry starts clean.
- **Secrets stay secret.** Passwords reach the containers as environment variables, never on a
  command line, and every log line is scrubbed of `user:password@` before it is written.

## Requirements

- Docker, with local MongoDB / MySQL running from the official images
  (`mongo`, `mysql`). By default the containers are named `mongodb` and `mysql`; override with
  `DBCLONE_MONGO_CONTAINER` / `DBCLONE_MYSQL_CONTAINER`.
- The local containers must have been created with their root credentials in the environment —
  `MONGO_INITDB_ROOT_USERNAME` / `MONGO_INITDB_ROOT_PASSWORD` for Mongo and
  `MYSQL_ROOT_PASSWORD` for MySQL — which is how the official images are normally configured.
  dbclone reads them inside the container; you never type local credentials.
- The source server must be reachable **from inside** the containers.

Not sure your machine is ready? `dbclone -check` lists each requirement with ✓/✗ and the exact
command to fix anything missing (Docker not installed, daemon not running, container missing
or not from the official image). dbclone runs the same check on every start, and an engine
whose local container is unusable is simply skipped.

```
Checking local setup
  ✓ Docker  29.8.2
  ✓ mongo: local container "mongodb"  running
  ✗ mysql: local container "mysql" not found
      create one:  docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=<password> mysql:8.4
      or use an existing container:  DBCLONE_MYSQL_CONTAINER=<name> dbclone
```

## Install

```bash
go install github.com/phuthuycoding/dbclone@latest
```

or download a binary from [Releases](https://github.com/phuthuycoding/dbclone/releases).

## Usage

```bash
dbclone            # first run asks for the source connections, then lets you pick
dbclone -setup     # re-enter the connections
```

1. **Connections** — on first run dbclone checks the local setup, then asks for the source
   MongoDB URI and/or MySQL host, port, user and password — only for engines whose local
   container is ready; leave one empty to skip it. The connections are tested right away; if
   they fail you can re-enter them. Once they work, dbclone offers to save them to
   `.env.staging` with mode `0600`. See [`.env.staging.example`](.env.staging.example).
2. **Databases** — tick the databases to clone; ones that already exist locally are marked.
3. **Narrow down** (optional) — pick which of those databases to restrict to specific
   tables/collections; each shows every object with its size, all ticked
   (`space` toggle, `ctrl+a` toggle all, `/` filter).
4. **Confirm** — anything that would be overwritten locally is listed before it happens.

Non-interactive:

```bash
dbclone -only mongo:shop,mysql:app -yes                     # whole databases
dbclone -only mongo:shop.orders,mongo:shop.users,mysql:app  # some collections + a whole db
dbclone -all -fresh -j 3 -w 2                               # everything, identical copy, gentle on the source
```

| Flag | Default | Meaning |
|---|---|---|
| `-j` | 8 | global pool: concurrent dump→restore streams across all databases |
| `-w` | 4 | max streams for one database |
| `-only` | | `engine:db` or `engine:db.object`, comma separated; skips the picker |
| `-all` | off | clone every database on the source, no picker |
| `-fresh` | off | drop each wholly-cloned local database first, so local is identical to the source |
| `-yes` | off | do not ask before overwriting local databases |
| `-env` | `.env.staging` | connection file |
| `-logs` | `logs` | per-database tool output, one folder per run |
| `-setup` | off | re-enter the connections |
| `-check` | | check the local setup (Docker, containers) and exit |
| `-version` | | print the version |

## What gets overwritten

| | Object exists on the source | Object exists only locally |
|---|---|---|
| default | dropped and recreated | kept |
| `-fresh`, whole database | dropped and recreated | **dropped** (the database is dropped first) |
| subset of a database | dropped and recreated (selected objects only) | kept, even with `-fresh` |

## How it works

```
                    ┌──────────── local container ────────────┐
source ──network──▶ │ mongodump / mysqldump ─pipe─▶ restore   │──▶ local database
                    └──────────────────────────────────────────┘
```

- **MongoDB** — collections of 64 MiB or more get their own `mongodump --collection | mongorestore`
  stream; everything else (small collections, views) travels in one more stream that runs
  several collections in parallel. Database names in the URI are handled for you.
- **MySQL** — tables are split into up to `-w` size-balanced groups, each its own
  `mysqldump --single-transaction | mysql` stream; views, routines and events follow once the
  tables are in. Binary logging is off for the import session.
- **Scheduling** — streams from all databases share one weighted pool of `-j` slots and are
  dispatched biggest first, so a slot freed by a small database goes straight to a big one.

Things to know:

- MySQL table groups each take their own snapshot, so the copy is not one consistent
  point in time across tables. Fine for development data, not a backup tool.
- The MySQL source user needs `SELECT`, `SHOW VIEW` and `TRIGGER`. Without `EVENT`, events are
  skipped with a warning; routines the user cannot see are skipped silently by mysqldump.
- When only some objects of a MySQL database are cloned, routines and events are not copied.

## Adding an engine

Engines are adapters behind one interface. To add one (PostgreSQL, …), implement
`driver.Driver` in `internal/driver/<engine>/` — connection fields, listing, `Objects`, `Plan`
and `Prepare` — and register it in `drivers` in `main.go`. Scheduling, retries, progress,
prompts and config need no change.

```
main            flags + wiring
internal/ui     terminal prompts and live progress
internal/clone  engine: worker pool, dump → restore streaming, retries
internal/driver adapter interface; one package per engine
internal/preflight checks Docker and the local containers before anything runs
internal/docker runs the engine's own tools inside the local containers
internal/config connection file load/save
```

## License

[MIT](LICENSE)
