package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func newCheckpointTestHandler(t *testing.T) (*Handler, *events.Bus) {
	t.Helper()

	bus := events.New()
	h := New(db.New(testPool), testPool, realtime.NewHub(), bus, service.NewEmailService(), nil, nil, Config{AllowSignup: true})
	return h, bus
}

func TestCompleteTask_EmitsCheckpointEventWhenPRURLPresent(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	h, bus := newCheckpointTestHandler(t)

	agentID := createHandlerTestAgent(t, "Checkpoint Complete Agent", nil)
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'checkpoint completion issue', 'in_progress', 'medium', 'agent', $2, 'member', $3)
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

	checkpoints := make([]protocol.TaskCheckpointPayload, 0, 1)
	bus.Subscribe(protocol.EventTaskCheckpoint, func(e events.Event) {
		payload, ok := e.Payload.(protocol.TaskCheckpointPayload)
		if !ok {
			t.Fatalf("checkpoint payload type = %T, want protocol.TaskCheckpointPayload", e.Payload)
		}
		checkpoints = append(checkpoints, payload)
	})

	task, err := h.TaskService.CompleteTask(context.Background(), parseUUID(taskID), []byte(`{"task_id":"task-1","pr_url":"https://example.com/pr/1","output":"done"}`), "session-1", "/workspace")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if task == nil {
		t.Fatal("CompleteTask returned nil task")
	}

	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint events = %d, want 1", len(checkpoints))
	}
	if checkpoints[0].Reason != "pr_updated" {
		t.Fatalf("checkpoint reason = %q, want pr_updated", checkpoints[0].Reason)
	}
	if checkpoints[0].PRURL != "https://example.com/pr/1" {
		t.Fatalf("checkpoint pr_url = %q, want https://example.com/pr/1", checkpoints[0].PRURL)
	}

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("load issue status: %v", err)
	}
	if status != "in_review" {
		t.Fatalf("issue status = %q, want in_review", status)
	}

	comments, err := testHandler.Queries.ListComments(context.Background(), db.ListCommentsParams{
		IssueID:     parseUUID(issueID),
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	found := false
	for _, comment := range comments {
		if comment.AuthorType == "agent" && comment.Content == "done" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected completion comment, got %#v", comments)
	}
}

func TestApplyIssueIntervention_EmitsCheckpointBeforeForceClose(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	h, bus := newCheckpointTestHandler(t)
	agentID := createHandlerTestAgent(t, "Checkpoint Intervention Agent", nil)

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, assignee_type, assignee_id, creator_type, creator_id)
		VALUES ($1, 'checkpoint intervention issue', 'blocked', 'medium', 'agent', $2, 'member', $3)
		RETURNING id
	`, testWorkspaceID, agentID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	checkpoints := make([]protocol.TaskCheckpointPayload, 0, 1)
	bus.Subscribe(protocol.EventTaskCheckpoint, func(e events.Event) {
		payload, ok := e.Payload.(protocol.TaskCheckpointPayload)
		if !ok {
			t.Fatalf("checkpoint payload type = %T, want protocol.TaskCheckpointPayload", e.Payload)
		}
		checkpoints = append(checkpoints, payload)
	})

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/intervention", map[string]any{
		"action": "force_close",
	})
	req = withURLParam(req, "id", issueID)
	h.ApplyIssueIntervention(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyIssueIntervention: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint events = %d, want 1", len(checkpoints))
	}
	if checkpoints[0].Reason != "pre_force_close" {
		t.Fatalf("checkpoint reason = %q, want pre_force_close", checkpoints[0].Reason)
	}
	if checkpoints[0].InterventionAction != "force_close" {
		t.Fatalf("checkpoint intervention action = %q, want force_close", checkpoints[0].InterventionAction)
	}
}
