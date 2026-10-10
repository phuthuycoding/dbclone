---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
status: planning
---

# Use Case Index

Every use case is its own file under `use-cases/`, named `UC-###-<slug>.md` (e.g. `UC-001-create-task.md`). Do not write a combined narrative here.

## Use Case Files

| ID | Name | File | Primary Actor | Status |
|---|---|---|---|---|
| UC-001 | {use_case_name_001} | [UC-001](use-cases/UC-001-{use_case_slug_001}.md) | {primary_actor_001} | planned |
| UC-002 | {use_case_name_002} | [UC-002](use-cases/UC-002-{use_case_slug_002}.md) | {primary_actor_002} | planned |

## Use Case Coverage

| UC ID | FR references | TC references | Acceptance coverage |
|---|---|---|---|
| UC-001 | FR-001 | TC-001 | {uc_001_coverage} |
| UC-002 | FR-002 | TC-002 | {uc_002_coverage} |

## Totals

| Metric | Total |
|---|---:|
| Use cases | {total_use_cases} |
| Actors | {total_actors} |
| Functional requirements covered | {total_requirements} |
| Test cases linked | {total_linked_test_cases} |
