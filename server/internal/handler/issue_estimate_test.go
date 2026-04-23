package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueEstimateFallbackUsesClassificationWhenMissing(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Write tests for sandbox resume behavior",
		"description": "Cover the workflow with a regression test and make sure it fails before the fix.",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.EstimatedHours == nil {
		t.Fatalf("expected fallback estimated_hours to be populated")
	}
	if got := *created.EstimatedHours; got != 8 {
		t.Fatalf("expected fallback estimated_hours 8, got %v", got)
	}
	if created.EstimateSource == nil || *created.EstimateSource != "agent" {
		t.Fatalf("expected fallback estimate_source agent, got %v", created.EstimateSource)
	}

	cleanupReq := newRequest(http.MethodDelete, "/api/issues/"+created.ID, nil)
	cleanupReq = withURLParam(cleanupReq, "id", created.ID)
	testHandler.DeleteIssue(httptest.NewRecorder(), cleanupReq)
}

func TestIssueEstimatePreservesDeveloperProvidedBaseline(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":           "Plan the sandbox rollout",
		"estimated_hours": 6.5,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.EstimatedHours == nil {
		t.Fatalf("expected estimated_hours to be populated")
	}
	if got := *created.EstimatedHours; got != 6.5 {
		t.Fatalf("expected provided estimated_hours 6.5, got %v", got)
	}
	if created.EstimateSource == nil || *created.EstimateSource != "human" {
		t.Fatalf("expected estimate_source human, got %v", created.EstimateSource)
	}

	cleanupReq := newRequest(http.MethodDelete, "/api/issues/"+created.ID, nil)
	cleanupReq = withURLParam(cleanupReq, "id", created.ID)
	testHandler.DeleteIssue(httptest.NewRecorder(), cleanupReq)
}
