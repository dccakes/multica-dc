package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompleteTask_WritesCostLedger(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()

	var agentID, runtimeID string
	if err := testPool.QueryRow(ctx, `
		SELECT a.id, a.runtime_id
		FROM agent a
		WHERE a.workspace_id = $1
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("fetch agent/runtime: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id)
		VALUES ($1, 'task cost ledger test', 'todo', 'medium', 'member', $2)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	startedAt := time.Now().UTC().Add(-5 * time.Minute)
	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, issue_id, runtime_id, status, started_at)
		VALUES ($1, $2, $3, 'running', $4)
		RETURNING id
	`, agentID, issueID, runtimeID, startedAt).Scan(&taskID); err != nil {
		t.Fatalf("create running task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	if _, err := testPool.Exec(ctx, `
		INSERT INTO task_usage (task_id, provider, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens)
		VALUES ($1, 'claude', 'claude-sonnet-4-6', 1000000, 0, 0, 0)
	`, taskID); err != nil {
		t.Fatalf("insert task usage: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM task_usage WHERE task_id = $1`, taskID)
	})

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output":              "done",
		"session_id":          "session-1",
		"work_dir":            "/workspace",
		"branch_name":         "main",
		"snapshot_id":         "snapshot-1",
		"sandbox_id":          "sandbox-1",
		"snapshot_expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	}, testWorkspaceID, "ledger-test-daemon")
	req = withURLParam(req, "taskId", taskID)

	testHandler.CompleteTask(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sandboxCents, modelCents, totalCents int64
	var billable bool
	if err := testPool.QueryRow(ctx, `
		SELECT sandbox_cost_cents, model_cost_cents, total_cost_cents, billable
		FROM task_cost_ledger
		WHERE task_id = $1
	`, taskID).Scan(&sandboxCents, &modelCents, &totalCents, &billable); err != nil {
		if err == sql.ErrNoRows {
			t.Fatal("expected task_cost_ledger row to be written")
		}
		t.Fatalf("read task_cost_ledger: %v", err)
	}
	if !billable {
		t.Fatal("expected vercel task to be marked billable")
	}
	if sandboxCents != 3 {
		t.Fatalf("sandbox cost = %d, want 3", sandboxCents)
	}
	if modelCents != 300 {
		t.Fatalf("model cost = %d, want 300", modelCents)
	}
	if totalCents != 303 {
		t.Fatalf("total cost = %d, want 303", totalCents)
	}
}
