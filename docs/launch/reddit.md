# Reddit posts

r/golang rules verified 2026-10-07 in the browser (rule 9 "Must be Go Related" explicitly
allows "Announcements & articles about open source Go libraries or applications"; rule 4 bans
engagement hooks; rule 6 bans "Ask an AI" posts). Other subs: see the verified block in
`docs/reference/2026-10-07-launch-platforms.md` §1.2. Keep self-posts under
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

Repo: https://github.com/phuthuycoding/dbclone (MIT).

(r/golang rule 4, verified 2026-10-07: no "engagement hooks" such as "redundantly asking what
others think" — so end on the facts, not on a question.)

## r/devops — ONLY as a comment in the weekly self-promotion thread (rule 4, verified). Sat 2026-10-17

Start the comment with "I'm the author." (rule 4: affiliation at the top). 5–6 lines, your own
words (rule 5 removes LLM-sounding text). Problem (client version skew, dump files, no
progress on a 3 GB restore) → the pipe trick → the pool → the honest limit → link. No blog
link (rule 4: self-contained).

## r/mongodb — rules still unverified (login wall). Mon 2026-10-19, after checking the rules

**Title:** Open-source CLI: clone a MongoDB database (or just some collections) from staging to local, using the mongo image's own mongodump/mongorestore

Body: 4 lines. Mention per-collection streams ≥ 64 MiB, retries, picker with `/` filter.

## r/mysql — DROPPED. Rule 2 "No Self Promotion" (verified). Only mention dbclone when it
genuinely answers someone's question.

## r/selfhosted — ONLY in the "New Project Megathread" until 2027-01-02 (rule 6, verified)

Comment, Fri 2026-10-16: 3 lines. Rule 2 wants "production ready and have docs" — point at
the README flag table and `-check`. Wednesday exception (rule 5) allows tool posts with the
right flair; still a 3-month-old project is megathread-only, so wait.

## r/commandline — NOT before 2026-11-02 (rule 5: no projects newer than 30 days, verified)

Rule 4: title/text must be human-written; if a meaningful share of the code was
AI-generated the post must say "This software's code is partially AI-generated" — decide
honestly. Rule 8: list similar tools and the difference (manual mongodump/mysqldump,
pgsync / replibyte / neosync for Postgres-first setups). Post the GIF of the progress view.
