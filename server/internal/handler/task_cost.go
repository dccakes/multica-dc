package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/multica-ai/multica/server/internal/runtimepolicy"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	defaultCostProfileVCPUs    = 2
	defaultCostProfileMemoryGB = 4.0
)

func (h *Handler) recordTaskCostLedger(ctx context.Context, task db.AgentTaskQueue) {
	store, ok := h.runtimePolicyStore()
	if !ok || !task.RuntimeID.Valid || !task.IssueID.Valid || !task.StartedAt.Valid || !task.CompletedAt.Valid {
		return
	}

	issue, err := h.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		slog.Warn("skip task cost ledger: load issue failed", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	runtime, err := h.Queries.GetAgentRuntime(ctx, task.RuntimeID)
	if err != nil {
		slog.Warn("skip task cost ledger: load runtime failed", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	usageRows, err := h.Queries.GetTaskUsage(ctx, task.ID)
	if err != nil {
		slog.Warn("skip task cost ledger: load task usage failed", "task_id", uuidToString(task.ID), "error", err)
		return
	}

	duration := task.CompletedAt.Time.Sub(task.StartedAt.Time)
	runtimeType := runtimepolicy.ClassifyRuntimeType(runtime.RuntimeMode)
	sandboxCost := runtimepolicy.EstimateSandboxCostCents(runtimeType, duration, defaultCostProfileVCPUs, defaultCostProfileMemoryGB)
	var modelCost int64
	for _, row := range usageRows {
		modelCost += runtimepolicy.EstimateTokenCostCents(row.Model, row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens)
	}

	budgetParentIssueID := issue.ID
	if issue.ParentIssueID.Valid {
		budgetParentIssueID = issue.ParentIssueID
	}

	ledger := runtimepolicy.TaskCostLedger{
		TaskID:              uuidToString(task.ID),
		WorkspaceID:         uuidToString(issue.WorkspaceID),
		IssueID:             uuidToString(task.IssueID),
		BudgetParentIssueID: uuidToString(budgetParentIssueID),
		RuntimeID:           uuidToString(task.RuntimeID),
		RuntimeMode:         runtimepolicy.RuntimeType(runtime.RuntimeMode),
		Billable:            runtimepolicy.IsBillableRuntime(runtime.RuntimeMode),
		SandboxCostCents:    sandboxCost,
		ModelCostCents:      modelCost,
		TotalCostCents:      sandboxCost + modelCost,
	}

	if _, err := store.UpsertTaskCostLedger(ctx, ledger); err != nil {
		slog.Warn("upsert task cost ledger failed", "task_id", uuidToString(task.ID), "error", err)
	}
}

func (h *Handler) recordTaskCostLedgerAsync(ctx context.Context, task db.AgentTaskQueue) {
	go func() {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		h.recordTaskCostLedger(cctx, task)
	}()
}
