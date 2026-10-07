---
title: Cloning a staging MongoDB and MySQL to your laptop without installing a single database client
published: false
tags: go, docker, mongodb, mysql
canonical_url: https://phuthuycoding.github.io/dbclone/
---

Every week someone on my team needs a copy of our staging data on their laptop. Two
databases, MongoDB and MySQL, a few gigabytes. For years the answer was a wiki page:
install `mongodump` of the right major version, install the MySQL client, run four
commands, wait, and hope the SSH session doesn't drop halfway through a 3 GB restore.

I got tired of the wiki page and wrote [dbclone](https://github.com/phuthuycoding/dbclone),
a small Go CLI. This post is about the three decisions that made it pleasant, and the one
limit you should know before you trust it.

## The trick: your Docker images already contain the tools

If you run MongoDB or MySQL locally, you almost certainly run the official `mongo` and
`mysql` images. Those images ship `mongodump`, `mongorestore`, `mysqldump` and `mysql`.
dbclone never installs anything on your host; every dump and restore is a `docker exec`
into the container you already have:

```
                  ┌──────────────── local container ────────────────┐
source ──────────▶│ mongodump / mysqldump ──pipe──▶ mongorestore / mysql │──────────▶ target
(any profile)     └──────────────────────────────────────────────────┘   (any profile)
```

Two consequences fell out of this that I did not plan for:

- **Version skew disappears.** The client that dumps is the same major version as the
  server that restores, because they are the same image.
- **Any direction works.** The container is just the pipe. `staging → local` is the
  common case, but `local → staging` and `staging → another server` go through the same
  code path. Your local containers are simply a built-in profile called `local`.

## Streams, never stages

Each dump is piped straight into its restore. There is no dump file on disk, so there is no
"out of disk at 90%" and no cleanup step.

Big work gets its own stream. A MongoDB collection of 64 MiB or more becomes one
`mongodump --collection | mongorestore` pipe; everything smaller travels together in one
extra stream. MySQL tables are split into up to `-w` size-balanced groups, each its own
`mysqldump --single-transaction | mysql` pipe, and triggers, views, routines and events
follow once the data has landed.

All streams from all databases share one weighted pool of `-j` slots (default 8) and are
dispatched biggest first, so a slot freed by a tiny database goes straight to the large one
instead of waiting for its siblings.

```
Cloning 3 databases prod → local · pool 8 streams · max 4 streams/db

⠹ mongo:shop   orders ⇉2         [==========>-------------]  42%  1.2 GiB / ~2.9 GiB  18.4 MiB/s  01:07
✓ mysql:app    done              [========================] 100%  235.6 MiB           57.2 MiB/s  00:04
· mysql:crm    waiting for slot  [------------------------]   0%    0.0 b / ~80.0 MiB
```

## Flaky links and the rails that stop you from hurting yourself

Every stream is retried three times with backoff. Because restores drop and recreate the
object, a retry starts clean instead of appending to a half-written table. Errors that no
retry can fix, such as access denied, fail at once.

Then the part I care about most, because the tool can write to a server:

- Writing to anything but `local` means typing the target's name, interactively or with
  `-confirm staging`.
- `-fresh` (drop whole databases first) only works when the target is `local`.
- A server is never cloned onto itself.
- Passwords reach the tools as environment variables, never on a command line. Profiles are
  stored with mode `0600`. Every log line is scrubbed of `user:password@`.

## The limit you should know

MySQL table groups each take their own `--single-transaction` snapshot. The copy is
therefore not one consistent point in time across tables. That is fine for development
data, which is what this is for. It is not a backup tool, and I would not use it as one.

## Using it

```bash
brew install phuthuycoding/tap/dbclone      # or scoop, go install, or a binary from Releases
dbclone -check                              # Docker + containers OK? prints the fix if not
dbclone                                     # first run: add a profile, pick FROM, TO, databases
dbclone -from staging -only mongo:shop,mysql:app -yes
```

The engine interface is one package per database (`internal/driver/<engine>/`), and
PostgreSQL is the obvious next one. If you try it on a setup that breaks, an issue with the
`logs/` folder from that run is the most useful thing you can send me.

Repo: https://github.com/phuthuycoding/dbclone — MIT.
