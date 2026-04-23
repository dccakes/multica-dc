## ADDED Requirements

### Requirement: Portable Execution Checkpoint State
The system SHALL persist execution checkpoint state sufficient to resume independently of provider-specific history retention.

#### Scenario: Required checkpoint fields persisted
- **WHEN** a checkpoint is written for an executing run
- **THEN** the checkpoint MUST include run identity, execution state, sandbox and snapshot references, workspace and git state, agent session context, artifacts/log references, and cumulative cost state

#### Scenario: Checkpoint supports multi-path recovery
- **WHEN** a run requires recovery after interruption
- **THEN** persisted checkpoint data SHALL be sufficient to resume from sandbox, resume from snapshot, or hand off to local execution

### Requirement: Checkpoint Cadence and Trigger Policy
The system SHALL checkpoint on transition and heartbeat with additional high-priority event triggers.

#### Scenario: Transition and heartbeat checkpointing
- **WHEN** a run is in progress
- **THEN** the system SHALL write a checkpoint on every execution state transition
- **AND** the system SHALL write a heartbeat checkpoint at least every 5 minutes

#### Scenario: High-priority checkpoint triggers
- **WHEN** any of the following events occur: PR opened, PR updated, intervention transition, budget threshold crossing, or force-close pre-step
- **THEN** the system SHALL write an immediate checkpoint

### Requirement: Safe Checkpoint Pause Semantics
The system SHALL pause budget-blocked runs only at defined safe checkpoints.

#### Scenario: Safe checkpoint definition
- **WHEN** the system evaluates whether a run can be paused for budget policy
- **THEN** it SHALL treat these boundaries as safe checkpoints: command step completion, git operation completion, PR synchronization step completion, and successful checkpoint persistence

#### Scenario: Budget pause execution
- **WHEN** a budget block is active and an in-flight billable run reaches a safe checkpoint
- **THEN** the system SHALL pause execution at that point and transition to `needs_human_intervention`

### Requirement: Resume Eligibility and Fallback Order
The system SHALL resolve resume paths with health and budget validation before selecting fallback behavior.

#### Scenario: Resume from sandbox eligibility
- **WHEN** a user requests resume from sandbox
- **THEN** the system SHALL permit sandbox resume only if sandbox health check passes and budget policy permits billable execution

#### Scenario: Resume fallback order
- **WHEN** sandbox resume is ineligible or fails
- **THEN** the system SHALL attempt resume from latest valid snapshot
- **AND** when snapshot resume is unavailable, the system SHALL permit local handoff as recovery path
