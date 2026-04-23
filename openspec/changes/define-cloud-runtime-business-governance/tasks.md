## 1. Policy Model and Data Contracts

- [ ] 1.1 Define shared policy enums/constants for runtime type, budget states, intervention outcomes, and completion states
- [x] 1.2 Define checkpoint payload contract with required persisted fields for portable resume
- [ ] 1.3 Define budget threshold and enforcement contract (80/90 warnings, 100 block with safe-checkpoint pause)
- [ ] 1.4 Define role/permission contract for admin-only budget controls and issue-owner completion authority

## 2. Budget Enforcement and Concurrency Controls

- [ ] 2.1 Implement parent-issue budget accounting where sub-issues consume only against parent cap
- [ ] 2.2 Implement monthly company budget accounting and block behavior for billable runtimes
- [ ] 2.3 Implement runtime-type-aware enforcement (`vercel` billable, `local` non-billable exception)
- [ ] 2.4 Implement admin-configurable global remote concurrency limit and optional per-parent limit
- [ ] 2.5 Implement per-issue admin override lifecycle with auto-expiry on issue completion

## 3. Delegation, Ownership, and Lifecycle Transitions

- [ ] 3.1 Enforce human canonical ownership invariant during agent delegation
- [ ] 3.2 Ensure owner reassignment during active runs continues execution without forced pause
- [ ] 3.3 Implement `needs_human_intervention` transitions for recoverable failures
- [ ] 3.4 Implement intervention actions: resume from sandbox, resume from snapshot, handoff to local, archive, force close
- [ ] 3.5 Enforce completion semantics by work type (code merged + owner completion, non-code owner approval/actioning)

## 4. Checkpointing and Resume Orchestration

- [x] 4.1 Implement checkpoint writes on every state transition and 5-minute in-progress heartbeat
- [ ] 4.2 Implement immediate checkpoint triggers (PR opened/updated, intervention transition, threshold crossing, pre-force-close)
- [ ] 4.3 Implement safe-checkpoint pause boundaries (command step, git step, PR sync step, persisted checkpoint)
- [ ] 4.4 Implement resume eligibility checks (sandbox health + budget) and fallback order (sandbox -> snapshot -> local handoff)

## 5. UX and Communication Surfaces

- [ ] 5.1 Add budget-block issue-thread comment behavior for out-of-budget transitions
- [ ] 5.2 Expose admin controls for budget caps and concurrency limits in runtime management surfaces
- [ ] 5.3 Expose blocked/paused/intervention statuses clearly in issue and runtime views
- [ ] 5.4 Show permission-denied feedback for non-admin budget control attempts
- [ ] 5.5 Ensure configured and bootstrapped Vercel sandboxes appear in runtimes list as remote runtimes, including offline visibility

## 6. Metrics and Validation

- [x] 6.1 Track billable spend (sandbox + model/token API) and non-billable local usage for productivity analytics
- [x] 6.2 Support developer-provided effort estimate with agent auto-estimate fallback when missing
- [ ] 6.3 Validate reliability success criteria across end-to-end chain (provision, setup, repo correctness, PR workflow, notification)
- [ ] 6.4 Add tests for budget enforcement, permission gates, lifecycle transitions, checkpoint cadence, and resume fallback behavior
