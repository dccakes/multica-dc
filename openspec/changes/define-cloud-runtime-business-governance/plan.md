# Cloud Runtime Business Governance Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement budget-governed delegated cloud runtime execution with resumable lifecycle controls, permission gates, and runtime-list visibility for configured Vercel remote runtimes.

**Architecture:** Extend existing cloudrunner + daemon lifecycle by adding a policy layer that enforces budgets, concurrency, and permissioned overrides before/at safe checkpoints. Persist portable checkpoints and budget state in backend DB + APIs, then surface controls/states in shared runtimes UI and issue workflow states. Keep runtime-type split (`local` non-billable, `vercel` billable) as a first-class decision in server enforcement paths.

**Tech Stack:** Go (Chi handlers, services, sqlc, pgx), PostgreSQL migrations, TypeScript (React, TanStack Query), Vitest, Go test.

---

## References

- Change proposal: `openspec/changes/define-cloud-runtime-business-governance/proposal.md`
- Change design: `openspec/changes/define-cloud-runtime-business-governance/design.md`
- Specs:
  - `openspec/changes/define-cloud-runtime-business-governance/specs/agent-runtime-budget-governance/spec.md`
  - `openspec/changes/define-cloud-runtime-business-governance/specs/delegated-agent-ownership-lifecycle/spec.md`
  - `openspec/changes/define-cloud-runtime-business-governance/specs/resumable-agent-execution-continuity/spec.md`
- Source task list: `openspec/changes/define-cloud-runtime-business-governance/tasks.md`

## File Structure Map

- DB/migrations:
  - Create: `server/migrations/052_runtime_policy_budget_controls.up.sql`
  - Create: `server/migrations/052_runtime_policy_budget_controls.down.sql`
  - Modify: `server/pkg/db/queries/runtime.sql`
  - Regenerate: `server/pkg/db/generated/*.go` via `make sqlc`
- Backend policy + lifecycle:
  - Create: `server/internal/runtimepolicy/types.go`
  - Create: `server/internal/runtimepolicy/service.go`
  - Create: `server/internal/runtimepolicy/service_test.go`
  - Modify: `server/internal/handler/daemon.go`
  - Modify: `server/internal/handler/runtime.go`
  - Modify: `server/internal/handler/cloud_runtime_credential.go`
  - Modify: `server/internal/service/task.go`
  - Modify: `server/internal/cloudrunner/runner.go`
  - Modify: `server/internal/cloudrunner/provider/provider.go`
  - Modify: `server/internal/cloudrunner/session.go`
  - Modify tests:
    - `server/internal/handler/daemon_test.go`
    - `server/internal/handler/runtime_test.go`
    - `server/internal/handler/cloud_runtime_credential_test.go`
    - `server/internal/cloudrunner/runner_test.go`
    - `server/internal/cloudrunner/session_test.go`
- Shared frontend/core:
  - Modify: `packages/core/types/agent.ts`
  - Modify: `packages/core/api/client.ts`
  - Modify: `packages/core/runtimes/queries.ts`
  - Modify: `packages/core/runtimes/mutations.ts`
  - Modify: `packages/views/runtimes/components/cloud-credentials-panel.tsx`
  - Modify: `packages/views/runtimes/components/runtime-list.tsx`
  - Modify: `packages/views/runtimes/components/runtime-detail.tsx`
  - Add tests:
    - `packages/views/runtimes/components/runtime-list.test.tsx`
    - `packages/views/runtimes/components/cloud-credentials-panel.test.tsx`

## Chunk 1: Policy Model and Data Contracts

### Task 1.1: Shared policy enums/constants

**Files:**
- Create: `server/internal/runtimepolicy/types.go`
- Modify: `packages/core/types/agent.ts`
- Test: `server/internal/runtimepolicy/service_test.go`

- [ ] **Step 1: Write failing enum/constant tests (Go)**
```go
func TestRuntimeTypeClassification(t *testing.T) {
  if IsBillableRuntime("local") { t.Fatal("local must be non-billable") }
  if !IsBillableRuntime("vercel") { t.Fatal("vercel must be billable") }
}
```
- [ ] **Step 2: Run test to verify failure**
Run: `cd server && go test ./internal/runtimepolicy -run TestRuntimeTypeClassification -v`
Expected: FAIL (package/file or symbol missing)
- [ ] **Step 3: Implement minimal shared policy types**
```go
type RuntimeType string
const (RuntimeTypeLocal RuntimeType = "local"; RuntimeTypeVercel RuntimeType = "vercel")
```
- [ ] **Step 4: Mirror types in shared TS model**
```ts
export type BudgetBlockState = "ok" | "warn_80" | "warn_90" | "blocked_100";
```
- [ ] **Step 5: Re-run tests**
Run: `cd server && go test ./internal/runtimepolicy -run TestRuntimeTypeClassification -v`
Expected: PASS
- [ ] **Step 6: Commit**
```bash
git add server/internal/runtimepolicy/types.go server/internal/runtimepolicy/service_test.go packages/core/types/agent.ts
git commit -m "feat(policy): add runtime and budget policy enums"
```

### Task 1.2: Checkpoint payload contract

**Files:**
- Create: `server/internal/runtimepolicy/checkpoint.go`
- Modify: `server/internal/cloudrunner/session.go`
- Test: `server/internal/cloudrunner/session_test.go`

- [ ] **Step 1: Add failing test for required checkpoint fields**
```go
func TestCheckpointRequiresPortableFields(t *testing.T) { /* assert missing fields rejected */ }
```
- [ ] **Step 2: Run failing test**
Run: `cd server && go test ./internal/cloudrunner -run TestCheckpointRequiresPortableFields -v`
- [ ] **Step 3: Implement checkpoint contract struct + validator**
```go
type Checkpoint struct { RunID, RuntimeID, SessionID, SnapshotID, WorkDir, Branch string }
```
- [ ] **Step 4: Wire contract into `RecordSnapshot` path**
Run validator before persistence in `session.go`.
- [ ] **Step 5: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run 'TestCheckpointRequiresPortableFields|TestRecordSnapshot' -v`
- [ ] **Step 6: Commit**
```bash
git add server/internal/runtimepolicy/checkpoint.go server/internal/cloudrunner/session.go server/internal/cloudrunner/session_test.go
git commit -m "feat(checkpoint): add portable checkpoint contract and validation"
```

### Task 1.3: Budget threshold/enforcement contract

**Files:**
- Create: `server/internal/runtimepolicy/thresholds.go`
- Modify: `server/internal/runtimepolicy/service.go`
- Test: `server/internal/runtimepolicy/service_test.go`

- [ ] **Step 1: Add failing tests for 80/90/100 threshold transitions**
- [ ] **Step 2: Run failing tests**
Run: `cd server && go test ./internal/runtimepolicy -run TestThresholdTransitions -v`
- [ ] **Step 3: Implement threshold evaluator**
```go
func ThresholdState(spend, cap int64) BudgetBlockState { /* 80/90/100 logic */ }
```
- [ ] **Step 4: Add safe-checkpoint pause decision helper**
```go
func ShouldPauseAtCheckpoint(state BudgetBlockState, billable bool) bool { return state == "blocked_100" && billable }
```
- [ ] **Step 5: Re-run tests**
Run: `cd server && go test ./internal/runtimepolicy -run TestThresholdTransitions -v`
- [ ] **Step 6: Commit**
```bash
git add server/internal/runtimepolicy/thresholds.go server/internal/runtimepolicy/service.go server/internal/runtimepolicy/service_test.go
git commit -m "feat(policy): implement threshold and pause decision contract"
```

### Task 1.4: Role/permission contract

**Files:**
- Create: `server/internal/runtimepolicy/permissions.go`
- Modify: `server/internal/handler/cloud_runtime_credential.go`
- Test: `server/internal/handler/cloud_runtime_credential_test.go`

- [ ] **Step 1: Add failing tests for admin-only controls and issue-owner completion**
- [ ] **Step 2: Run failing tests**
Run: `cd server && go test ./internal/handler -run TestVercelCloudRuntimeCredentialRoleEnforcement -v`
- [ ] **Step 3: Implement permission helpers**
```go
func CanManageBudgets(role string) bool { return role == "owner" || role == "admin" }
```
- [ ] **Step 4: Apply helpers in credential/admin endpoints**
- [ ] **Step 5: Re-run handler tests**
Run: `cd server && go test ./internal/handler -run 'TestVercelCloudRuntimeCredentialRoleEnforcement|TestVercelCloudRuntimeCredentialCRUD' -v`
- [ ] **Step 6: Commit**
```bash
git add server/internal/runtimepolicy/permissions.go server/internal/handler/cloud_runtime_credential.go server/internal/handler/cloud_runtime_credential_test.go
git commit -m "feat(authz): enforce admin-only runtime policy controls"
```

## Chunk 2: Budget Enforcement and Concurrency Controls

### Task 2.1: Parent-issue budget accounting

**Files:**
- Create: `server/migrations/052_runtime_policy_budget_controls.up.sql`
- Create: `server/migrations/052_runtime_policy_budget_controls.down.sql`
- Modify: `server/pkg/db/queries/runtime.sql`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Write failing DB-level test for parent issue spend aggregation**
- [ ] **Step 2: Add migration tables/columns for parent budget and spend ledger**
```sql
CREATE TABLE issue_budget (... parent_issue_id uuid not null, cap_cents bigint not null ...);
```
- [ ] **Step 3: Add sqlc queries for upsert/read budget + spend rollups**
- [ ] **Step 4: Regenerate sqlc**
Run: `make sqlc`
Expected: generated files updated under `server/pkg/db/generated/`
- [ ] **Step 5: Re-run failing tests**
Run: `cd server && go test ./internal/handler -run TestBudgetParentAggregation -v`
- [ ] **Step 6: Commit**
```bash
git add server/migrations/052_runtime_policy_budget_controls.* server/pkg/db/queries/runtime.sql server/pkg/db/generated
git commit -m "feat(db): add parent-issue budget accounting schema and queries"
```

### Task 2.2: Monthly company budget accounting + block behavior

**Files:**
- Modify: `server/internal/runtimepolicy/service.go`
- Modify: `server/internal/handler/daemon.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing test for monthly block on billable starts**
- [ ] **Step 2: Implement monthly cap read + threshold evaluation in policy service**
- [ ] **Step 3: Enforce block in claim/start path (`ClaimTaskByRuntime` / lifecycle gate)**
- [ ] **Step 4: Ensure already-started items support resume/handoff handling**
- [ ] **Step 5: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestMonthlyBudgetBlockBehavior -v`
- [ ] **Step 6: Commit**
```bash
git add server/internal/runtimepolicy/service.go server/internal/handler/daemon.go server/internal/handler/daemon_test.go
git commit -m "feat(policy): enforce monthly budget blocks for billable runtimes"
```

### Task 2.3: Runtime-type-aware enforcement

**Files:**
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/cloudrunner/runner.go`
- Test: `server/internal/cloudrunner/runner_test.go`

- [ ] **Step 1: Add failing tests for local exception during budget blocks**
- [ ] **Step 2: Implement runtime-mode detection from runtime metadata/runtime row**
- [ ] **Step 3: Gate pause/block logic on billable runtime type**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run TestBudgetBlockSkipsLocalRuntime -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/handler/daemon.go server/internal/cloudrunner/runner.go server/internal/cloudrunner/runner_test.go
git commit -m "feat(policy): apply budget blocks only to billable runtimes"
```

### Task 2.4: Configurable global + per-parent concurrency limits

**Files:**
- Modify: `server/pkg/db/queries/runtime.sql`
- Modify: `server/internal/runtimepolicy/service.go`
- Modify: `server/internal/handler/runtime.go`
- Test: `server/internal/handler/runtime_test.go`

- [ ] **Step 1: Add failing tests for workspace limit and per-parent limit**
- [ ] **Step 2: Add storage/query contracts for workspace + parent concurrency caps**
- [ ] **Step 3: Enforce checks before dispatch/start**
- [ ] **Step 4: Add admin API surface in runtime handler**
- [ ] **Step 5: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestRuntimeConcurrencyLimitEnforcement -v`
- [ ] **Step 6: Commit**
```bash
git add server/pkg/db/queries/runtime.sql server/internal/runtimepolicy/service.go server/internal/handler/runtime.go server/internal/handler/runtime_test.go
git commit -m "feat(concurrency): add workspace and parent runtime caps"
```

### Task 2.5: Per-issue admin overrides with auto-expiry

**Files:**
- Modify: `server/internal/runtimepolicy/service.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Write failing test for per-issue override bypass**
- [ ] **Step 2: Implement override model and lookup in policy service**
- [ ] **Step 3: Expire override on issue completion path**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestIssueBudgetOverrideLifecycle -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/runtimepolicy/service.go server/internal/handler/daemon.go server/internal/service/task.go server/internal/handler/daemon_test.go
git commit -m "feat(policy): add per-issue admin override lifecycle"
```

## Chunk 3: Delegation, Ownership, and Lifecycle Transitions

### Task 3.1: Human ownership invariant

**Files:**
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/handler/issue.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing tests ensuring delegated runs retain human owner**
- [ ] **Step 2: Implement validation that delegated issues must have human assignee**
- [ ] **Step 3: Block delegation mutation if no human owner attached**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestDelegationRequiresHumanOwner -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/service/task.go server/internal/handler/issue.go server/internal/handler/daemon_test.go
git commit -m "feat(ownership): enforce human canonical ownership during delegation"
```

### Task 3.2: Owner reassignment while run active

**Files:**
- Modify: `server/internal/handler/issue.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing test for reassignment without forced pause**
- [ ] **Step 2: Update reassignment path to preserve active task + delegation state**
- [ ] **Step 3: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestOwnerReassignmentKeepsRunActive -v`
- [ ] **Step 4: Commit**
```bash
git add server/internal/handler/issue.go server/internal/service/task.go server/internal/handler/daemon_test.go
git commit -m "feat(lifecycle): allow ownership reassignment during active delegated run"
```

### Task 3.3: `needs_human_intervention` for recoverable failures

**Files:**
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing test where partial work + downstream failure maps to intervention**
- [ ] **Step 2: Implement status mapping and transition API logic**
- [ ] **Step 3: Ensure budget pause also maps to intervention**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestRecoverableFailureTransitionsToIntervention -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/handler/daemon.go server/internal/service/task.go server/internal/handler/daemon_test.go
git commit -m "feat(lifecycle): add needs_human_intervention transitions"
```

### Task 3.4: Intervention actions (resume/snapshot/local/archive/force close)

**Files:**
- Modify: `server/internal/handler/runtime.go`
- Modify: `server/internal/cloudrunner/runner.go`
- Modify: `server/internal/cloudrunner/provider/provider.go`
- Test: `server/internal/cloudrunner/runner_test.go`

- [ ] **Step 1: Add failing tests per intervention action contract**
- [ ] **Step 2: Add API payload/type support for intervention actions**
- [ ] **Step 3: Wire runner behavior for each action path**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run TestInterventionActions -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/handler/runtime.go server/internal/cloudrunner/runner.go server/internal/cloudrunner/provider/provider.go server/internal/cloudrunner/runner_test.go
git commit -m "feat(intervention): implement resume/handoff/archive/force-close actions"
```

### Task 3.5: Completion semantics by work type + owner authority

**Files:**
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/handler/daemon.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing tests for code merge gate and non-code owner approval gate**
- [ ] **Step 2: Add work-type branch in completion rules**
- [ ] **Step 3: Enforce assigned owner as sole final completer**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestCompletionSemanticsByWorkType -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/service/task.go server/internal/handler/daemon.go server/internal/handler/daemon_test.go
git commit -m "feat(completion): enforce owner-authorized completion semantics"
```

## Chunk 4: Checkpointing and Resume Orchestration

### Task 4.1: Transition + 5-minute heartbeat checkpointing

**Files:**
- Modify: `server/internal/cloudrunner/session.go`
- Modify: `server/internal/cloudrunner/runner.go`
- Test: `server/internal/cloudrunner/session_test.go`

- [ ] **Step 1: Add failing test for heartbeat cadence while running**
- [ ] **Step 2: Add ticker-driven checkpoint pulse every 5 minutes (injectable clock in tests)**
- [ ] **Step 3: Add transition-hook checkpoint writes**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run 'TestCheckpointHeartbeatCadence|TestTransitionCheckpoint' -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/cloudrunner/session.go server/internal/cloudrunner/runner.go server/internal/cloudrunner/session_test.go
git commit -m "feat(checkpoint): add transition and heartbeat checkpoint cadence"
```

### Task 4.2: Immediate checkpoint triggers

**Files:**
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/cloudrunner/session.go`
- Test: `server/internal/handler/cloud_runtime_session_handler_test.go`

- [ ] **Step 1: Add failing tests for PR update/intervention/threshold/force-close triggers**
- [ ] **Step 2: Insert immediate checkpoint calls at trigger points**
- [ ] **Step 3: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestPersistCloudRuntimeSession_WritesIssueContinuity -v`
- [ ] **Step 4: Commit**
```bash
git add server/internal/handler/daemon.go server/internal/cloudrunner/session.go server/internal/handler/cloud_runtime_session_handler_test.go
git commit -m "feat(checkpoint): trigger immediate persistence on critical lifecycle events"
```

### Task 4.3: Safe-checkpoint pause boundaries

**Files:**
- Modify: `server/internal/cloudrunner/provider/provider.go`
- Modify: `server/internal/cloudrunner/runner.go`
- Test: `server/internal/cloudrunner/runner_test.go`

- [ ] **Step 1: Add failing tests for pause only at safe boundaries**
- [ ] **Step 2: Introduce boundary enum (`command_done`, `git_done`, `pr_sync_done`, `checkpoint_persisted`)**
- [ ] **Step 3: Gate budget pause on boundary checks**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run TestPauseOccursAtSafeBoundaryOnly -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/cloudrunner/provider/provider.go server/internal/cloudrunner/runner.go server/internal/cloudrunner/runner_test.go
git commit -m "feat(pause): enforce safe-checkpoint pause boundaries"
```

### Task 4.4: Resume eligibility + fallback order

**Files:**
- Modify: `server/internal/cloudrunner/session.go`
- Modify: `server/internal/handler/daemon.go`
- Test: `server/internal/cloudrunner/session_test.go`

- [ ] **Step 1: Add failing tests for sandbox health + budget checks**
- [ ] **Step 2: Implement eligibility check for sandbox resume**
- [ ] **Step 3: Implement fallback chain (`sandbox -> snapshot -> local handoff`)**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/cloudrunner -run TestResumeFallbackOrder -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/cloudrunner/session.go server/internal/handler/daemon.go server/internal/cloudrunner/session_test.go
git commit -m "feat(resume): add eligibility checks and fallback resume order"
```

## Chunk 5: UX and Communication Surfaces

### Task 5.1: Budget-block issue-thread comment

**Files:**
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing test asserting out-of-budget comment is created on block**
- [ ] **Step 2: Add comment publish/write path for budget block transitions**
- [ ] **Step 3: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestBudgetBlockPostsIssueComment -v`
- [ ] **Step 4: Commit**
```bash
git add server/internal/handler/daemon.go server/internal/service/task.go server/internal/handler/daemon_test.go
git commit -m "feat(comms): post issue-thread comment when budget blocks agent execution"
```

### Task 5.2: Admin controls for budget caps and concurrency

**Files:**
- Modify: `packages/core/api/client.ts`
- Modify: `packages/views/runtimes/components/cloud-credentials-panel.tsx`
- Modify: `server/internal/handler/runtime.go`
- Test: `server/internal/handler/runtime_test.go`

- [ ] **Step 1: Add failing handler test for admin update endpoint**
- [ ] **Step 2: Add API client methods for budget/cap configuration**
```ts
async updateRuntimePolicy(input: UpdateRuntimePolicyRequest): Promise<RuntimePolicyResponse>
```
- [ ] **Step 3: Add UI controls in cloud credentials panel for caps/limits**
- [ ] **Step 4: Re-run backend tests + typecheck**
Run: `cd server && go test ./internal/handler -run TestRuntimePolicyAdminControls -v`
Run: `pnpm typecheck`
- [ ] **Step 5: Commit**
```bash
git add packages/core/api/client.ts packages/views/runtimes/components/cloud-credentials-panel.tsx server/internal/handler/runtime.go server/internal/handler/runtime_test.go
git commit -m "feat(ui): expose admin runtime budget and concurrency controls"
```

### Task 5.3: Status visibility for blocked/paused/intervention

**Files:**
- Modify: `packages/core/types/agent.ts`
- Modify: `packages/views/runtimes/components/runtime-detail.tsx`
- Modify: `packages/views/runtimes/components/runtime-list.tsx`
- Test: `packages/views/runtimes/components/runtime-list.test.tsx`

- [ ] **Step 1: Add failing UI test for intervention/budget statuses**
- [ ] **Step 2: Extend runtime/task status labels and badges**
- [ ] **Step 3: Render new status chips in list/detail**
- [ ] **Step 4: Run UI tests**
Run: `pnpm --filter @multica/views exec vitest run runtimes/components/runtime-list.test.tsx -v`
- [ ] **Step 5: Commit**
```bash
git add packages/core/types/agent.ts packages/views/runtimes/components/runtime-detail.tsx packages/views/runtimes/components/runtime-list.tsx packages/views/runtimes/components/runtime-list.test.tsx
git commit -m "feat(ui): show blocked paused and intervention runtime states"
```

### Task 5.4: Permission-denied feedback for non-admin actions

**Files:**
- Modify: `packages/views/runtimes/components/cloud-credentials-panel.tsx`
- Test: `packages/views/runtimes/components/cloud-credentials-panel.test.tsx`

- [ ] **Step 1: Add failing test for explicit permission error toast/message**
- [ ] **Step 2: Map API permission errors to clear UI messaging**
- [ ] **Step 3: Run tests**
Run: `pnpm --filter @multica/views exec vitest run runtimes/components/cloud-credentials-panel.test.tsx -v`
- [ ] **Step 4: Commit**
```bash
git add packages/views/runtimes/components/cloud-credentials-panel.tsx packages/views/runtimes/components/cloud-credentials-panel.test.tsx
git commit -m "feat(ui): show explicit permission-denied feedback for policy actions"
```

### Task 5.5: Show configured Vercel runtime as remote in runtime list

**Files:**
- Modify: `server/internal/handler/cloud_runtime_credential.go`
- Modify: `server/internal/handler/runtime.go`
- Modify: `packages/views/runtimes/components/runtime-list.tsx`
- Test: `server/internal/handler/cloud_runtime_credential_test.go`
- Test: `packages/views/runtimes/components/runtime-list.test.tsx`

- [ ] **Step 1: Add failing backend test for bootstrapped runtime list visibility**
- [ ] **Step 2: Ensure bootstrap/upsert sets `runtime_mode='cloud'` and stable owner/runtime metadata**
- [ ] **Step 3: Add/verify UI label for remote runtime mode**
```tsx
<span className="truncate">{runtime.runtime_mode === "cloud" ? "remote" : "local"}</span>
```
- [ ] **Step 4: Run backend + UI tests**
Run: `cd server && go test ./internal/handler -run TestVercelCloudRuntimeCredentialTestAndBootstrap -v`
Run: `pnpm --filter @multica/views exec vitest run runtimes/components/runtime-list.test.tsx -v`
- [ ] **Step 5: Commit**
```bash
git add server/internal/handler/cloud_runtime_credential.go server/internal/handler/runtime.go server/internal/handler/cloud_runtime_credential_test.go packages/views/runtimes/components/runtime-list.tsx packages/views/runtimes/components/runtime-list.test.tsx
git commit -m "feat(runtimes): show configured vercel sandbox as remote runtime in list"
```

## Chunk 6: Metrics and Validation

### Task 6.1: Billable + non-billable usage tracking

**Files:**
- Modify: `server/pkg/db/queries/task_usage.sql` (or `runtime.sql` if centralized)
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/runtimepolicy/service.go`
- Test: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Add failing test for split accounting (billable vercel vs non-billable local)**
- [ ] **Step 2: Add runtime-type tagging to usage writes**
- [ ] **Step 3: Update aggregates to expose billable + non-billable metrics**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestUsageAccountingByRuntimeType -v`
- [ ] **Step 5: Commit**
```bash
git add server/pkg/db/queries/task_usage.sql server/internal/handler/daemon.go server/internal/runtimepolicy/service.go server/internal/handler/daemon_test.go
git commit -m "feat(metrics): track billable and non-billable runtime usage"
```

### Task 6.2: Developer estimate + agent fallback estimate

**Files:**
- Modify: `packages/core/types/issue.ts`
- Modify: `server/internal/handler/issue.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/issue_test.go` (create if missing)

- [ ] **Step 1: Add failing tests for missing-estimate fallback path**
- [ ] **Step 2: Add estimate fields/contracts for issue effort baseline**
- [ ] **Step 3: Implement fallback classifier hook (task type -> baseline hours)**
- [ ] **Step 4: Re-run tests**
Run: `cd server && go test ./internal/handler -run TestIssueEstimateFallback -v`
- [ ] **Step 5: Commit**
```bash
git add packages/core/types/issue.ts server/internal/handler/issue.go server/internal/service/task.go server/internal/handler/issue_test.go
git commit -m "feat(metrics): support human estimate baseline with agent fallback classification"
```

### Task 6.3: Reliability success validation chain

**Files:**
- Modify: `server/internal/cloudrunner/runner_test.go`
- Modify: `server/internal/handler/daemon_test.go`
- Modify: `docs/plans/2026-04-23-vercel-sandbox-cloudruntime-implementation-summary.md` (append validation checklist)

- [ ] **Step 1: Add integration-style tests for end-to-end claim->PR-ready notification chain**
- [ ] **Step 2: Add assertions for provisioning/setup/repo/PR/notification checkpoints**
- [ ] **Step 3: Run tests**
Run: `cd server && go test ./internal/cloudrunner ./internal/handler -run 'TestCloudRuntimeE2E|TestClaimTaskByRuntime' -v`
- [ ] **Step 4: Commit**
```bash
git add server/internal/cloudrunner/runner_test.go server/internal/handler/daemon_test.go docs/plans/2026-04-23-vercel-sandbox-cloudruntime-implementation-summary.md
git commit -m "test(reliability): validate end-to-end cloud runtime workflow checkpoints"
```

### Task 6.4: Full regression test matrix

**Files:**
- Modify: `server/internal/handler/runtime_test.go`
- Modify: `server/internal/handler/daemon_test.go`
- Modify: `server/internal/cloudrunner/session_test.go`
- Modify: `packages/views/runtimes/components/runtime-list.test.tsx`

- [ ] **Step 1: Add missing regression tests for budget, permissions, lifecycle, checkpoint cadence, resume fallback**
- [ ] **Step 2: Execute focused backend test suite**
Run: `cd server && go test ./internal/handler ./internal/cloudrunner -v`
- [ ] **Step 3: Execute frontend runtime tests**
Run: `pnpm --filter @multica/views exec vitest run runtimes/components/runtime-list.test.tsx runtimes/components/cloud-credentials-panel.test.tsx -v`
- [ ] **Step 4: Execute workspace checks**
Run: `pnpm typecheck`
Run: `make test`
- [ ] **Step 5: Commit**
```bash
git add server/internal/handler/runtime_test.go server/internal/handler/daemon_test.go server/internal/cloudrunner/session_test.go packages/views/runtimes/components/runtime-list.test.tsx packages/views/runtimes/components/cloud-credentials-panel.test.tsx
git commit -m "test(runtime-governance): complete regression coverage for governance and recovery flows"
```

## Final Verification Gate

- [ ] Run full implementation checks:
```bash
pnpm typecheck
pnpm test
make test
make check
```
- [ ] Confirm OpenSpec alignment:
  - `openspec/changes/define-cloud-runtime-business-governance/specs/agent-runtime-budget-governance/spec.md`
  - `openspec/changes/define-cloud-runtime-business-governance/specs/delegated-agent-ownership-lifecycle/spec.md`
  - `openspec/changes/define-cloud-runtime-business-governance/specs/resumable-agent-execution-continuity/spec.md`
- [ ] Prepare PR summary with explicit mapping from each `tasks.md` item to merged commits.
