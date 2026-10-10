---
feature: "{feature_name}"
context: "{context}"
created: "{timestamp}"
status: planning
---

# Use Case

## Overview

| Field | Value |
|---|---|
| ID | {use_case_id} |
| Name | {use_case_name} |
| Requirement reference | {requirement_reference} |
| Goal | {goal} |
| Primary actor | {primary_actor} |

## Supporting Actors
- {supporting_actor}

## Preconditions
- {precondition}

## Trigger
{trigger}

## Main Flow

| Step | Actor / system | Action | Outcome |
|---|---|---|---|
| 1 | {step_actor} | {main_flow_step} | {step_outcome} |

## Alternative Flows
### A1
- Trigger: {alternative_trigger}

| Step | Action | Outcome / return to main flow |
|---|---|---|
| A1.1 | {alternative_step} | {alternative_outcome} |

## Exception Flows
### E1

| Trigger | Handling | Resulting state / message |
|---|---|---|
| {exception_trigger} | {exception_handling} | {exception_outcome} |

## Postconditions
- {postcondition}

## Business Rules
- {business_rule}

## Data
- {data_entity}: {data_description}

## Acceptance Criteria
- [ ] {acceptance_criterion}
