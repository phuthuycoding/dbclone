---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
status: DONE
---

# Feature Report

## Feature
{feature_name}

## Completion Overview

| Field | Result |
|---|---|
| Context | {context} |
| Delivery summary | {delivery_summary} |
| Reviewed execution | {execution_id} |
| Testing / review status | PASS / PASS |

## Requirement Summary
{requirement_summary}

## Scope
{scope}

## Implementation Summary
{implementation_summary}

## Changed Areas

| Area / files | Change | Requirement / use case |
|---|---|---|
| {changed_area} | {change_summary} | {requirement_reference} |

## Use Cases

| Use case | Delivered behavior | Evidence |
|---|---|---|
| UC-001 {use_case_name} | {delivered_behavior} | {use_case_evidence} |

## Test Summary
| Metric | Result |
|---|---:|
| Test cases | {total_cases} |
| Passed | {passed} |
| Failed | {failed} |
| Coverage | {overall_coverage}% |

## Review Summary
{review_summary}

## Documentation Updated

| Document | Update / purpose | Delivery |
|---|---|---|
| `docs/requirement/{context}/{feature_name}.md` | Confirmed requirement | CLI copies on archive |
| `docs/use-cases/{context}/{feature_name}/README.md` | UC index and coverage | CLI copies on archive |
| `docs/use-cases/{context}/{feature_name}/UC-###.md` | Individual UC narratives | CLI copies on archive |
| `docs/use-cases/{context}/{feature_name}/diagram.md` | Actor/use-case diagram | CLI copies on archive |
| `docs/testplan/{context}/{feature_name}.md` | Test contract | CLI copies on archive |
| `docs/testplan/{context}/{feature_name}-result.md` | Current execution evidence | CLI copies on archive |
| {other_document} | {documentation_updated} | {documentation_status} |

Replace the UC wildcard row with the actual files. Record `N/A` with a reason if no other documents need updates.

## Known Limitations
- {known_limitation}

## Accepted Follow-ups

| Item | Reason deferred | Backlog reference |
|---|---|---|
| {follow_up} | {deferred_reason} | {backlog_reference} |

## Final Status
DONE
