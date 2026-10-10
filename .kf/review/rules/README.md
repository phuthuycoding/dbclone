# Kanban Flow Review Rules

## Structure
- `general.md` — Rules applied to every review
- `security.md` — Security-specific checks
- `performance.md` — Performance-specific checks
- `{stack}.md` — Stack-specific rules. Packs ship in the package at `kanban-flow/review/stacks/{stack}.md` (7 packs: `node`, `go`, `rust`, `python`, `php`, `ruby`, `java`) and are installed into this directory via `kf rules`.

## How to Use
Skill `kanban-review` loads all rules in this directory at review time.
Project-specific rules override global rules when same filename exists.

## Installing Stack Rules
```bash
kf rules                                  # auto-detect stacks from manifests (monorepo: installs every detected pack)
kf rules --stack go --stack python        # install specific packs
kf rules --list                           # list available packs and detected stacks
kf rules --force                          # overwrite a project rule file that differs from the pack
```
Without `--force`, an existing `.kf/review/rules/{stack}.md` that differs from the pack is skipped with a warning — project rules are user-owned. Re-running on an identical file reports "already installed". Works before `kf init`.

## Adding Rules
Create a new `.md` file in this directory. Each rule should be:
- A short imperative sentence
- Specific enough to verify (not "write good code")

## Severity Levels
- **HIGH** — Must fix before merge (security risk, data loss, crash)
- **MEDIUM** — Should fix before merge (performance, maintainability)
- **LOW** — Suggestion, can defer
