package vercel

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	calls []commandCall
	run   func(context.Context, string, []string, []string) ([]byte, []byte, error)
}

type commandCall struct {
	binary string
	args   []string
	env    []string
}

func (f *fakeCommandRunner) Run(ctx context.Context, binary string, args []string, env []string) ([]byte, []byte, error) {
	f.calls = append(f.calls, commandCall{
		binary: binary,
		args:   append([]string{}, args...),
		env:    append([]string{}, env...),
	})
	if f.run != nil {
		return f.run(ctx, binary, args, env)
	}
	switch {
	case contains(args, "create"):
		return []byte(`creating sandbox... sb_abc123xyz`), nil, nil
	case contains(args, "exec"):
		return []byte(`{"stdout":"ok"}`), nil, nil
	case contains(args, "snapshot"):
		return []byte(`snapshot created: snap_abc123`), nil, nil
	case contains(args, "stop"):
		return []byte("stopped"), nil, nil
	default:
		return nil, nil, errors.New("unexpected command")
	}
}

func TestParseSandboxID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:   "json field",
			output: `{"sandbox_id":"sb_json123"}`,
			wantID: "sb_json123",
		},
		{
			name:   "noisy output",
			output: "created sandbox successfully: sb_noise123",
			wantID: "sb_noise123",
		},
		{
			name:    "missing",
			output:  "no id here",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSandboxID([]byte(tc.output))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSandboxID() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSandboxID() error = %v", err)
			}
			if got != tc.wantID {
				t.Fatalf("parseSandboxID() = %q, want %q", got, tc.wantID)
			}
		})
	}
}

func TestParseSnapshotID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:   "json field",
			output: `{"snapshot_id":"snap_json123"}`,
			wantID: "snap_json123",
		},
		{
			name:   "noisy output",
			output: "snapshot complete: snap_noise123",
			wantID: "snap_noise123",
		},
		{
			name:    "missing",
			output:  "no id here",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSnapshotID([]byte(tc.output))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSnapshotID() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSnapshotID() error = %v", err)
			}
			if got != tc.wantID {
				t.Fatalf("parseSnapshotID() = %q, want %q", got, tc.wantID)
			}
		})
	}
}

func TestClientCreateOrConnect_MapsCLIConfigAndParsesSandboxID(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{}
	client := NewClient(ClientConfig{
		Binary:  "sandbox",
		Args:    []string{"--json"},
		Token:   "tok_123",
		Project: "proj_123",
		Team:    "team_123",
		Runtime: "node24",
		Timeout: 30 * time.Minute,
		Workdir: "/workspace",
	}, WithCommandRunner(runner))

	sbx, err := client.CreateOrConnect(context.Background(), CreateSandboxInput{
		BaseSnapshotID: "snap_base123",
		Workdir:        "/sandbox-work",
	})
	if err != nil {
		t.Fatalf("CreateOrConnect() error = %v", err)
	}
	if sbx.ID != "sb_abc123xyz" {
		t.Fatalf("CreateOrConnect() sandbox id = %q, want %q", sbx.ID, "sb_abc123xyz")
	}
	if sbx.Workdir != "/sandbox-work" {
		t.Fatalf("CreateOrConnect() workdir = %q, want %q", sbx.Workdir, "/sandbox-work")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	wantArgs := []string{
		"--json",
		"create",
		"--token", "tok_123",
		"--project", "proj_123",
		"--scope", "team_123",
		"--snapshot", "snap_base123",
		"--timeout", "30m",
	}
	if !reflect.DeepEqual(runner.calls[0].args, wantArgs) {
		t.Fatalf("create args = %#v, want %#v", runner.calls[0].args, wantArgs)
	}
}

func TestClientExecTask_MapsRuntimeInputs(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{}
	client := NewClient(ClientConfig{
		Binary:  "sandbox",
		Args:    []string{"--json"},
		Token:   "tok_123",
		Project: "proj_123",
		Team:    "team_123",
		Workdir: "/workspace",
	}, WithCommandRunner(runner))

	result, err := client.ExecTask(context.Background(), "sb_abc123xyz", ExecInput{
		Workdir: "/repo",
		Command: []string{"codex", "--model", "gpt-5"},
		Env: map[string]string{
			"ALPHA": "1",
			"ZETA":  "9",
		},
	})
	if err != nil {
		t.Fatalf("ExecTask() error = %v", err)
	}
	if result.WorkDir != "/repo" {
		t.Fatalf("ExecTask() workdir = %q, want %q", result.WorkDir, "/repo")
	}
	if result.Output != `{"stdout":"ok"}` {
		t.Fatalf("ExecTask() output = %q, want json output", result.Output)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	wantArgs := []string{
		"--json",
		"exec",
		"--token", "tok_123",
		"--project", "proj_123",
		"--scope", "team_123",
		"--workdir", "/repo",
		"--env", "ALPHA=1",
		"--env", "ZETA=9",
		"sb_abc123xyz",
		"codex",
		"--model", "gpt-5",
	}
	if !reflect.DeepEqual(runner.calls[0].args, wantArgs) {
		t.Fatalf("exec args = %#v, want %#v", runner.calls[0].args, wantArgs)
	}
}

func TestClientCreateSnapshotAndDestroy_UseCLIArguments(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{}
	client := NewClient(ClientConfig{
		Binary:  "sandbox",
		Args:    []string{"--json"},
		Token:   "tok_123",
		Project: "proj_123",
		Team:    "team_123",
		Workdir: "/workspace",
	}, WithCommandRunner(runner))

	snapshot, err := client.CreateSnapshot(context.Background(), "sb_abc123xyz", SnapshotInput{})
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}
	if snapshot.ID != "snap_abc123" {
		t.Fatalf("CreateSnapshot() snapshot id = %q, want %q", snapshot.ID, "snap_abc123")
	}
	if err := client.Destroy(context.Background(), "sb_abc123xyz", DestroyInput{}); err != nil {
		t.Fatalf("Destroy() error = %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %d, want 2", len(runner.calls))
	}
	wantSnapshotArgs := []string{
		"--json",
		"snapshot",
		"--token", "tok_123",
		"--project", "proj_123",
		"--scope", "team_123",
		"sb_abc123xyz",
		"--stop",
	}
	if !reflect.DeepEqual(runner.calls[0].args, wantSnapshotArgs) {
		t.Fatalf("snapshot args = %#v, want %#v", runner.calls[0].args, wantSnapshotArgs)
	}
	wantDestroyArgs := []string{
		"--json",
		"stop",
		"--token", "tok_123",
		"--project", "proj_123",
		"--scope", "team_123",
		"sb_abc123xyz",
	}
	if !reflect.DeepEqual(runner.calls[1].args, wantDestroyArgs) {
		t.Fatalf("destroy args = %#v, want %#v", runner.calls[1].args, wantDestroyArgs)
	}
}

func TestClientStageErrorNormalization(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{}
	runner.run = func(_ context.Context, _ string, _ []string, _ []string) ([]byte, []byte, error) {
		return []byte(""), []byte("permission denied"), errors.New("exit status 1")
	}
	client := NewClient(ClientConfig{Binary: "sandbox"}, WithCommandRunner(runner))

	_, err := client.ExecTask(context.Background(), "sb_abc123xyz", ExecInput{Command: []string{"codex"}})
	if err == nil {
		t.Fatal("ExecTask() error = nil, want exec failure")
	}
	if got := err.Error(); !strings.Contains(got, "exec sandbox") {
		t.Fatalf("ExecTask() error = %q, want stage-specific error", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
