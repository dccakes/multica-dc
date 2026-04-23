package cloudrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendHeartbeat_SendsAuthAndPayload(t *testing.T) {
	var gotAuth string
	var gotRuntimeID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/daemon/heartbeat" {
			t.Fatalf("path = %s, want /api/daemon/heartbeat", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotRuntimeID = body["runtime_id"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	client.SetToken("secret-token")
	if err := client.SendHeartbeat(context.Background(), "rt-123"); err != nil {
		t.Fatalf("SendHeartbeat() error = %v", err)
	}

	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotRuntimeID != "rt-123" {
		t.Fatalf("runtime_id = %q, want rt-123", gotRuntimeID)
	}
}

func TestClientClaimTask_DecodesTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/daemon/runtimes/rt-123/tasks/claim" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task":{"id":"task-1","runtime_id":"rt-123","agent_id":"ag-1","issue_id":"is-1","workspace_id":"ws-1","prior_session_id":"sess-1","prior_work_dir":"/workspace","runtime_metadata":{"project_id":"prj-1","token":"tok-1"}}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	task, err := client.ClaimTask(context.Background(), "rt-123")
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if task == nil || task.ID != "task-1" || task.RuntimeID != "rt-123" {
		t.Fatalf("task = %#v, want decoded task", task)
	}
	if task.AgentID != "ag-1" || task.IssueID != "is-1" || task.PriorSessionID != "sess-1" {
		t.Fatalf("task continuity fields not decoded: %#v", task)
	}
	if task.RuntimeMetadata["project_id"] != "prj-1" || task.RuntimeMetadata["token"] != "tok-1" {
		t.Fatalf("runtime_metadata not decoded: %#v", task.RuntimeMetadata)
	}
}

func TestClientPostJSON_Non2xxReturnsRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	err := client.SendHeartbeat(context.Background(), "rt-123")
	if err == nil {
		t.Fatal("SendHeartbeat() error = nil, want requestError")
	}
	reqErr, ok := err.(*requestError)
	if !ok {
		t.Fatalf("error type = %T, want *requestError", err)
	}
	if reqErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", reqErr.StatusCode, http.StatusUnauthorized)
	}
	if !strings.Contains(reqErr.Body, "unauthorized") {
		t.Fatalf("Body = %q, want contains unauthorized", reqErr.Body)
	}
}

func TestClientTaskLifecycleEndpoints(t *testing.T) {
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	if err := client.StartTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if err := client.CompleteTask(context.Background(), "task-1", "ok", "main", "sess", "/workspace"); err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if err := client.FailTask(context.Background(), "task-2", "failed", "sess2", "/workspace2"); err != nil {
		t.Fatalf("FailTask() error = %v", err)
	}

	if !seen["/api/daemon/tasks/task-1/start"] {
		t.Fatalf("missing start endpoint call: %+v", seen)
	}
	if !seen["/api/daemon/tasks/task-1/complete"] {
		t.Fatalf("missing complete endpoint call: %+v", seen)
	}
	if !seen["/api/daemon/tasks/task-2/fail"] {
		t.Fatalf("missing fail endpoint call: %+v", seen)
	}
}
