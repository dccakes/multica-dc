## ADDED Requirements

### Requirement: Human Ownership Invariant with Agent Delegation
The system SHALL preserve a human as canonical issue owner at all times, including while an agent is delegated for execution.

#### Scenario: Issue delegated to agent
- **WHEN** a developer delegates an issue to an agent
- **THEN** the system SHALL keep the human assignee as canonical owner
- **AND** the system SHALL record the agent as delegated executor rather than owner

#### Scenario: Ownership reassigned during active execution
- **WHEN** issue ownership is reassigned from one human to another while an agent run is active
- **THEN** the system SHALL continue execution automatically under the new owner
- **AND** the system SHALL preserve delegation state

### Requirement: Execution Intervention Outcomes
The system MUST support recoverable intervention outcomes for runs that cannot proceed automatically.

#### Scenario: Run enters intervention state
- **WHEN** a run encounters a blocking condition but has salvageable work state
- **THEN** the system SHALL transition the run to `needs_human_intervention`
- **AND** the system SHALL expose allowed outcomes: `resume_from_sandbox`, `resume_from_snapshot`, `handoff_to_local`, `archive`, `force_close`

#### Scenario: External integration failure after partial work
- **WHEN** a run completes substantive work but fails at a downstream integration step such as GitHub connectivity
- **THEN** the system SHALL preserve work state for intervention outcomes
- **AND** the system SHALL NOT discard work solely due to that downstream failure

### Requirement: Completion Authority and Definition
The system SHALL enforce completion semantics by work type and assigned issue owner authority.

#### Scenario: Code work completion
- **WHEN** a delegated agent marks code work as complete
- **THEN** the system SHALL allow only `ready_for_review` until merge occurs
- **AND** the system SHALL require assigned issue owner action to mark final completion

#### Scenario: Non-code work completion
- **WHEN** a delegated agent produces a plan, design, or research output
- **THEN** the system SHALL require assigned issue owner approval and actioning before marking done

### Requirement: Estimation Baseline for Productivity Metrics
The system SHALL support engineering-hours-saved analysis with human-first estimates and agent fallback classification.

#### Scenario: Human estimate provided
- **WHEN** an issue has a developer-provided effort estimate
- **THEN** the system SHALL use that estimate as baseline for hours-saved metrics

#### Scenario: Human estimate missing
- **WHEN** an issue starts without a developer-provided effort estimate
- **THEN** the system SHALL allow execution to start
- **AND** the system SHALL allow agent auto-classification to assign an estimate baseline
- **AND** the system SHALL allow humans to override any classification or estimate outcome
