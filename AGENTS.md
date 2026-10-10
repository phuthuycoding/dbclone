# AGENTS.md

Conventions for AI coding agents working in this repository. Seeded by `kf init`; edit freely.

## Commands

| Task | Command |
|---|---|
| Build | `go build ./...` |
| Test | `go test ./...` |
| Lint | `gofmt -l .` (must print nothing) and `go vet ./...` |

Detect anything else by reading the repository, never by guessing.

## Workflow (kanban-flow)

- Start a feature with one command: `kanban <context> <feature>` (bug: `kanban <context> <name> --type bug`).
- Only two human gates: confirm the requirement (Phase 1) and approve the plan + choose start-now vs backlog (Phase 2). Do not ask "continue?" between other phases.
- Work item state lives in `.works/`; move items only with `kf stage` / `kf archive`, never by moving folders.
- Never use `--force` or `--skip-hooks` without explicit user approval; every bypass is recorded in the work item's metadata.
- Run `kf autoconfig` for the setup checklist and the review rules to apply as coding conventions.
