# Verification table (all fetches returned summaries by a small model; VERBATIM = the summary quoted the exact text)

url | status | verdict | note
https://news.ycombinator.com/showhn.html | 200 | VERBATIM | all 5 quotes present (5th continues "; anybody can do that now.")
https://news.ycombinator.com/newsguidelines.html | 200 | VERBATIM | all 3
https://keydiscussions.com/2026/03/09/hacker-news-moves-toward-restricting-show-hn-posts-amid-the-ai-slop-wave/ | 200 | VERBATIM | page says "discussing throttling Show HN somehow, responding in a thread with 600+ upvotes entitled 'Ask HN: Please restrict new accounts from posting'"
https://arxiv.org/abs/2511.04453 | 200 | VERBATIM | star figures exact; Show HN no advantage + timing confirmed
https://www.producthunt.com/launch | 200 | VERBATIM | both quotes + "best day is the day you're most prepared"
https://help.producthunt.com/en/articles/3615694-community-guidelines | 200 | PARAPHRASE-OK | summary elided the tail of the sentence with "..."; meaning and the removal/loss-of-access consequence preserved
https://www.producthunt.com/launch/how-product-hunt-works | 200 | VERBATIM |
https://lobste.rs/about | 200 | PARAPHRASE-OK | "self-promo ... quarter" verbatim; 70-day limits confirmed (invites, unseen domains, flag); summary listed meta/announce tags, did NOT confirm 'show' tag or "edits"; "New users can't send invites" and the "next five or ten years" tail not seen verbatim
https://redship.io/blog/reddit-self-promotion-rules-2026 | 200 | PARAPHRASE-OK | 10% quote returned as "...probably a spammer." (tail "in Reddit's eyes" not shown); website/redditor quote verbatim
https://dev.to/terms | 200 | VERBATIM | both
https://github.com/avelino/awesome-go/blob/main/CONTRIBUTING.md | 200 | VERBATIM | all 4; page also requires go.mod + a semver release (not in worker file)
https://github.com/ramnes/awesome-mongodb | 200 | VERBATIM | sections exist (mgob under Administration, migrate-mongo under Development, mongo-connector under Data)
https://github.com/shlomi-noach/awesome-mysql | 200 | VERBATIM | Backup section with MyDumper and Xtrabackup present
https://goreleaser.com/customization/homebrew_casks/ | 200 | VERBATIM | "Since v2.10" present; page does NOT state macOS-only (worker hedged as unverified)
https://goreleaser.com/customization/scoop/ | 200 | VERBATIM |
https://docs.brew.sh/Package-Acceptance-Policy | 200 | PARAPHRASE-OK | thresholds 30/30/75 and 90/90/225 and 30-day rule confirmed, but returned as separate fragments, not the single worker sentence
https://pkg.go.dev/about | 200 | VERBATIM |
https://goreleaser.com/customization/docker/ | 200 | VERBATIM | v2.12 deprecation confirmed
https://console.dev/selection-criteria | 200 | VERBATIM | "hello@console.dev" email and the self-service signup criterion
https://dev.to/rophilogene/a-tool-to-seed-your-dev-database-with-real-data-5gj5 | 200 | PARAPHRASE-OK | the summary returned "...does not reflect the real-world data" and then "difficult to maintain"; worker's ending "and painful to keep updated" not confirmed
https://hn.algolia.com/api/v1/search?query=replibyte&tags=story | 200 | VERBATIM | 129/78/2022-04-26; 222/22/2022-07-10; Greenmask 94/19/2024-10-16 all exact
https://hn.algolia.com/api/v1/search?query=neosync&tags=story | 200 | MISMATCH | cited row 246/44/2024-05-22 and archived 1/1/2025-09-22 exact. BUT the worker's claim that the first Show HN (2023-12-07) got 24 points is wrong: live is 4 points, 1 comment, created 2023-12-08
https://hn.algolia.com/api/v1/search?query=pgsync&tags=story | 200 | VERBATIM | 166/35/2020-03-24
https://hn.algolia.com/api/v1/search?query=lazydocker&tags=story | 200 | VERBATIM | 340/47/2019-06-30; resubmissions 259 (2021-12), 481 (2023-07), 74 (2024-11) confirmed (submitted by other users, titles differ)
https://hn.algolia.com/api/v1/search?query=dbmate&tags=story | 200 | VERBATIM | 3/3/2015-12-02; 79/13/2024-06-16
https://hn.algolia.com/api/v1/search?query=snaplet&tags=story | 200 | PARAPHRASE-OK | cited row 13 points/4 comments/2023-10-07 exact; shutdown 4/1/2024-07-01 and open-source 4/1/2024-08-14 exact; the worker's "13, 4 and 5 points" for Show HN posts is unsupported (only one Show HN row; the 4 is the comment count)
https://hn.algolia.com/api/v1/search?query=greenmask&tags=story | 200 | VERBATIM | 94/19/2024-10-16

## Totals
VERBATIM: 20 | PARAPHRASE-OK: 6 | MISMATCH: 1 | UNREACHABLE: 0 (27 URLs; reddit skipped, none cited as findings)
