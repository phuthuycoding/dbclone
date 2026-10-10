---
feature: "postgres-driver"
context: "engines"
created: "20261010_1959"
status: planning
---

# Use Case Diagram

```mermaid
flowchart LR
  U[CLI user] --> UC1[UC-001 Clone whole database remote → local]
  U --> UC2[UC-002 Clone selected objects -only]
  U --> UC3[UC-003 Fresh clone onto local]
  U --> UC4[UC-004 Clone between remote endpoints]
  U --> UC5[UC-005 Preflight fix guidance]
  U --> UC6[UC-006 Retry and fast-fail]
  LC[(local postgres container)] --> UC1
  LC --> UC3
  RS[(remote PostgreSQL server)] --> UC1
  RS --> UC4
  UC3 -.extends.-> UC1
  UC2 -.subset of.-> UC1
  UC4 -.reuses.-> UC1
```

- Actors: CLI user; local `postgres` container (tools host); remote PostgreSQL server (profile endpoint)
- Use cases: whole-db clone, subset clone, fresh clone, remote↔remote clone, preflight guidance, retry/fast-fail
- Relationships: UC-003 and UC-002 specialize UC-001 (fresh drop / object subset); UC-004 reuses the UC-001 stream with remote connections on both sides; UC-005 and UC-006 are cross-cutting behaviors of every clone
