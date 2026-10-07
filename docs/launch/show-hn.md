# Show HN — material (REWRITE IN YOUR OWN WORDS before posting)

HN guideline: "Please don't post generated text or AI-edited text." This file is a fact sheet
and a structure, not the post. Type the title and the text yourself from it.

**URL to submit:** https://github.com/phuthuycoding/dbclone (the repo, not the website —
Show HN wants something people can run).

## Title candidates (≤ 80 chars, name the databases, name the job)

1. Show HN: Dbclone – Clone MongoDB/MySQL from prod or staging to local, Docker only
2. Show HN: Dbclone – Copy a staging MongoDB/MySQL database to your laptop in one command
3. Show HN: Dbclone – Stream MongoDB/MySQL databases between servers with nothing installed

Why this shape: Neosync's vague first post got 4 points; the relaunch titled
"…for Postgres and MySQL" got 246. Replibyte's "A tool to seed your dev database with real
data" got 129.

## Facts to build the text from (all true per README / code)

- Why I built it: our team clones a shared staging MongoDB + MySQL to laptops every week.
  mongodump/mysqldump by hand meant installing matching client versions on every machine,
  dump files on disk, and no idea how far along a 3 GB restore was.
- What it does: picks source and target from named profiles (`staging`, `prod`, `local`),
  lists databases, lets you tick collections/tables, then streams each dump straight into the
  restore. No dump files on disk.
- Nothing to install on the host: the dump/restore binaries are the ones already inside the
  official `mongo` and `mysql` Docker images on your machine. Works between two remote
  servers too; the local containers are just the pipe.
- Parallelism: one global pool of streams (`-j`, default 8) shared across all databases,
  biggest work dispatched first. Mongo collections ≥ 64 MiB get their own stream; MySQL
  tables are split into size-balanced groups (`-w`).
- Flaky links: every stream retried 3× with backoff; restores drop-and-recreate so a retry
  starts clean. Auth errors fail immediately instead of retrying.
- Safety: writing to anything but `local` requires typing the target's name; `-fresh` (drop
  whole DB) only works on `local`; a server is never cloned onto itself. Passwords go to the
  tools via env vars, never argv; profiles are `0600`; logs are scrubbed of `user:password@`.
- Honest limits (say them; HN rewards it): MySQL table groups each take their own snapshot,
  so the copy is not one consistent point in time — dev data, not a backup tool. Needs Docker
  with containers from the official images. Postgres is not there yet; the driver interface is
  one package per engine and it is the obvious next one.
- Install: `brew install phuthuycoding/tap/dbclone`, Scoop, `go install`, or a binary from
  Releases (linux/macOS/windows, amd64/arm64). MIT.

## Suggested structure (6–8 lines, first person)

1. One sentence: what it is and the exact pain.
2. One sentence: the trick (dump piped into restore inside the official images).
3. Two sentences: scheduling + retries, with the numbers.
4. One sentence: the safety rails.
5. One sentence: what it is NOT (consistency, backup).
6. Ask: what engine next, what breaks on your setup.

## Day-of checklist

- Post Tue–Thu, 20:00–22:00 Vietnam time (morning US Pacific). Not an official HN rule;
  it is where the audience is awake.
- Stay in the thread for ~6 hours. Answer every comment, including the harsh ones.
- Do NOT ask anyone to upvote. Do not post the link in team chats with "please upvote".
- If it does not take off, it is allowed to resubmit later on a big release (Postgres
  driver). lazydocker hit the front page 4 times over 5 years.
