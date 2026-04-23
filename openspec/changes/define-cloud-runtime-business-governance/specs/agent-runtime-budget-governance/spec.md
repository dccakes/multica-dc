## ADDED Requirements

### Requirement: Hierarchical Budget Enforcement
The system SHALL enforce agent-execution budgets at both parent-issue and monthly-company levels for billable runtimes.

#### Scenario: Parent budget reaches warning thresholds
- **WHEN** cumulative billable spend for a parent issue reaches 80% or 90% of its configured cap
- **THEN** the system SHALL emit threshold warnings without blocking ongoing execution

#### Scenario: Parent budget reaches 100%
- **WHEN** cumulative billable spend for a parent issue reaches 100% of its configured cap
- **THEN** the system SHALL block new billable subagent starts under that parent issue
- **AND** the system SHALL pause in-flight billable subagent runs at the next safe checkpoint
- **AND** the system SHALL post an out-of-budget comment in the issue thread
- **AND** the system SHALL transition affected runs to `needs_human_intervention`

#### Scenario: Monthly company budget reaches 100%
- **WHEN** cumulative billable spend for the company month reaches 100% of its configured cap
- **THEN** the system SHALL block new billable issue starts
- **AND** the system SHALL allow resume or handoff workflows for already-started issues based on permission policy
- **AND** the system SHALL pause in-flight billable runs at the next safe checkpoint

### Requirement: Runtime-Type-Aware Budget Policy
The system MUST auto-detect runtime type and apply budget enforcement only to billable runtime types.

#### Scenario: Local runtime under budget block
- **WHEN** a run executes on a `local` runtime during a parent or monthly 100% budget block
- **THEN** the system SHALL classify that run as non-billable
- **AND** the system SHALL allow execution to continue

#### Scenario: Vercel runtime under budget block
- **WHEN** a run executes on a `vercel` runtime during a parent or monthly 100% budget block
- **THEN** the system SHALL enforce blocking and pause policies for billable runs

### Requirement: Permission-Gated Budget Overrides
The system SHALL restrict budget and override control actions by role and scope.

#### Scenario: Admin applies per-issue override
- **WHEN** a workspace admin applies an override to a blocked issue
- **THEN** the system SHALL allow billable agent execution for that specific issue
- **AND** the system SHALL keep other blocked issues constrained by budget policy

#### Scenario: Override lifecycle ends on issue completion
- **WHEN** an issue with an active budget override is completed
- **THEN** the system SHALL automatically expire that override

#### Scenario: Non-admin attempts override
- **WHEN** a non-admin user attempts to change budgets, limits, or overrides
- **THEN** the system SHALL deny the action according to permission policy

### Requirement: Admin-Configurable Concurrency Limits
The system SHALL enforce remote execution concurrency at workspace and parent-issue levels.

#### Scenario: Workspace global remote cap enforced
- **WHEN** active remote billable runs equal the configured workspace global limit
- **THEN** the system SHALL queue additional remote starts until capacity is available

#### Scenario: Parent issue cap enforced
- **WHEN** active runs for a parent issue equal the configured per-parent limit
- **THEN** the system SHALL block additional starts for that parent issue until one run finishes or pauses

### Requirement: Configured Vercel Runtime Visibility
The system SHALL show configured Vercel sandbox runtimes in the runtimes list as remote runtime entries.

#### Scenario: Vercel runtime configured and bootstrapped
- **WHEN** a workspace admin configures Vercel credentials and bootstrap creates or links a runtime
- **THEN** the runtimes list SHALL include that runtime with `remote` mode identity
- **AND** the entry SHALL remain visible even when runtime status is offline
