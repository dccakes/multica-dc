package handler

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/runtimepolicy"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func createRuntimePolicyClaimTestRuntime(t *testing.T, runtimeMode, provider, name string) (string, string) {
	t.Helper()

	ctx := context.Background()
	var runtimeID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, $3, $4, 'online', $5, '{}'::jsonb, now())
		RETURNING id
	`, testWorkspaceID, name, runtimeMode, provider, name).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', $3, '{}'::jsonb, $4, 'workspace', 1, $5)
		RETURNING id
	`, testWorkspaceID, name+" Agent", runtimeMode, runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})

	return runtimeID, agentID
}

func createRuntimePolicyClaimTestAgent(t *testing.T, runtimeID, runtimeMode, name string) string {
	t.Helper()

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', $3, '{}'::jsonb, $4, 'workspace', 1, $5)
		RETURNING id
	`, testWorkspaceID, name, runtimeMode, runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})

	return agentID
}

func createRuntimePolicyClaimTestIssue(t *testing.T, title string, parentID string) string {
	t.Helper()

	ctx := context.Background()
	var issueID string
	if parentID == "" {
		if err := testPool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id)
			VALUES ($1, $2, 'todo', 'medium', 'member', $3)
			RETURNING id
		`, testWorkspaceID, title, testUserID).Scan(&issueID); err != nil {
			t.Fatalf("create issue: %v", err)
		}
	} else {
		if err := testPool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, parent_issue_id)
			VALUES ($1, $2, 'todo', 'medium', 'member', $3, $4)
			RETURNING id
		`, testWorkspaceID, title, testUserID, parentID).Scan(&issueID); err != nil {
			t.Fatalf("create issue with parent: %v", err)
		}
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	return issueID
}

func createRuntimePolicyClaimTestQueuedTask(t *testing.T, agentID, issueID, runtimeID string) string {
	t.Helper()

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, issue_id, runtime_id, status, priority)
		VALUES ($1, $2, $3, 'queued', 0)
		RETURNING id
	`, agentID, issueID, runtimeID).Scan(&taskID); err != nil {
		t.Fatalf("create queued task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	return taskID
}

func createRuntimePolicyClaimTestBurnTask(t *testing.T, agentID, issueID, runtimeID string) string {
	t.Helper()

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, issue_id, runtime_id, status, priority, started_at, completed_at)
		VALUES ($1, $2, $3, 'completed', 0, now(), now())
		RETURNING id
	`, agentID, issueID, runtimeID).Scan(&taskID); err != nil {
		t.Fatalf("create burn task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	return taskID
}

func upsertRuntimePolicyWorkspace(t *testing.T, monthlyBudget, remoteConcurrency, defaultParentBudget int64) {
	t.Helper()

	store := runtimepolicy.NewStore(testPool)
	if _, err := store.UpsertWorkspacePolicy(context.Background(), runtimepolicy.WorkspacePolicy{
		WorkspaceID:                   testWorkspaceID,
		MonthlyBudgetCents:            monthlyBudget,
		RemoteConcurrencyLimit:        int32(remoteConcurrency),
		DefaultParentIssueBudgetCents: defaultParentBudget,
	}); err != nil {
		t.Fatalf("upsert workspace policy: %v", err)
	}
}

func upsertRuntimePolicyOverride(t *testing.T, issueID string, budgetCents int64, concurrency int32) {
	t.Helper()

	store := runtimepolicy.NewStore(testPool)
	if _, err := store.UpsertIssueBudgetOverride(context.Background(), runtimepolicy.IssueBudgetOverride{
		IssueID:                issueID,
		BudgetCents:            budgetCents,
		RemoteConcurrencyLimit: pgtype.Int4{Int32: concurrency, Valid: true},
		UpdatedBy:              testUserID,
	}); err != nil {
		t.Fatalf("upsert issue override: %v", err)
	}
}

func insertRuntimePolicyLedger(t *testing.T, taskID, issueID, parentIssueID, runtimeID string, totalCents int64) {
	t.Helper()

	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO task_cost_ledger (
			task_id, workspace_id, issue_id, budget_parent_issue_id, runtime_id, runtime_mode,
			billable, sandbox_cost_cents, model_cost_cents, total_cost_cents
		)
		VALUES ($1, $2, $3, $4, $5, 'cloud', TRUE, $6, 0, $6)
	`, taskID, testWorkspaceID, issueID, parentIssueID, runtimeID, totalCents); err != nil {
		t.Fatalf("insert task cost ledger: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM task_cost_ledger WHERE task_id = $1`, taskID)
	})
}

func TestClaimTaskForRuntime_SkipsBillableWhenParentBudgetBlocked(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	upsertRuntimePolicyWorkspace(t, 10_000, 2, 100)
	runtimeID, agentID := createRuntimePolicyClaimTestRuntime(t, "cloud", "vercel", "Budget Block Runtime")
	parentIssueID := createRuntimePolicyClaimTestIssue(t, "parent budget issue", "")
	childIssueID := createRuntimePolicyClaimTestIssue(t, "child budget issue", parentIssueID)

	burnTaskID := createRuntimePolicyClaimTestBurnTask(t, agentID, parentIssueID, runtimeID)
	insertRuntimePolicyLedger(t, burnTaskID, parentIssueID, parentIssueID, runtimeID, 100)
	createRuntimePolicyClaimTestQueuedTask(t, agentID, childIssueID, runtimeID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("ClaimTaskForRuntime: %v", err)
	}
	if task != nil {
		t.Fatalf("expected billable task to be blocked, got %#v", task)
	}

	comments, err := testHandler.Queries.ListComments(context.Background(), db.ListCommentsParams{
		IssueID:     parseUUID(childIssueID),
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	found := false
	for _, comment := range comments {
		if comment.AuthorType == "system" && comment.Content == "Out of budget: budget block: parent issue budget exhausted." {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected out-of-budget system comment on issue thread, got %#v", comments)
	}
}

func TestClaimTaskForRuntime_SkipsBillableWhenMonthlyBudgetBlocked(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	upsertRuntimePolicyWorkspace(t, 100, 2, 10_000)
	runtimeID, agentID := createRuntimePolicyClaimTestRuntime(t, "cloud", "vercel", "Monthly Block Runtime")
	issueID := createRuntimePolicyClaimTestIssue(t, "monthly budget issue", "")
	burnTaskID := createRuntimePolicyClaimTestBurnTask(t, agentID, issueID, runtimeID)
	insertRuntimePolicyLedger(t, burnTaskID, issueID, issueID, runtimeID, 100)
	createRuntimePolicyClaimTestQueuedTask(t, agentID, issueID, runtimeID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("ClaimTaskForRuntime: %v", err)
	}
	if task != nil {
		t.Fatalf("expected monthly budget to block billable task, got %#v", task)
	}
}

func TestClaimTaskForRuntime_AllowsLocalRuntimeWhenBudgetBlocked(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	upsertRuntimePolicyWorkspace(t, 100, 2, 10_000)
	billableRuntimeID, billableAgentID := createRuntimePolicyClaimTestRuntime(t, "cloud", "vercel", "Billable Block Runtime")
	issueID := createRuntimePolicyClaimTestIssue(t, "local runtime issue", "")
	burnTaskID := createRuntimePolicyClaimTestBurnTask(t, billableAgentID, issueID, billableRuntimeID)
	insertRuntimePolicyLedger(t, burnTaskID, issueID, issueID, billableRuntimeID, 100)
	localRuntimeID, localAgentID := createRuntimePolicyClaimTestRuntime(t, "local", "claude", "Local Runtime")
	localTaskID := createRuntimePolicyClaimTestQueuedTask(t, localAgentID, issueID, localRuntimeID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(context.Background(), parseUUID(localRuntimeID))
	if err != nil {
		t.Fatalf("ClaimTaskForRuntime: %v", err)
	}
	if task == nil || uuidToString(task.ID) != localTaskID {
		t.Fatalf("expected local task to be claimable, got %#v", task)
	}
}

func TestClaimTaskForRuntime_RespectsParentConcurrencyLimit(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	upsertRuntimePolicyWorkspace(t, 10_000, 2, 10_000)
	runtimeID, activeAgentID := createRuntimePolicyClaimTestRuntime(t, "cloud", "vercel", "Concurrency Runtime")
	queuedAgentID := createRuntimePolicyClaimTestAgent(t, runtimeID, "cloud", "Queued Agent")
	parentIssueID := createRuntimePolicyClaimTestIssue(t, "parent concurrency issue", "")
	childIssueID := createRuntimePolicyClaimTestIssue(t, "child concurrency issue", parentIssueID)
	burnTaskID := createRuntimePolicyClaimTestBurnTask(t, activeAgentID, parentIssueID, runtimeID)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE agent_id = $1 AND issue_id = $2 AND runtime_id = $3 AND status = 'completed'
	`, activeAgentID, parentIssueID, runtimeID); err != nil {
		t.Fatalf("promote burn task to running: %v", err)
	}
	createRuntimePolicyClaimTestQueuedTask(t, queuedAgentID, childIssueID, runtimeID)
	_ = burnTaskID

	task, err := testHandler.TaskService.ClaimTaskForRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("ClaimTaskForRuntime: %v", err)
	}
	if task != nil {
		t.Fatalf("expected parent concurrency cap to block queued task, got %#v", task)
	}
}

func TestClaimTaskForRuntime_RespectsWorkspaceConcurrencyLimit(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	upsertRuntimePolicyWorkspace(t, 10_000, 1, 10_000)
	runtimeID, activeAgentID := createRuntimePolicyClaimTestRuntime(t, "cloud", "vercel", "Workspace Concurrency Runtime")
	queuedAgentID := createRuntimePolicyClaimTestAgent(t, runtimeID, "cloud", "Queued Workspace Agent")
	activeIssueID := createRuntimePolicyClaimTestIssue(t, "active workspace issue", "")
	queuedIssueID := createRuntimePolicyClaimTestIssue(t, "queued workspace issue", "")
	burnTaskID := createRuntimePolicyClaimTestBurnTask(t, activeAgentID, activeIssueID, runtimeID)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue SET status = 'running', started_at = now()
		WHERE id = $1
	`, burnTaskID); err != nil {
		t.Fatalf("promote burn task to running: %v", err)
	}
	upsertRuntimePolicyOverride(t, queuedIssueID, 10_000, 2)
	createRuntimePolicyClaimTestQueuedTask(t, queuedAgentID, queuedIssueID, runtimeID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("ClaimTaskForRuntime: %v", err)
	}
	if task != nil {
		t.Fatalf("expected workspace concurrency cap to block queued task, got %#v", task)
	}
}
