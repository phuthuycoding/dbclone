---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
status: planning
---

# Test Plan

Test Strategy from `phase-1-spec-requirement.md` decides the depth:
`unit` → Unit; `unit+integration` → Unit + Integration; `full` → Unit + Integration + UI/E2E.

Extend the tables and repeat `## TC-###` for every planned test. Remove unused sample rows. Counts must match the detailed cases; use `0` with a reason for excluded test types. This is the planned contract: execution outcomes belong in the testing-result report.

## Feature Test Summary

| Field | Value |
|---|---|
| Feature | {feature_name} |
| Context | {context} |
| Test level | {test_level} |
| UI scope | {ui_test_scope} |
| Tools / commands | {test_tools} |
| Coverage target | {coverage_target}% |

## Overall Case Counts

| Test type | Planned | Must pass | Notes |
|---|---:|---:|---|
| Unit | {unit_total} | {unit_required} | {unit_scope} |
| Integration | {integration_total} | {integration_required} | {integration_scope} |
| UI / E2E | {e2e_total} | {e2e_required} | {e2e_scope} |
| **Total** | **{total_cases}** | **{total_required}** | **{overall_scope}** |

## Use Case Coverage Matrix

| Use case | Requirement(s) | Test cases | Planned | Pass criteria |
|---|---|---|---:|---|
| UC-001 | FR-001 | TC-001 | {uc_001_case_count} | {uc_001_pass_criteria} |
| UC-002 | FR-002 | TC-002 | {uc_002_case_count} | {uc_002_pass_criteria} |

## Requirement Coverage Matrix

| Requirement | Use case(s) | Test case(s) | Covered? | Gap / note |
|---|---|---|---|---|
| FR-001 | UC-001 | TC-001 | {fr_001_covered} | {fr_001_note} |
| FR-002 | UC-002 | TC-002 | {fr_002_covered} | {fr_002_note} |

## TC-001

| Field | Detail |
|---|---|
| Test case ID | TC-001 |
| Requirement reference | FR-001 |
| Use case reference | UC-001 |
| Test type | Unit / Integration / UI / E2E |
| Priority | High / Medium / Low |
| Preconditions | {precondition} |
| Input | {input} |
| Steps | See steps table below |
| Expected outcome | {expected_outcome} |
| Status | PENDING |

### Steps

| Step | Action | Expected result |
|---|---|---|
| 1 | {step} | {step_expected_result} |
