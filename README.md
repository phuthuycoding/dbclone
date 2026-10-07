# dbclone

[![CI](https://github.com/phuthuycoding/dbclone/actions/workflows/ci.yml/badge.svg)](https://github.com/phuthuycoding/dbclone/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/phuthuycoding/dbclone)](https://github.com/phuthuycoding/dbclone/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/phuthuycoding/dbclone.svg)](https://pkg.go.dev/github.com/phuthuycoding/dbclone)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Website:** https://phuthuycoding.github.io/dbclone/

Clone MongoDB and MySQL databases between **any two connections** — staging → local,
prod → local, local → staging, staging → another server — whole databases or just the
tables/collections you pick, with parallel streaming, retries and a live progress view in the
terminal.

```
Cloning 3 databases prod → local · pool 8 streams · max 4 streams/db

⠹ mongo:shop   orders ⇉2         [==========>-------------]  42%  1.2 GiB / ~2.9 GiB  18.4 MiB/s  01:07
✓ mysql:app    done              [========================] 100%  235.6 MiB           57.2 MiB/s  00:04
· mysql:crm    waiting for slot  [------------------------]   0%    0.0 b / ~80.0 MiB
```

## Why

- **Any direction.** Save your servers as named profiles (`staging`, `prod`, …) and clone from
  any of them to any other; your local Docker containers are the built-in profile `local`.
- **Nothing to install on the host.** Dump and restore run with the tools that already ship in
  the official `mongo` and `mysql` images, inside your local containers.
- **Streams, never stages.** Each dump is piped straight into the restore; no dump files on disk.
- **Fast.** One global pool of streams shared by all databases, biggest work first. Big Mongo
  collections and groups of MySQL tables each get their own stream.
- **Survives flaky links.** Every stream is retried (3 attempts, backoff); restores
  drop-and-recreate, so a retry starts clean. Errors no retry can fix — access denied, for
  one — fail at once.
- **Hard to misuse.** Writing to anything but `local` means typing the target's name;
  `-fresh` (drop whole databases) only works on `local`; a server is never cloned onto itself.
- **Secrets stay secret.** Passwords reach the tools as environment variables, never on a
  command line; profiles are stored with mode `0600`; every log line is scrubbed of
  `user:password@`.

## Requirements

- Docker, with local MongoDB / MySQL running from the official images (`mongo`, `mysql`).
  Their tools run every dump and restore — also when neither side is local. By default the
  containers are named `mongodb` and `mysql`; override with `DBCLONE_MONGO_CONTAINER` /
  `DBCLONE_MYSQL_CONTAINER`.
- The local containers must carry their root credentials in the environment —
  `MONGO_INITDB_ROOT_USERNAME` / `MONGO_INITDB_ROOT_PASSWORD` for Mongo and
  `MYSQL_ROOT_PASSWORD` for MySQL — which is how the official images are normally configured.
  They are used for the `local` profile; you never type local credentials.
- Remote servers must be reachable **from inside** the containers.

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
brew install phuthuycoding/tap/dbclone        # macOS (Homebrew)
scoop bucket add phuthuycoding https://github.com/phuthuycoding/scoop-bucket && scoop install dbclone   # Windows
go install github.com/phuthuycoding/dbclone@latest   # any OS with Go
```

or download a binary from [Releases](https://github.com/phuthuycoding/dbclone/releases)
(linux / macOS / windows, amd64 / arm64).

### Or run everything with Docker Compose

No Go, no binary: the repo's [`compose.yaml`](compose.yaml) starts local MongoDB and MySQL from
the official images, already carrying the root credentials dbclone expects, and builds the CLI
into a small image that talks to them over the Docker socket.

```bash
git clone https://github.com/phuthuycoding/dbclone && cd dbclone
docker compose up -d                      # mongodb + mysql
docker compose run --rm dbclone -check    # ✓ Docker, ✓ mongo, ✓ mysql
docker compose run --rm dbclone           # add profiles, pick FROM / TO / databases
docker compose run --rm dbclone -from staging -only mongo:shop -yes
```

Profiles persist in the `dbclone-config` volume; logs land in `./logs`. Override ports or
names with `DBCLONE_MONGO_PORT`, `DBCLONE_MYSQL_PORT`, `DBCLONE_MONGO_CONTAINER`,
`DBCLONE_MYSQL_CONTAINER`, and the local root password with `DBCLONE_LOCAL_PASSWORD`
(default `dbclone`; it only guards the databases on your machine).

## Usage

```bash
dbclone            # first run: add profiles, then pick FROM, TO and the databases
dbclone -setup     # add, edit or delete profiles
```

1. **Profiles** — a profile is a named connection: a MongoDB URI and/or MySQL host, port, user
   and password. On first run dbclone checks the local setup and opens the profile manager.
   Profiles live in `<user config dir>/dbclone/profiles.json` (mode `0600`; `-config` to use
   another file). An old `.env.staging` in the current directory is imported once as `staging`.
   Example: [`profiles.example.json`](profiles.example.json).
2. **From / to** — pick the source and the target (default `local`). Both connections are
   tested right away; if one fails you can edit that profile and retry.
3. **Databases** — tick the databases to clone; ones that already exist on the target are marked.
4. **Narrow down** (optional) — restrict any of them to specific tables/collections
   (`space` toggle, `ctrl+a` toggle all, `/` filter).
5. **Confirm** — what will be written is listed first. For any target other than `local`
   you must type the target's name.

Non-interactive:

```bash
dbclone -from staging -only mongo:shop,mysql:app -yes          # staging → local
dbclone -from prod -only mongo:shop.orders,mysql:app.users     # some objects, prod → local
dbclone -from local -to staging -only mysql:app -confirm staging   # local → staging
dbclone -from staging -all -fresh -j 3 -w 2                     # identical local copy, gentle on the source
```

| Flag | Default | Meaning |
|---|---|---|
| `-from` | asked | source profile (`local` = your Docker containers) |
| `-to` | `local` | target profile |
| `-confirm` | | the target's name; required to write to a non-local target without the prompt |
| `-only` | | `engine:db` or `engine:db.object`, comma separated; skips the picker |
| `-all` | off | clone every database of the source, no picker |
| `-fresh` | off | drop each wholly-cloned database on the target first — `local` target only |
| `-yes` | off | do not ask before overwriting databases on the `local` target |
| `-j` | 8 | global pool: concurrent dump→restore streams across all databases |
| `-w` | 4 | max streams for one database |
| `-config` | user config dir | profiles file |
| `-logs` | `logs` | per-database tool output, one folder per run |
| `-setup` | off | manage profiles before cloning |
| `-check` | | check the local setup (Docker, containers) and exit |
| `-version` | | print the version |

## What gets overwritten

| On the target | Object exists on the source | Object exists only on the target |
|---|---|---|
| default | dropped and recreated | kept |
| `-fresh`, whole database (`local` only) | dropped and recreated | **dropped** (the database is dropped first) |
| subset of a database | dropped and recreated (selected objects only) | kept, even with `-fresh` |

## How it works

```
                  ┌──────────────── local container ────────────────┐
source ──────────▶│ mongodump / mysqldump ──pipe──▶ mongorestore / mysql │──────────▶ target
(any profile)     └──────────────────────────────────────────────────┘   (any profile)
```

- **MongoDB** — collections of 64 MiB or more get their own `mongodump --collection | mongorestore`
  stream; everything else (small collections, views) travels in one more stream that runs
  several collections in parallel. Database names in the URI are handled for you.
- **MySQL** — tables are split into up to `-w` size-balanced groups, each its own
  `mysqldump --single-transaction | mysql` stream; triggers, views, routines and events follow
  once the data is in.
- **Scheduling** — streams from all databases share one weighted pool of `-j` slots and are
  dispatched biggest first, so a slot freed by a small database goes straight to a big one.

Things to know:

- MySQL table groups each take their own snapshot, so the copy is not one consistent
  point in time across tables. Fine for development data, not a backup tool.
- The MySQL source user needs `SELECT`, `SHOW VIEW` and `TRIGGER`. Without `EVENT`, events are
  skipped with a warning; routines the user cannot see are skipped silently by mysqldump.
- When only some objects of a MySQL database are cloned, routines and events are not copied.
- Writing to a **remote** MySQL: `DEFINER` clauses are removed so a non-root user can create
  views, routines and triggers (they get that user as definer). If the server has binary
  logging on, creating triggers and routines needs `SUPER` or
  `log_bin_trust_function_creators=1` — without it the data still lands and the error says so.
  On the `local` target, binary logging is turned off for the import session instead.

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
