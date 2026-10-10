---
feature: "{feature_name}"
context: "{context}"
tested: "{timestamp}"
execution: "{execution_id}"
status: BLOCKED
---

# Testing Result

## Feature
{feature_name}

## Environment
- OS: {os}
- Runtime: {runtime}
- Tooling: {tooling}

## Execution Time
{execution_time}

## Summary
| Metric | Result |
|---|---:|
| Total | {total} |
| Passed | {passed} |
| Failed | {failed} |
| Rejected | {rejected} |
| Blocked | {blocked} |

## Test Results

| Case / test name | Type | Status | Expected | Actual | Evidence |
|---|---|---|---|---|---|
| TC-001 | {test_type} | {tc_status} | {expected_outcome} | {actual_outcome} | {evidence} |

Use the approved TC IDs for features; use reproduction/regression test names for bugs. Include every required test, including blocked or unexecuted tests.

## Commands and Evidence

| Command / tool | Exit code | Evidence / output |
|---|---:|---|
| {tool} | {exit_code} | {command_evidence} |

## Failures and Blockers

| Case / test name | Error / blocker | Impact | Next action |
|---|---|---|---|
| {affected_test} | {error} | {failure_impact} | {next_action} |

## Coverage

| Metric / scope | Target | Measured | Evidence |
|---|---:|---:|---|
| Overall code coverage | {coverage_target}% | {overall_coverage}% | {coverage_evidence} |

Do not equate test pass rate with code coverage. Use `N/A` with a reason for an unrequired metric; required but unmeasured coverage blocks PASS.

## Regression
{regression}

## Conclusion
- {conclusion}
