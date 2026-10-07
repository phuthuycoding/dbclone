# Reddit posts

Before posting anywhere: open `reddit.com/r/<sub>/about/rules` logged in and read the
rules; they were login-walled during research and are unverified. Keep self-posts under
10% of your activity; comment for real in these subs for 1–2 weeks first.

## r/golang — text post (technical, Go-first), Thu 2026-10-15

**Title:** dbclone: a Go CLI that pipes mongodump/mysqldump into the restore inside Docker, with a weighted worker pool

**Body (rewrite freely):**

I wrote a small CLI to clone MongoDB/MySQL databases between servers (mostly staging → my
laptop). The Go-specific parts people here might find interesting:

- The dump and restore are two `exec.CommandContext` processes run through `docker exec`,
  with the dump's stdout wired to the restore's stdin. No temp files, and the progress bar
  reads byte counts off the pipe.
- All streams from all databases share one weighted pool (`-j` slots, default 8). Big
  Mongo collections (≥ 64 MiB) and size-balanced groups of MySQL tables each get a stream,
  dispatched biggest first.
- Retries: 3 attempts with backoff per stream; restores drop-and-recreate so a retry starts
  clean. Errors that can't be fixed by retrying (auth) short-circuit.
- Engines are adapters behind one `driver.Driver` interface, one package per engine. Adding
  Postgres should touch nothing in scheduling/progress/prompts.
- Safety rails because this thing can write to a server: non-local targets require typing
  the target's name; `-fresh` only on `local`; passwords only via env vars, never argv.

Repo: https://github.com/phuthuycoding/dbclone (MIT). Happy to hear what you'd do
differently in the pool/pipe code.

## r/devops — text post, Sat 2026-10-17

**Title:** We stopped maintaining a "how to get staging data on your laptop" wiki page — wrote a CLI that streams MongoDB/MySQL through your local Docker containers

Body: 5–6 lines. Lead with the problem (version-skew of client tools, dump files on disk,
no progress on a 3 GB restore). Then: the pipe trick, the pool, the honest limit (MySQL
snapshot not consistent across tables, not a backup tool). Link.

## r/mongodb — Mon 2026-10-19

**Title:** Open-source CLI: clone a MongoDB database (or just some collections) from staging to local, using the mongo image's own mongodump/mongorestore

Body: 4 lines. Mention per-collection streams ≥ 64 MiB, retries, picker with `/` filter.
Ask about Atlas / SRV edge cases.

## r/mysql — Wed 2026-10-21

**Title:** Open-source CLI: clone a MySQL database between servers, tables split into size-balanced mysqldump streams

Body: 4 lines. Mention `--single-transaction` per group, DEFINER stripping for non-root
remote targets, `log_bin_trust_function_creators` note, the consistency caveat.

## r/selfhosted — ONLY in the "New Project Friday" megathread until 2027-01-02 (repo < 3 months old)

Comment (not a post), Fri 2026-10-16: 3 lines. "Clone your MongoDB/MySQL between servers
with only Docker installed; streams, retries, live progress; MIT." + link.

## r/commandline — optional, after the others

One screenshot/GIF post of the progress view. Title: "dbclone — live multi-stream progress
for database clones (Go)".
