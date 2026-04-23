CREATE TABLE workspace_runtime_policy (
    workspace_id UUID PRIMARY KEY REFERENCES workspace(id) ON DELETE CASCADE,
    monthly_budget_cents BIGINT NOT NULL DEFAULT 0,
    remote_concurrency_limit INT NOT NULL DEFAULT 2,
    default_parent_issue_budget_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE issue_budget_override (
    issue_id UUID PRIMARY KEY REFERENCES issue(id) ON DELETE CASCADE,
    budget_cents BIGINT NOT NULL,
    remote_concurrency_limit INT DEFAULT NULL,
    updated_by UUID NOT NULL REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_cost_ledger (
    task_id UUID PRIMARY KEY REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    budget_parent_issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    runtime_id UUID REFERENCES agent_runtime(id) ON DELETE SET NULL,
    runtime_mode TEXT NOT NULL CHECK (runtime_mode IN ('local', 'cloud')),
    billable BOOLEAN NOT NULL DEFAULT TRUE,
    sandbox_cost_cents BIGINT NOT NULL DEFAULT 0,
    model_cost_cents BIGINT NOT NULL DEFAULT 0,
    total_cost_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_cost_ledger_workspace_created
    ON task_cost_ledger(workspace_id, created_at DESC);

CREATE INDEX idx_task_cost_ledger_issue_created
    ON task_cost_ledger(issue_id, created_at DESC);

CREATE INDEX idx_task_cost_ledger_parent_created
    ON task_cost_ledger(budget_parent_issue_id, created_at DESC);
