# Worker 3: distribution channels for a Go CLI

Sub-question: requirements/process for awesome-go, awesome-mongodb/mysql, Homebrew tap, Scoop, pkg.go.dev, Docker via goreleaser, newsletters.

## Findings

According to https://github.com/avelino/awesome-go/blob/main/CONTRIBUTING.md: "have at least 5 months of history since the first commit."
→ awesome-go requires a repo at least 5 months old (no star minimum is stated).

According to https://github.com/avelino/awesome-go/blob/main/CONTRIBUTING.md: "if the library/program is testable, then coverage should be >= 80% for non-data-related packages and >=90% for data-related packages."
→ awesome-go requires coverage of 80% or more (90% for data-related packages) when testable. A DB-clone tool may count as data-related.

According to https://github.com/avelino/awesome-go/blob/main/CONTRIBUTING.md: "One PR adds, removes, or changes **only one item**." PR body must include "Forge link and pkg.go.dev are provided."
→ PR rules: one item per PR, with both the repo link and the pkg.go.dev link. Format: "- [project-name](https://github.com/org/project) - Short, clear description." with the exact project name and a description ending in punctuation. A pkg.go.dev link that loads is checked automatically. Go Report Card is not listed in the fetched text.

According to https://github.com/ramnes/awesome-mongodb: "Feel free to improve this list by [contributing](/ramnes/awesome-mongodb/blob/master/CONTRIBUTING.md)!"
→ A suitable list exists (ramnes/awesome-mongodb) with Tools/GUI, Backup (mgob) and Migration (migrate-mongo, mongo-connector) sections. The CONTRIBUTING.md text itself was not read.

According to https://github.com/shlomi-noach/awesome-mysql: "This list accepts and encourages pull requests. See [CONTRIBUTING](https://github.com/shlomi-noach/awesome-mysql/blob/master/CONTRIBUTING.md)"
→ A suitable list exists (shlomi-noach/awesome-mysql) with a Backup section (MyDumper, Xtrabackup); no explicit clone/migration section was seen. The CONTRIBUTING.md text was not read.

According to https://goreleaser.com/customization/homebrew_casks/: "Optionally a token can be provided, if it differs from the token provided to GoReleaser"
→ goreleaser publishes to your own tap via `homebrew_casks` (repository.owner/name/branch/token, optional pull_request). The page lists the older "Homebrew (deprecated)" `brews` section separately. `homebrew_casks` is available since v2.10, per the fetch summary. Casks are macOS-only, which matters for a CLI (not verified further).

According to https://docs.brew.sh/Package-Acceptance-Policy (via WebSearch result snippet only, page NOT fetched; the cap was hit): "at least 30 forks, 30 watchers or 75 stars, or at least 90 forks, 90 watchers or 225 stars for a self-submission by the repository owner. A code repository less than 30 days old is normally not eligible."
→ homebrew-core notability thresholds are 30 forks/30 watchers/75 stars, tripled for self-submission; repos younger than 30 days are normally ineligible. Re-fetch to confirm the wording before relying on it.

According to https://goreleaser.com/customization/scoop/: "Optionally a token can be provided, if it differs from the token provided to GoReleaser"
→ Scoop publishing uses `scoops:` with repository owner/name/branch/token. Your own bucket repo is enough; no notability gate was found.

According to https://pkg.go.dev/about: "Making a request to proxy.golang.org for the module version, to any endpoint specified by the Module proxy protocol."
→ Indexing is triggered by requesting the module version from proxy.golang.org (e.g. `.../@v/v1.0.0.info`). A redistributable license affects what is displayed (see https://pkg.go.dev/license-policy, not fetched).

According to https://goreleaser.com/customization/docker/: "Note that you will have to manually log in to the Docker registries you want to push to — GoReleaser does not log in by itself."
→ Docker Hub and ghcr publishing works through `image_templates` (docker.io/..., and by extension ghcr.io; the page example shows docker.io and gcr.io). You must `docker login` first (CI step). `dockers` and `docker_manifests` are marked deprecated as of v2.12 in favour of `dockers_v2`.

According to https://console.dev/selection-criteria: "Email [hello@console.dev](mailto:hello@console.dev) with the details and we'll happily take a look."
→ Console.dev submission is by email. Criteria include a developer primary user, active maintenance, good docs, speed, and no security/privacy harm. One criterion is "Is there a self-service signup?"

## Not found

- not found: Golang Weekly submission process — tried https://golangweekly.com/ (the page had no submit/contact text). Not tried: a dedicated contact page.
- not found: Changelog News submission rules — tried https://changelog.com/submit (404). A search snippet said submissions go via changelog.com/submit or changelog.com/news/submit, but I did not fetch that page. Go Time was not researched.
- not found: TLDR newsletter submission process — tried a search only; nothing editorial found.
- not found: awesome-mongodb and awesome-mysql CONTRIBUTING.md contents — only the README was fetched.
- not found: awesome-go Go Report Card and star requirements — absent from the fetched CONTRIBUTING.md (may exist in other sections; the fetch used an LLM summary).
- not found: homebrew-core thresholds on the Package-Acceptance-Policy page itself — tried https://docs.brew.sh/Acceptable-Formulae twice (it has no such numbers). The Package-Acceptance-Policy page was not fetched.
- not found: awesome-mongodb-specific requirement for non-Go tools, plus Go-specific Homebrew formula (brews) vs cask guidance for Linux — not researched.

Note: all WebFetch results are summaries from a small model, not raw page text; quotes should be re-verified.

## Calls used: 15/15
