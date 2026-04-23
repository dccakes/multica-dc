package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/runtimepolicy"
)

func createRuntimePolicyTestIssue(t *testing.T, title string) string {
	t.Helper()

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id)
		VALUES ($1, $2, 'todo', 'medium', 'member', $3)
		RETURNING id
	`, testWorkspaceID, title, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	return issueID
}

func TestUpdateWorkspaceRuntimePolicy_PersistsPolicy(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/workspaces/"+testWorkspaceID+"/runtime-policy", map[string]any{
		"monthly_budget_cents":              125000,
		"remote_concurrency_limit":          4,
		"default_parent_issue_budget_cents": 25000,
	})
	req = withURLParam(req, "id", testWorkspaceID)

	testHandler.UpdateWorkspaceRuntimePolicy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpdateWorkspaceRuntimePolicy: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		MonthlyBudgetCents            int64 `json:"monthly_budget_cents"`
		RemoteConcurrencyLimit        int32 `json:"remote_concurrency_limit"`
		DefaultParentIssueBudgetCents int64 `json:"default_parent_issue_budget_cents"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MonthlyBudgetCents != 125000 || resp.RemoteConcurrencyLimit != 4 || resp.DefaultParentIssueBudgetCents != 25000 {
		t.Fatalf("response = %#v, want updated workspace policy", resp)
	}

	store := runtimepolicy.NewStore(testPool)
	policy, err := store.GetWorkspacePolicy(context.Background(), testWorkspaceID)
	if err != nil {
		t.Fatalf("GetWorkspacePolicy: %v", err)
	}
	if policy.MonthlyBudgetCents != 125000 || policy.RemoteConcurrencyLimit != 4 || policy.DefaultParentIssueBudgetCents != 25000 {
		t.Fatalf("stored policy = %#v, want updated workspace policy", policy)
	}
}

func TestIssueRuntimePolicy_ExpiresOnCompletion(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	issueID := createRuntimePolicyTestIssue(t, "runtime policy expiry test")
	store := runtimepolicy.NewStore(testPool)

	putReq := newRequest("PUT", "/api/issues/"+issueID+"/runtime-policy", map[string]any{
		"budget_cents":             9000,
		"remote_concurrency_limit": 1,
	})
	putReq = withURLParam(putReq, "id", issueID)
	putW := httptest.NewRecorder()
	testHandler.UpdateIssueRuntimePolicy(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("UpdateIssueRuntimePolicy: expected 200, got %d: %s", putW.Code, putW.Body.String())
	}

	override, err := store.GetIssueBudgetOverride(context.Background(), issueID)
	if err != nil {
		t.Fatalf("GetIssueBudgetOverride: %v", err)
	}
	if override.BudgetCents != 9000 {
		t.Fatalf("override budget = %d, want 9000", override.BudgetCents)
	}
	if !override.RemoteConcurrencyLimit.Valid || override.RemoteConcurrencyLimit.Int32 != 1 {
		t.Fatalf("override concurrency = %#v, want 1", override.RemoteConcurrencyLimit)
	}

	updateReq := newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"status": "done",
	})
	updateReq = withURLParam(updateReq, "id", issueID)
	updateW := httptest.NewRecorder()
	testHandler.UpdateIssue(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}

	if _, err := store.GetIssueBudgetOverride(context.Background(), issueID); err == nil {
		t.Fatal("expected issue runtime policy override to expire when issue completed")
	}
}
