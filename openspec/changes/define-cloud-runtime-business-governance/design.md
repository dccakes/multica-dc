## Context

Multica is introducing internal cloud agent execution on Vercel sandboxes to increase engineering throughput while keeping spend controlled. Existing implementation work already provides cloud credential handling, cloudrunner loops, and snapshot continuity scaffolding, but product and policy semantics are not yet formalized. The immediate rollout context is internal-only usage, starting with one developer and moving to shared remote capacity across multiple developers.

The design must define a business-first control model for budget, ownership, intervention, completion, and resumability. The system must preserve human accountability, allow human recovery of partially successful runs, and block uncontrolled remote spend while still allowing local non-billable execution.

## Goals / Non-Goals

**Goals:**
- Define enforceable budget governance with parent-issue and monthly company controls.
- Define permission-gated overrides and admin-managed concurrency limits.
- Define delegation semantics where humans remain canonical owners and agents are executors.
- Define recoverable execution with checkpointing and resume paths (sandbox, snapshot, local handoff).
- Define completion and approval semantics for both code and non-code work.

**Non-Goals:**
- Multi-provider runtime orchestration beyond local and Vercel runtime-type policy.
- Reminder-system implementation if not already present in the platform.
- Full audit-log framework for all human overrides in v1.
- Multi-region orchestration in v1 rollout.

## Decisions

### 1) Budget governance is hierarchical and tiered
The system uses:
- Parent-issue budget cap (sub-issues consume against parent only).
- Monthly company budget cap.
- Thresholds at 80% warning, 90% critical warning, and 100% block.

At 100%, billable subagent execution is paused at the next safe checkpoint and status moves to `needs_human_intervention` with an out-of-budget issue-thread comment.

Alternatives considered:
- Flat per-task budget only: rejected because parent/sub-issue work needs aggregate control.
- Hard-kill at cap: rejected because it risks losing partial useful work.

### 2) Runtime type determines billability and block behavior
Runtime classification is automatic:
- `vercel` runtime: billable, budget-governed, blocked at 100%.
- `local` runtime: non-billable, allowed to continue under budget blocks.

This exception applies to both parent-issue and monthly-company budget blocks.

Alternatives considered:
- Manual billable flag per run: rejected due to operational overhead and policy drift risk.
- Block all runtime types equally: rejected because internal subscription/local execution should remain available for continuity.

### 3) Human ownership remains canonical; agent acts as delegate
A delegated issue MUST always retain a human owner. Agent delegation does not transfer ownership authority. Ownership may be reassigned between humans while agent execution continues automatically.

Alternatives considered:
- Agent as owner: rejected because completion/accountability must remain with humans.
- Require pause before reassignment: rejected to avoid unnecessary operational friction.

### 4) Completion authority is explicit and strict
- Code work is done only when merged and marked complete by the assigned issue owner.
- Non-code outputs are done only after assigned issue owner approval and actioning.
- Agent can reach `ready_for_review` but cannot unilaterally complete.

Alternatives considered:
- Any reviewer can complete: rejected to preserve single-accountability model.
- Agent auto-complete on CI green: rejected to avoid quality and intent mismatch.

### 5) Recoverability uses portable checkpoints, not provider assumptions
Checkpointing persists enough state to resume independently of provider-specific history retention. Required state includes runtime/session refs, workspace/git state, execution state, artifacts, and cumulative cost state.

Checkpoint writes happen on every state transition and every 5-minute heartbeat while running, plus priority events (PR open/update, intervention, threshold crossing, force close pre-step).

Alternatives considered:
- Event-only checkpointing: rejected due to higher loss risk on long runs.
- Provider-history-only recovery: rejected due to portability and reliability risk.

### 6) Capacity control is admin-configurable and conservative by default
- Global remote concurrency limit exists at workspace level (initial default 2).
- Optional per-parent-issue concurrency limit exists (default 1).
- Admins can adjust limits; developers cannot.

Alternatives considered:
- Unbounded queue-driven concurrency: rejected for budget risk.
- Parent-only limit without global limit: rejected because shared remote capacity must be centrally governed.

### 7) Runtime discoverability is explicit in the runtimes UI
Once Vercel credentials are configured and runtime bootstrap completes, the runtime must appear in the runtimes list as a remote runtime entry. This ensures admins and developers can see execution capacity state and avoids hidden configured capacity.

Alternatives considered:
- Keep runtime visible only in cloud-credential settings: rejected because operators need a single runtime inventory view.
- Show only active/online runtimes: rejected because offline configured runtimes still need discoverability and troubleshooting.

## Risks / Trade-offs

- [Strict budget blocks can interrupt momentum] -> Mitigation: safe-checkpoint pause, resume/snapshot/local handoff options, and per-issue admin override.
- [No broad override audit in v1 reduces forensics] -> Mitigation: keep permission-gated controls and defer deep audit trails to later phase.
- [Auto task-type classification may mis-estimate hours saved] -> Mitigation: humans can override any agent classification or estimate.
- [Local-runtime exception could shift load off governed remote capacity] -> Mitigation: track non-billable local usage for productivity analytics.

## Migration Plan

1. Land spec and policy model as source of truth for runtime governance.
2. Align backend/cloudrunner policy checks to new requirement set.
3. Expose admin controls for budgets and concurrency limits where missing.
4. Add status/comment behavior for budget-block transitions.
5. Validate rollout with internal team phase gates (single-user local -> shared remote).

Rollback approach:
- If policy enforcement causes unacceptable workflow disruption, disable new enforcement toggles and fall back to existing runtime behavior while preserving checkpoint persistence.

## Open Questions

- Whether to introduce sub-issue soft caps in a later phase.
- Whether to add a lightweight override activity feed after v1.
- Whether monthly budget policies should support team-level partitioning in addition to company-level cap.
