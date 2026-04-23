package runtimepolicy

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type Store struct {
	db Executor
}

func NewStore(db Executor) *Store {
	return &Store{db: db}
}

type WorkspacePolicy struct {
	WorkspaceID                   string
	MonthlyBudgetCents            int64
	RemoteConcurrencyLimit        int32
	DefaultParentIssueBudgetCents int64
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

type IssueBudgetOverride struct {
	IssueID                string
	BudgetCents            int64
	RemoteConcurrencyLimit pgtype.Int4
	UpdatedBy              string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type TaskCostLedger struct {
	TaskID              string
	WorkspaceID         string
	IssueID             string
	BudgetParentIssueID string
	RuntimeID           string
	RuntimeMode         RuntimeType
	Billable            bool
	SandboxCostCents    int64
	ModelCostCents      int64
	TotalCostCents      int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (s *Store) UpsertWorkspacePolicy(ctx context.Context, policy WorkspacePolicy) (WorkspacePolicy, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO workspace_runtime_policy (
			workspace_id,
			monthly_budget_cents,
			remote_concurrency_limit,
			default_parent_issue_budget_cents
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id)
		DO UPDATE SET
			monthly_budget_cents = EXCLUDED.monthly_budget_cents,
			remote_concurrency_limit = EXCLUDED.remote_concurrency_limit,
			default_parent_issue_budget_cents = EXCLUDED.default_parent_issue_budget_cents,
			updated_at = now()
		RETURNING workspace_id, monthly_budget_cents, remote_concurrency_limit, default_parent_issue_budget_cents, created_at, updated_at
	`, policy.WorkspaceID, policy.MonthlyBudgetCents, policy.RemoteConcurrencyLimit, policy.DefaultParentIssueBudgetCents)
	return scanWorkspacePolicy(row)
}

func (s *Store) GetWorkspacePolicy(ctx context.Context, workspaceID string) (WorkspacePolicy, error) {
	row := s.db.QueryRow(ctx, `
		SELECT workspace_id, monthly_budget_cents, remote_concurrency_limit, default_parent_issue_budget_cents, created_at, updated_at
		FROM workspace_runtime_policy
		WHERE workspace_id = $1
	`, workspaceID)
	policy, err := scanWorkspacePolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return WorkspacePolicy{
				WorkspaceID:            workspaceID,
				RemoteConcurrencyLimit: 2,
			}, nil
		}
		return WorkspacePolicy{}, err
	}
	return policy, nil
}

func (s *Store) UpsertIssueBudgetOverride(ctx context.Context, override IssueBudgetOverride) (IssueBudgetOverride, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO issue_budget_override (
			issue_id,
			budget_cents,
			remote_concurrency_limit,
			updated_by
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (issue_id)
		DO UPDATE SET
			budget_cents = EXCLUDED.budget_cents,
			remote_concurrency_limit = EXCLUDED.remote_concurrency_limit,
			updated_by = EXCLUDED.updated_by,
			updated_at = now()
		RETURNING issue_id, budget_cents, remote_concurrency_limit, updated_by, created_at, updated_at
	`, override.IssueID, override.BudgetCents, override.RemoteConcurrencyLimit, override.UpdatedBy)
	return scanIssueBudgetOverride(row)
}

func (s *Store) GetIssueBudgetOverride(ctx context.Context, issueID string) (IssueBudgetOverride, error) {
	row := s.db.QueryRow(ctx, `
		SELECT issue_id, budget_cents, remote_concurrency_limit, updated_by, created_at, updated_at
		FROM issue_budget_override
		WHERE issue_id = $1
	`, issueID)
	return scanIssueBudgetOverride(row)
}

func (s *Store) DeleteIssueBudgetOverride(ctx context.Context, issueID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM issue_budget_override WHERE issue_id = $1`, issueID)
	return err
}

func (s *Store) UpsertTaskCostLedger(ctx context.Context, row TaskCostLedger) (TaskCostLedger, error) {
	dbRow := s.db.QueryRow(ctx, `
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
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
		RETURNING task_id, workspace_id, issue_id, budget_parent_issue_id, runtime_id, runtime_mode, billable, sandbox_cost_cents, model_cost_cents, total_cost_cents, created_at, updated_at
	`, row.TaskID, row.WorkspaceID, row.IssueID, row.BudgetParentIssueID, nullableUUID(row.RuntimeID), string(row.RuntimeMode), row.Billable, row.SandboxCostCents, row.ModelCostCents, row.TotalCostCents)
	return scanTaskCostLedger(dbRow)
}

func (s *Store) GetIssueCostTotal(ctx context.Context, budgetParentIssueID string) (int64, int64, int64, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(total_cost_cents), 0)::bigint AS total_cost_cents,
			COALESCE(SUM(sandbox_cost_cents), 0)::bigint AS sandbox_cost_cents,
			COALESCE(SUM(model_cost_cents), 0)::bigint AS model_cost_cents
		FROM task_cost_ledger
		WHERE budget_parent_issue_id = $1
		  AND billable = TRUE
	`, budgetParentIssueID)
	var total, sandbox, model int64
	if err := row.Scan(&total, &sandbox, &model); err != nil {
		return 0, 0, 0, err
	}
	return total, sandbox, model, nil
}

func (s *Store) GetWorkspaceCostTotal(ctx context.Context, workspaceID string, since time.Time) (int64, int64, int64, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(total_cost_cents), 0)::bigint AS total_cost_cents,
			COALESCE(SUM(sandbox_cost_cents), 0)::bigint AS sandbox_cost_cents,
			COALESCE(SUM(model_cost_cents), 0)::bigint AS model_cost_cents
		FROM task_cost_ledger
		WHERE workspace_id = $1
		  AND billable = TRUE
		  AND created_at >= $2
	`, workspaceID, since)
	var total, sandbox, model int64
	if err := row.Scan(&total, &sandbox, &model); err != nil {
		return 0, 0, 0, err
	}
	return total, sandbox, model, nil
}

func scanWorkspacePolicy(row pgx.Row) (WorkspacePolicy, error) {
	var policy WorkspacePolicy
	if err := row.Scan(
		&policy.WorkspaceID,
		&policy.MonthlyBudgetCents,
		&policy.RemoteConcurrencyLimit,
		&policy.DefaultParentIssueBudgetCents,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return WorkspacePolicy{}, err
	}
	return policy, nil
}

func scanIssueBudgetOverride(row pgx.Row) (IssueBudgetOverride, error) {
	var override IssueBudgetOverride
	if err := row.Scan(
		&override.IssueID,
		&override.BudgetCents,
		&override.RemoteConcurrencyLimit,
		&override.UpdatedBy,
		&override.CreatedAt,
		&override.UpdatedAt,
	); err != nil {
		return IssueBudgetOverride{}, err
	}
	return override, nil
}

func scanTaskCostLedger(row pgx.Row) (TaskCostLedger, error) {
	var ledger TaskCostLedger
	var runtimeMode string
	var runtimeID pgtype.UUID
	if err := row.Scan(
		&ledger.TaskID,
		&ledger.WorkspaceID,
		&ledger.IssueID,
		&ledger.BudgetParentIssueID,
		&runtimeID,
		&runtimeMode,
		&ledger.Billable,
		&ledger.SandboxCostCents,
		&ledger.ModelCostCents,
		&ledger.TotalCostCents,
		&ledger.CreatedAt,
		&ledger.UpdatedAt,
	); err != nil {
		return TaskCostLedger{}, err
	}
	if runtimeID.Valid {
		ledger.RuntimeID = runtimeID.String()
	}
	ledger.RuntimeMode = RuntimeType(runtimeMode)
	return ledger, nil
}

func nullableUUID(raw string) any {
	if raw == "" {
		return pgtype.UUID{}
	}
	var uuid pgtype.UUID
	_ = uuid.Scan(raw)
	return uuid
}
