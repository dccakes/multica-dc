## Why

Multica needs a clear business and operational contract for internal agent execution before additional cloud runtime implementation continues. The team needs higher development throughput without uncontrolled spend, while preserving human ownership and recoverability when agent runs need intervention.

## What Changes

- Define a governance model for agent execution with strict budget controls, permission-based overrides, and runtime-type-aware enforcement.
- Define ownership and delegation semantics where humans remain canonical issue owners and agents act as delegated executors.
- Define resumable execution behavior with explicit checkpointing, intervention states, and resume pathways (sandbox, snapshot, local handoff).
- Define completion semantics for code and non-code work, including human approval authority and outcome states.
- Define internal success metrics for early rollout, including reliability, productivity, and quality indicators.
- Define phased operating constraints for initial rollout (single team internal usage, configurable remote concurrency, admin-managed shared capacity).
- Define runtime list UX behavior so configured Vercel sandboxes appear as remote runtimes in the runtimes list.

## Capabilities

### New Capabilities
- `agent-runtime-budget-governance`: Budget hierarchy, threshold enforcement, runtime-aware blocking, and admin override rules for internal agent execution.
- `delegated-agent-ownership-lifecycle`: Human ownership, agent delegation, intervention and completion state machine, and approval authority rules.
- `resumable-agent-execution-continuity`: Required persisted execution state, checkpoint policy, and resume decision logic across sandbox/snapshot/local paths.

### Modified Capabilities
- None.

## Impact

- Affects cloud runtime control-plane behavior in backend APIs and cloudrunner lifecycle enforcement.
- Affects assignment/delegation, issue status transitions, and completion workflows in product UX.
- Affects cost attribution and budget controls across runtime execution paths (local vs Vercel).
- Establishes requirements for future implementation in runtime management, policy enforcement, and telemetry/reporting surfaces.
