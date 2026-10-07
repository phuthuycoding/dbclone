# Worker 1: launch rules for HN Show HN, Product Hunt, Lobsters (2025-2026)

Sub-question: current submission rules, best practices, timing, recent changes for Show HN, Product Hunt, Lobsters, for an OSS Go CLI (clone MongoDB/MySQL to local Docker).

Note: WebFetch returns model-summarized text, so quotes are as returned by the fetch tool; verifier should compare against the live pages.

## Findings

According to https://news.ycombinator.com/showhn.html: "Show HN is for something you've made that other people can play with."
→ Runnable software such as a CLI qualifies for Show HN.

According to https://news.ycombinator.com/showhn.html: "The project must be something you've worked on personally and which you're around to discuss"
→ The author must be present to answer comments.

According to https://news.ycombinator.com/showhn.html: "Please make it easy for users to try your thing out, ideally without barriers such as signups or emails."
→ Repo plus install instructions (no signup) fits; a landing page or waitlist does not.

According to https://news.ycombinator.com/showhn.html: "Off topic: blog posts, sign-up pages, newsletters, lists, and other reading material."
→ A website/blog post alone is not a valid Show HN; link the repo or something runnable.

According to https://news.ycombinator.com/showhn.html: "The project should be non-trivial. Don't post quickly-generated one-offs"
→ Low-effort or quickly generated tools are explicitly discouraged.

According to https://news.ycombinator.com/newsguidelines.html: "Please don't use HN primarily for promotion. It's ok to post your own stuff part of the time, but the primary use of the site should be for curiosity."
→ Account should show non-promotional participation.

According to https://news.ycombinator.com/newsguidelines.html: "Don't solicit upvotes, comments, or submissions."
→ Do not ask friends or communities to upvote the Show HN.

According to https://news.ycombinator.com/newsguidelines.html: "Please don't post generated text or AI-edited text. HN is for conversation between humans."
→ Write the title, post text and replies yourself (2025-2026 relevant policy on AI text).

According to https://keydiscussions.com/2026/03/09/hacker-news-moves-toward-restricting-show-hn-posts-amid-the-ai-slop-wave/: HN's top moderator was "discussing throttling Show HN somehow, responding in a thread" (fetch summary; the page links a thread "Ask HN: Please restrict new accounts from posting"); no concrete rule was announced.
→ As of March 2026 Show HN restriction was under discussion only; secondary source, no verbatim dang quote obtained.

According to https://arxiv.org/abs/2511.04453: "repositories gain an average of 121 stars within 24 hours, 189 stars within 48 hours, and 289 stars within a week of HN exposure."
→ Empirical benefit of HN exposure for AI/LLM tool repos (138 launches 2024-2025); the same paper reportedly found the Show HN tag gave no statistical advantage once other variables were controlled, and that posting time matters (fetch summary, not a verbatim quote).

According to https://www.producthunt.com/launch: "12:01 am Pacific Time is the best time to launch for makers that are planning ahead"
→ Official timing guidance; the same page says the best day is the one you are most prepared for (fetch summary, no fixed weekday).

According to https://www.producthunt.com/launch: "Company accounts are prohibited."
→ Launch from a personal maker account.

According to https://help.producthunt.com/en/articles/3615694-community-guidelines: "Mass messaging users, asking for upvotes, using bots, incentivizing upvotes, and any other form of artificially increasing activity on your contribution is not acceptable."
→ Do not ask for upvotes; ask for visits and comments instead. Violations may remove the contribution and cost contribution access.

According to https://www.producthunt.com/launch/how-product-hunt-works: "The homepage leaderboard changes throughout the day based on the number of upvotes, comments, time since submission, and other factors."
→ Ranking mixes upvotes, comments and recency. The same page requires the product to be new or substantively updated, usable, and offer unique value (fetch summary).

According to https://lobste.rs/about: "self-promo should be less than a quarter of one's stories and comments."
→ Lobsters self-promotion rule of thumb (about 25% ceiling); the page also says authors should not use the site as "a write-only tool for product announcements or driving traffic to their work" (fetch of /about#invitations).

According to https://lobste.rs/about: "New users can't send invites" for their first 70 days.
→ Lobsters is invite-only. New users (green usernames) cannot use the 'show' tag, submit links to previously unseen domains, flag, or suggest edits during the first 70 days. An unseen domain such as a new project site/repo host is therefore restricted for new accounts.

According to https://lobste.rs/about: submission test "Will this improve the reader's next program? Will it deepen their understanding of their last program? Will it be more interesting in five or ten years?"
→ Lobsters favors technically substantive posts, so a design write-up of how the cloning works fits better than an announcement.

## Not found
- not found: official Product Hunt 2025-2026 changelog of rule changes (e.g. retirement of "Coming Soon", vote-integrity updates) — tried web search; only third-party blogs (innmind, causo, getlaunchlist, mean.ceo) claim it, none fetched; /launch page said no specific changes.
- not found: PH weekday recommendation from an official page — /launch says "the day on which you're most prepared"; third-party "Tue/Wed" claims unverified.
- not found: concrete Show HN rule change 2025-2026 beyond the AI-text guideline — tried search and keydiscussions (secondary, no verbatim dang text); did not fetch the HN thread itself.
- not found: verbatim best-time-to-post for Show HN from an official source — only third-party claims in search snippets (6-9 AM PT, Tue-Thu), not fetched.
- not found: Lobsters 'show' tag description text and any Lobsters AI/LLM policy on /about; the page did not mention AI. Did not fetch lobste.rs/tags or /rules.
- not found: Product Hunt policy on AI content or new accounts in community guidelines (page does not mention it).

## Calls used: 13/15
