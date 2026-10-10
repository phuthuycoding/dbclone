---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
execution: "{execution_id}"
status: REJECT
---

# Review Report

## Feature
{feature_name}

## Review Status
- {review_status}

## Review Overview

| Field | Result |
|---|---|
| Reviewed scope / diff | {reviewed_scope} |
| Execution | {execution_id} |
| Open blocking findings | {blocking_findings} |
| Overall decision | {review_status} |

## Requirement Compliance
{requirement_compliance}

## Scope Compliance
{scope_compliance}

## Code Convention
{code_convention}

## Architecture
{architecture}

## Logic
{logic}

## Tests
{tests}

## Test Coverage
{test_coverage}

## Linter / Formatter / Prettier
{linter}

## Type Check / Static Analysis
{type_check}

## Findings

| ID | Severity | Area / location | Finding | Evidence | Required action | Status |
|---|---|---|---|---|---|---|
| FINDING-001 | {severity} | {area} | {description} | {evidence} | {required_action} | {finding_status} |

If no findings remain, replace the sample row with `None` and record the review evidence. PASS requires all blocking findings to be resolved.

## Documentation Impact

| Update needed? | Related feature / docs | Reason / update |
|---|---|---|
| {docs_update_needed} | {related_docs} | {docs_update_reason} |

## Final Decision
{final_decision}
