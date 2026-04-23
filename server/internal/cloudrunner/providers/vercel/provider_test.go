package vercel

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/cloudrunner/provider"
)

type fakeClient struct {
	calls           []string
	createSandboxIn CreateSandboxInput
	lastExecInput   ExecInput

	createErr   error
	execErr     error
	snapshotErr error
	destroyErr  error

	sandbox  Sandbox
	execRes  ExecResult
	snapshot Snapshot
}

func (f *fakeClient) CreateOrConnect(_ context.Context, in CreateSandboxInput) (Sandbox, error) {
	f.calls = append(f.calls, "create")
	f.createSandboxIn = in
	if f.createErr != nil {
		return Sandbox{}, f.createErr
	}
	if f.sandbox.ID == "" {
		f.sandbox = Sandbox{ID: "sbx_1", Workdir: "/workspace"}
	}
	return f.sandbox, nil
}

func (f *fakeClient) ExecTask(_ context.Context, _ string, in ExecInput) (ExecResult, error) {
	f.calls = append(f.calls, "exec")
	f.lastExecInput = in
	if f.execErr != nil {
		return ExecResult{}, f.execErr
	}
	if f.execRes.Output == "" {
		f.execRes = ExecResult{Output: "ok", WorkDir: "/workspace", Branch: "main", SessionID: "sess_1"}
	}
	return f.execRes, nil
}

func (f *fakeClient) CreateSnapshot(_ context.Context, _ string, _ SnapshotInput) (Snapshot, error) {
	f.calls = append(f.calls, "snapshot")
	if f.snapshotErr != nil {
		return Snapshot{}, f.snapshotErr
	}
	if f.snapshot.ID == "" {
		f.snapshot = Snapshot{ID: "snap_1", ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}
	}
	return f.snapshot, nil
}

func (f *fakeClient) Destroy(context.Context, string, DestroyInput) error {
	f.calls = append(f.calls, "destroy")
	return f.destroyErr
}

func TestProviderExecuteTask_PipelineSuccess(t *testing.T) {
	client := &fakeClient{
		sandbox:  Sandbox{ID: "sbx_1", Workdir: "/workspace"},
		execRes:  ExecResult{Output: "done", WorkDir: "/workspace", Branch: "feat/x", SessionID: "codex_1"},
		snapshot: Snapshot{ID: "snap_1", ExpiresAt: time.Now().UTC().Add(24 * time.Hour)},
	}
	p := NewProvider(client, slog.Default(), WithProviderConfig(ProviderConfig{
		Token:   "tok_123",
		Project: "proj_123",
		Team:    "team_123",
		Command: []string{"codex"},
		Env: map[string]string{
			"BASE_ENV": "1",
		},
	}))
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := p.ExecuteTask(context.Background(), provider.Task{
		ID:               "task_1",
		WorkspaceID:      "ws_1",
		RuntimeID:        "rt_1",
		AgentID:          "ag_1",
		IssueID:          "issue_1",
		PriorWorkDir:     "/repo",
		ResumeSnapshotID: "snap_prev",
		ChatMessage:      "hello world",
		Agent: &provider.AgentData{
			Name:       "codex-agent",
			Model:      "gpt-5",
			CustomEnv:  map[string]string{"ALPHA": "1", "MULTICA_BLOCKED": "x"},
			CustomArgs: []string{"--model", "gpt-5"},
		},
		RuntimeMetadata: map[string]any{
			"project_id":       "runtime-proj",
			"team_id":          "runtime-team",
			"token":            "runtime-token",
			"base_snapshot_id": "runtime-base-snap",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}
	if result.SandboxID != "sbx_1" || result.SnapshotID != "snap_1" {
		t.Fatalf("result = %#v, want sandbox/snapshot IDs", result)
	}
	if client.createSandboxIn.BaseSnapshotID != "snap_prev" {
		t.Fatalf("resume snapshot not forwarded: %#v", client.createSandboxIn)
	}
	if client.createSandboxIn.Project != "runtime-proj" || client.createSandboxIn.Team != "runtime-team" || client.createSandboxIn.Token != "runtime-token" {
		t.Fatalf("create input config not forwarded: %#v", client.createSandboxIn)
	}
	if client.createSandboxIn.Runtime != "node24" {
		t.Fatalf("create runtime not forwarded: %#v", client.createSandboxIn)
	}
	if client.lastExecInput.Workdir != "/repo" {
		t.Fatalf("exec workdir not forwarded: %#v", client.lastExecInput)
	}
	if got, want := strings.Join(client.lastExecInput.Command, " "), "codex --model gpt-5"; got != want {
		t.Fatalf("exec command = %q, want %q", got, want)
	}
	if client.lastExecInput.Env["MULTICA_TASK_ID"] != "task_1" || client.lastExecInput.Env["MULTICA_WORKSPACE_ID"] != "ws_1" {
		t.Fatalf("runtime env not forwarded: %#v", client.lastExecInput.Env)
	}
	if client.lastExecInput.Env["MULTICA_AGENT_NAME"] != "codex-agent" {
		t.Fatalf("agent metadata not forwarded: %#v", client.lastExecInput.Env)
	}
	if client.lastExecInput.Env["MULTICA_RUNTIME_METADATA"] == "" {
		t.Fatalf("runtime metadata env not forwarded: %#v", client.lastExecInput.Env)
	}
	if client.lastExecInput.Env["ALPHA"] != "1" {
		t.Fatalf("custom env not forwarded: %#v", client.lastExecInput.Env)
	}
	if client.lastExecInput.Env["BASE_ENV"] != "1" {
		t.Fatalf("provider env not forwarded: %#v", client.lastExecInput.Env)
	}
	if _, ok := client.lastExecInput.Env["MULTICA_BLOCKED"]; ok {
		t.Fatalf("reserved env should not be forwarded from custom env: %#v", client.lastExecInput.Env)
	}
}

func TestProviderExecuteTask_SnapshotFailureIsNonFatal(t *testing.T) {
	client := &fakeClient{
		snapshotErr: errors.New("snapshot failed"),
	}
	p := NewProvider(client, slog.Default(), WithProviderConfig(ProviderConfig{Command: []string{"codex"}}))
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := p.ExecuteTask(context.Background(), provider.Task{ID: "task_1"})
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v, want nil when only snapshot fails", err)
	}
	if result.Output == "" {
		t.Fatalf("expected execution result even when snapshot fails: %#v", result)
	}
	if result.SnapshotID != "" {
		t.Fatalf("snapshot should be empty on snapshot failure: %#v", result)
	}
}

func TestProviderExecuteTask_CreateFailure(t *testing.T) {
	client := &fakeClient{
		createErr: errors.New("create failed"),
	}
	p := NewProvider(client, slog.Default(), WithProviderConfig(ProviderConfig{Command: []string{"codex"}}))
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err := p.ExecuteTask(context.Background(), provider.Task{ID: "task_1"})
	if err == nil {
		t.Fatal("ExecuteTask() error = nil, want create failure")
	}
}
