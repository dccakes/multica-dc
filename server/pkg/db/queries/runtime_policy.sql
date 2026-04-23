-- name: GetWorkspaceRuntimePolicy :one
SELECT *
FROM workspace_runtime_policy
WHERE workspace_id = $1;

-- name: UpsertWorkspaceRuntimePolicy :one
INSERT INTO workspace_runtime_policy (
    workspace_id,
    monthly_budget_cents,
    remote_concurrency_limit,
    default_parent_issue_budget_cents
) VALUES (
    @workspace_id,
    @monthly_budget_cents,
    @remote_concurrency_limit,
    @default_parent_issue_budget_cents
)
ON CONFLICT (workspace_id)
DO UPDATE SET
    monthly_budget_cents = EXCLUDED.monthly_budget_cents,
    remote_concurrency_limit = EXCLUDED.remote_concurrency_limit,
    default_parent_issue_budget_cents = EXCLUDED.default_parent_issue_budget_cents,
    updated_at = now()
RETURNING *;

-- name: GetIssueBudgetOverride :one
SELECT *
FROM issue_budget_override
WHERE issue_id = $1;

-- name: UpsertIssueBudgetOverride :one
INSERT INTO issue_budget_override (
    issue_id,
    budget_cents,
    updated_by
) VALUES (
    @issue_id,
    @budget_cents,
    @updated_by
)
ON CONFLICT (issue_id)
DO UPDATE SET
    budget_cents = EXCLUDED.budget_cents,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING *;

-- name: DeleteIssueBudgetOverride :exec
DELETE FROM issue_budget_override
WHERE issue_id = $1;

-- name: UpsertTaskCostLedger :one
INSERT INTO task_cost_ledger (
    task_id,
    workspace_id,
    issue_id,
    budget_parent_issue_id,
    runtime_id,
    runtime_mode,
    billable,
    sandbox_cost_cents,
    model_cost_cents,
    total_cost_cents
) VALUES (
    @task_id,
    @workspace_id,
    @issue_id,
    @budget_parent_issue_id,
    @runtime_id,
    @runtime_mode,
    @billable,
    @sandbox_cost_cents,
    @model_cost_cents,
    @total_cost_cents
)
ON CONFLICT (task_id)
DO UPDATE SET
    workspace_id = EXCLUDED.workspace_id,
    issue_id = EXCLUDED.issue_id,
    budget_parent_issue_id = EXCLUDED.budget_parent_issue_id,
    runtime_id = EXCLUDED.runtime_id,
    runtime_mode = EXCLUDED.runtime_mode,
    billable = EXCLUDED.billable,
    sandbox_cost_cents = EXCLUDED.sandbox_cost_cents,
    model_cost_cents = EXCLUDED.model_cost_cents,
    total_cost_cents = EXCLUDED.total_cost_cents,
    updated_at = now()
RETURNING *;

-- name: GetIssueCostTotal :one
SELECT
    COALESCE(SUM(total_cost_cents), 0)::bigint AS total_cost_cents,
    COALESCE(SUM(sandbox_cost_cents), 0)::bigint AS sandbox_cost_cents,
    COALESCE(SUM(model_cost_cents), 0)::bigint AS model_cost_cents
FROM task_cost_ledger
WHERE budget_parent_issue_id = $1
  AND billable = TRUE;

-- name: GetWorkspaceCostTotal :one
SELECT
    COALESCE(SUM(total_cost_cents), 0)::bigint AS total_cost_cents,
    COALESCE(SUM(sandbox_cost_cents), 0)::bigint AS sandbox_cost_cents,
    COALESCE(SUM(model_cost_cents), 0)::bigint AS model_cost_cents
FROM task_cost_ledger
WHERE workspace_id = $1
  AND billable = TRUE
  AND created_at >= date_trunc('month', now());
