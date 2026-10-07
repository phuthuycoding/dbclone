# X / LinkedIn (post the same day as Show HN, with the GIF)

## X — single post + 1 reply

Clone a MongoDB or MySQL database from staging to your laptop in one command — with
nothing installed except Docker.

dbclone pipes mongodump/mysqldump straight into the restore inside the official images.
Parallel streams, retries, live progress. Go, MIT.

https://github.com/phuthuycoding/dbclone

(reply) The limit, so nobody gets surprised: MySQL tables are dumped in parallel groups,
each with its own snapshot. Dev data, not a backup tool. Postgres is next.

## LinkedIn — short, first person

Every week someone on my team needed staging data on their laptop, and the answer was a
wiki page nobody liked. So I wrote dbclone: a small open-source Go CLI that clones
MongoDB/MySQL between any two servers using the dump/restore tools already inside your
Docker containers. No dump files, parallel streams, retries, a progress bar you can
actually read.

It is MIT and on GitHub: https://github.com/phuthuycoding/dbclone
If you try it and something breaks on your setup, I want to hear about it.
