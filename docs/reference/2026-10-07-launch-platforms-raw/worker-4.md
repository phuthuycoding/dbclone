# Worker 4: launch prior art for DB/dev CLI tools

Sub-question: How did comparable OSS DB/dev CLI tools launch and what worked?
Caveat: the HN Algolia fetches were summarised by the fetch tool into tables. Titles and points are as returned, not raw JSON. A verifier should re-check against the HN item pages.

## Findings

According to https://hn.algolia.com/api/v1/search?query=replibyte&tags=story: "Show HN: A tool to seed your dev database with real data | 129 | 78 | 2022-04-26 | https://github.com/Qovery/replibyte"
→ Replibyte launched as a plain Show HN whose title leads with the job-to-be-done and not the product name. It got 129 points and 78 comments.

According to https://hn.algolia.com/api/v1/search?query=replibyte&tags=story: "Replibyte – Seed your database with real data | 222 | 22 | 2022-07-10"
→ A second, non-Show-HN submission about 2.5 months later scored higher (222 points). The tool was resubmitted when it had more features, so a repeat launch is viable.

According to https://dev.to/rophilogene/a-tool-to-seed-your-dev-database-with-real-data-5gj5: "As a developer, creating a fake dataset for running tests is tedious. Plus, it does not reflect the real-world data and painful to keep updated."
→ The author's launch post on dev.to opens with a pain-point statement, and Qovery (a company) backed the project. Replibyte's GitHub stars were reported as 4.4k via a search snippet only (not fetched).

According to https://hn.algolia.com/api/v1/search?query=neosync&tags=story: "Show HN: Neosync – Open-Source Data Anonymization for Postgres and MySQL | 246 | 44 | 2024-05-22"
→ Neosync's first Show HN (2023-12-07, "Open-Source Data Replication and Anonymization") got 24 points. The reframed relaunch on 2024-05-22 got 246 points. The relaunch title named the concrete DBs and the "Open-Source" angle.

According to https://hn.algolia.com/api/v1/search?query=neosync&tags=story: "Neosync OSS repository is archived | 1 | 1 | 2025-09-22"
→ Neosync's OSS repo was archived in Sept 2025. A search snippet (not fetched) says Grow Therapy acquired Neosync on 2025-09-25. Good HN traction did not mean a durable standalone business.

According to https://hn.algolia.com/api/v1/search?query=pgsync&tags=story: "PgSync: Sync Postgres data between databases | 166 | 35 | 2020-03-24 | https://github.com/ankane/pgsync"
→ pgsync reached 166 points with a plain-descriptive title, not a Show HN. Ankane did the posting or someone else did; this is unknown. The title has no marketing language.

According to https://hn.algolia.com/api/v1/search?query=lazydocker&tags=story: "Lazydocker: a terminal GUI for Docker | 340 | 47 | 2019-06-30"
→ Lazydocker's launch-day HN submission scored 340 points. It was resubmitted three more times: 2021-12 (259), 2023-07 (481), 2024-11 (74). Repeated resubmission of the same repo kept producing front-page hits.

According to https://hn.algolia.com/api/v1/search?query=dbmate&tags=story: "Show HN: A lightweight, framework-agnostic database migration tool | 3 | 3 | 2015-12-02"
→ dbmate's 2015 Show HN got 3 points. A 2024-06-16 resubmission ("Dbmate: A lightweight, framework-agnostic database migration tool") got 79. The launch flopped, and adoption came later via other channels (unknown which).

According to https://hn.algolia.com/api/v1/search?query=snaplet&tags=story: "Show HN: Seed your Postgres database with production-like data | 13 | 4 | 2023-10-07"
→ Snaplet's Show HN posts were small (13, 4 and 5 points). Its other submissions: "Snaplet Is Shutting Down" (4 points, 2024-07-01) and "Snaplet is now open source" (4 points, 2024-08-14). A funded venture-style launch got little HN traction.

According to https://hn.algolia.com/api/v1/search?query=replibyte&tags=story: "Show HN: Greenmask 0.2 – Database anonymization tool | 94 | 19 | 2024-10-16"
→ A later competitor in the same space got 94 points with a versioned Show HN title.

## Not found
- not found: author write-up "how I got N GitHub stars" for any of these tools. Tried searches on lazydocker/Jesse Duffield and Replibyte, and the dev.to post.
- not found: verbatim Snaplet founder shutdown quote on a fetched page. plushcap.com paraphrased only; the supabase.com/snaplet.dev posts were not fetched (cap). The search snippet attributes a quote to Peter Pistorius (unverified).
- not found: the Jesse Duffield "posted on reddit ... virtually zero responses" story as a fetched page. It appeared only in a search snippet, source lookingatcomputer.substack.com/p/q-and-a-with-jesse-duffield (not fetched).
- not found: what channel drove pgsync and dbmate adoption (Show HN vs Rails/Ruby community vs Homebrew). Only HN Algolia data was fetched.
- not found: star counts at launch time for any tool (only the current 4.4k for Replibyte, from a snippet).

## Calls used: 15/15
