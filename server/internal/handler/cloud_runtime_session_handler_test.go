package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPersistCloudRuntimeSession_WritesIssueContinuity(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()

	var agentID, runtimeID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		SELECT id, runtime_id FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("load agent/runtime: %v", err)
	}

	var issueID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, position, creator_type, creator_id)
		VALUES ($1, 'Cloud session persistence issue', 'todo', 'medium', 0, 'member', gen_random_uuid())
		RETURNING id
	`, testWorkspaceID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	task, err := testHandler.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:   agentID,
		IssueID:   issueID,
		Priority:  0,
		RuntimeID: runtimeID,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, task.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM cloud_runtime_session WHERE runtime_id = $1 AND agent_id = $2 AND issue_id = $3`, runtimeID, agentID, issueID)
	})

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	if err := testHandler.persistCloudRuntimeSession(ctx, task, "snapshot_1", "sandbox_1", "feature/cloud", "codex_1", "/workspace", expiresAt); err != nil {
		t.Fatalf("persistCloudRuntimeSession: %v", err)
	}

	got, err := testHandler.Queries.GetCloudRuntimeSessionForIssue(ctx, db.GetCloudRuntimeSessionForIssueParams{
		RuntimeID: runtimeID,
		AgentID:   agentID,
		IssueID:   issueID,
	})
	if err != nil {
		t.Fatalf("GetCloudRuntimeSessionForIssue: %v", err)
	}
	if got.LastSnapshotID.String != "snapshot_1" {
		t.Fatalf("LastSnapshotID = %q, want snapshot_1", got.LastSnapshotID.String)
	}
	if got.LastSandboxID.String != "sandbox_1" {
		t.Fatalf("LastSandboxID = %q, want sandbox_1", got.LastSandboxID.String)
	}
	if got.LastBranch.String != "feature/cloud" {
		t.Fatalf("LastBranch = %q, want feature/cloud", got.LastBranch.String)
	}
	if got.LastCodexSessionID.String != "codex_1" {
		t.Fatalf("LastCodexSessionID = %q, want codex_1", got.LastCodexSessionID.String)
	}
}
