# Newsletter / curated submissions

## Console.dev — hello@console.dev (selection criteria: developer is the user, maintained, good docs)

Subject: Submission: dbclone — clone MongoDB/MySQL between servers using the official Docker images

Hi,

I'd like to submit dbclone for consideration.

- What: an open-source (MIT) Go CLI that clones MongoDB and MySQL databases between any
  two connections (staging → local, prod → local, local → staging), whole databases or
  selected tables/collections.
- Why it's different: nothing to install on the host. Dump and restore run with the tools
  inside the official `mongo`/`mysql` images already on your machine, piped directly, no
  dump files. One weighted pool of parallel streams, retries with backoff, live progress.
- Docs: README with every flag and an "what gets overwritten" table; `dbclone -check`
  prints the exact command to fix a missing requirement.
- Repo: https://github.com/phuthuycoding/dbclone
- Site: https://phuthuycoding.github.io/dbclone/
- Install: brew / scoop / go install / binaries for linux, macOS, windows.

Thanks,
Quyen (phuthuycoding)

## Golang Weekly — editor@cooperpress.com (no public submit form; contact page says email)

Subject: Link suggestion: dbclone — Go CLI to clone MongoDB/MySQL via Docker, weighted worker pool

Hi,

A link suggestion for Golang Weekly: dbclone, a Go CLI that clones MongoDB/MySQL
databases between servers by piping mongodump/mysqldump straight into the restore inside
the official Docker images. Interesting bits for Go readers: one weighted worker pool
shared across databases dispatched biggest-first, per-stream retries, and a driver
interface that is one package per engine.

https://github.com/phuthuycoding/dbclone (MIT)

Thanks,
Quyen
