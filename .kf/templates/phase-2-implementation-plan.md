---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
status: planning
---

# Implementation Plan

## Feature
{feature_name}

## Objective
{objective}

## Implementation Scope
### Implement Now
- {implement_now_item}

### Supporting Features
- {supporting_feature}

### Future Features
- {future_feature}

### Explicitly Not Implemented
- {not_implemented_item}

## Task Breakdown

| Task | Description | Area | Dependencies | Expected output | FR / UC references |
|---|---|---|---|---|---|
| TASK-001 | {task_description} | {area} | {dependencies} | {expected_output} | {requirement_reference} |

Add one row per task. Track execution progress separately in `tasks.md` so the approved plan remains unchanged.

## Complexity
- Level: {complexity_level}
- Reason: {complexity_reason}

## Impact Analysis

| Area | Impact |
|---|---|
| Backend | {backend_impact} |
| Frontend | {frontend_impact} |
| Database | {database_impact} |
| API | {api_impact} |
| Infrastructure | {infrastructure_impact} |
| Security | {security_impact} |
| Performance | {performance_impact} |
| Regression | {regression_impact} |
| Dependencies | {dependencies_impact} |

## Risks and Mitigation

| Risk | Impact | Mitigation / rollback |
|---|---|---|
| {risk} | {risk_impact} | {risk_mitigation} |

## Architecture Constraints
- {architecture_constraint}

## Implementation Constraints
- {implementation_constraint}

## Testing Strategy
- Unit: {unit_strategy}
- Integration: {integration_strategy}
- E2E: {e2e_strategy}
- Coverage target: >= {coverage_target}%

## Documentation Impact
- {documentation_impact}

## Definition of Done
- [ ] Implementation complete
- [ ] Unit tests
- [ ] Integration tests where applicable
- [ ] Coverage meets the agreed target
- [ ] E2E tests when required by the agreed Test Level
- [ ] All test cases pass
- [ ] Review passes
- [ ] Documentation updated
- [ ] Feature report generated
