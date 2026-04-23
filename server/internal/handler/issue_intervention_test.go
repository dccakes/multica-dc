package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyIssueIntervention_ResumeSnapshotQueuesTaskWithContext(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "Intervention Agent", nil)

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'intervention resume issue', 'blocked', 'medium', 'agent', $2, 'member', $3)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/intervention", map[string]any{
		"action": "resume_from_snapshot",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.ApplyIssueIntervention(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyIssueIntervention: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("load issue status: %v", err)
	}
	if status != "in_progress" {
		t.Fatalf("issue status = %q, want in_progress", status)
	}

	var taskContext []byte
	if err := testPool.QueryRow(context.Background(), `
		SELECT context
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, parseUUID(issueID)).Scan(&taskContext); err != nil {
		t.Fatalf("load task context: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(taskContext, &decoded); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	if decoded["intervention_action"] != "resume_from_snapshot" {
		t.Fatalf("task context = %#v, want resume_from_snapshot", decoded)
	}
}

func TestApplyIssueIntervention_ArchiveCancelsActiveTasks(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "Archive Agent", nil)

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'intervention archive issue', 'blocked', 'medium', 'agent', $2, 'member', $3)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, issue_id, runtime_id, status, priority, started_at)
		VALUES ($1, $2, $3, 'running', 0, now())
		RETURNING id
	`, parseUUID(agentID), parseUUID(issueID), parseUUID(handlerTestRuntimeID(t))).Scan(&taskID); err != nil {
		t.Fatalf("create running task: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/intervention", map[string]any{
		"action": "archive",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.ApplyIssueIntervention(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyIssueIntervention: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("load issue status: %v", err)
	}
	if status != "cancelled" {
		t.Fatalf("issue status = %q, want cancelled", status)
	}

	var taskStatus string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&taskStatus); err != nil {
		t.Fatalf("load task status: %v", err)
	}
	if taskStatus != "cancelled" {
		t.Fatalf("task status = %q, want cancelled", taskStatus)
	}
}

func TestApplyIssueIntervention_RejectsResumeWhenAgentInactive(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "Inactive Intervention Agent", nil)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent
		SET archived_at = now()
		WHERE id = $1
	`, agentID); err != nil {
		t.Fatalf("archive agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'inactive agent intervention issue', 'blocked', 'medium', 'agent', $2, 'member', $3)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/intervention", map[string]any{
		"action": "resume_from_snapshot",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.ApplyIssueIntervention(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("ApplyIssueIntervention: expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("load issue status: %v", err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}

	var taskCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, parseUUID(issueID)).Scan(&taskCount); err != nil {
		t.Fatalf("load task count: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("task count = %d, want 0", taskCount)
	}
}

func TestApplyIssueIntervention_RejectsNonOwner(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "Permission Agent", nil)

	var otherUserID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO "user" (name, email)
		VALUES ('Intervention Permission User', 'intervention-permission@multica.ai')
		RETURNING id
	`).Scan(&otherUserID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, testWorkspaceID, otherUserID); err != nil {
		t.Fatalf("create member: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM member WHERE user_id = $1 AND workspace_id = $2`, otherUserID, testWorkspaceID)
	})

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'intervention permission issue', 'blocked', 'medium', 'agent', $2, 'member', $3)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/intervention", map[string]any{
		"action": "archive",
	})
	req.Header.Set("X-User-ID", otherUserID)
	req = withURLParam(req, "id", issueID)
	testHandler.ApplyIssueIntervention(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("ApplyIssueIntervention: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("load issue status: %v", err)
	}
	if status != "blocked" {
		t.Fatalf("issue status = %q, want blocked", status)
	}
}
