package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CreateSandboxInput struct {
	Token          string
	Project        string
	Team           string
	BaseSnapshotID string
	Runtime        string
	Timeout        time.Duration
	Workdir        string
}

type Sandbox struct {
	ID      string
	Workdir string
}

type ExecInput struct {
	Token   string
	Project string
	Team    string
	Workdir string
	Command []string
	Env     map[string]string
}

type ExecResult struct {
	Output    string
	WorkDir   string
	Branch    string
	SessionID string
}

type Snapshot struct {
	ID        string
	ExpiresAt time.Time
}

type SnapshotInput struct {
	Token   string
	Project string
	Team    string
}

type DestroyInput struct {
	Token   string
	Project string
	Team    string
}

type Client interface {
	CreateOrConnect(ctx context.Context, in CreateSandboxInput) (Sandbox, error)
	ExecTask(ctx context.Context, sandboxID string, in ExecInput) (ExecResult, error)
	CreateSnapshot(ctx context.Context, sandboxID string, in SnapshotInput) (Snapshot, error)
	Destroy(ctx context.Context, sandboxID string, in DestroyInput) error
}

type ClientConfig struct {
	Binary         string
	Args           []string
	Token          string
	Project        string
	Team           string
	BaseSnapshotID string
	Runtime        string
	Timeout        time.Duration
	Workdir        string
}

type ClientOption func(*CLIClient)

type CommandRunner interface {
	Run(ctx context.Context, binary string, args []string, env []string) ([]byte, []byte, error)
}

type CLIClient struct {
	cfg    ClientConfig
	runner CommandRunner
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, binary string, args []string, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

// StubClient is a local no-op implementation used by v1 scaffolding/tests.
type StubClient struct{}

func NewStubClient() *StubClient { return &StubClient{} }

func NewClient(cfg ClientConfig, opts ...ClientOption) *CLIClient {
	client := &CLIClient{
		cfg:    normalizeClientConfig(cfg),
		runner: execRunner{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	if client.runner == nil {
		client.runner = execRunner{}
	}
	client.cfg = normalizeClientConfig(client.cfg)
	return client
}

func WithCommandRunner(runner CommandRunner) ClientOption {
	return func(c *CLIClient) {
		c.runner = runner
	}
}

func normalizeClientConfig(cfg ClientConfig) ClientConfig {
	if strings.TrimSpace(cfg.Binary) == "" {
		cfg.Binary = "sandbox"
	}
	if strings.TrimSpace(cfg.Runtime) == "" {
		cfg.Runtime = "node24"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Hour
	}
	if strings.TrimSpace(cfg.Workdir) == "" {
		cfg.Workdir = "/workspace"
	}
	return cfg
}

func (c *CLIClient) CreateOrConnect(ctx context.Context, in CreateSandboxInput) (Sandbox, error) {
	args := append([]string{}, c.cfg.Args...)
	args = append(args, "create")

	token := firstNonEmpty(in.Token, c.cfg.Token)
	project := firstNonEmpty(in.Project, c.cfg.Project)
	team := firstNonEmpty(in.Team, c.cfg.Team)
	baseSnapshotID := strings.TrimSpace(in.BaseSnapshotID)
	if baseSnapshotID == "" {
		baseSnapshotID = strings.TrimSpace(c.cfg.BaseSnapshotID)
	}
	runtime := firstNonEmpty(in.Runtime, c.cfg.Runtime)
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = c.cfg.Timeout
	}

	if token != "" {
		args = append(args, "--token", token)
	}
	if project != "" {
		args = append(args, "--project", project)
	}
	if team != "" {
		args = append(args, "--scope", team)
	}
	if baseSnapshotID != "" {
		args = append(args, "--snapshot", baseSnapshotID)
	} else if runtime != "" {
		args = append(args, "--runtime", runtime)
	}
	if timeout > 0 {
		args = append(args, "--timeout", formatSandboxDuration(timeout))
	}

	stdout, stderr, err := c.runner.Run(ctx, c.cfg.Binary, args, nil)
	if err != nil {
		return Sandbox{}, normalizeStageError("create", err, stdout, stderr)
	}

	sandboxID, err := parseSandboxID(append(stdout, stderr...))
	if err != nil {
		return Sandbox{}, normalizeStageError("create", err, stdout, stderr)
	}

	return Sandbox{ID: sandboxID, Workdir: c.resolveWorkdir(in.Workdir)}, nil
}

func (c *CLIClient) ExecTask(ctx context.Context, sandboxID string, in ExecInput) (ExecResult, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return ExecResult{}, fmt.Errorf("exec sandbox: sandbox id is required")
	}
	if len(in.Command) == 0 {
		return ExecResult{}, fmt.Errorf("exec sandbox: command is required")
	}

	args := append([]string{}, c.cfg.Args...)
	args = append(args, "exec")

	token := firstNonEmpty(in.Token, c.cfg.Token)
	project := firstNonEmpty(in.Project, c.cfg.Project)
	team := firstNonEmpty(in.Team, c.cfg.Team)
	if token != "" {
		args = append(args, "--token", token)
	}
	if project != "" {
		args = append(args, "--project", project)
	}
	if team != "" {
		args = append(args, "--scope", team)
	}

	workdir := c.resolveWorkdir(in.Workdir)
	if workdir != "" {
		args = append(args, "--workdir", workdir)
	}
	if len(in.Env) > 0 {
		keys := make([]string, 0, len(in.Env))
		for key := range in.Env {
			if strings.TrimSpace(key) == "" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			args = append(args, "--env", key+"="+in.Env[key])
		}
	}
	args = append(args, sandboxID)
	args = append(args, in.Command...)

	stdout, stderr, err := c.runner.Run(ctx, c.cfg.Binary, args, nil)
	if err != nil {
		return ExecResult{}, normalizeStageError("exec", err, stdout, stderr)
	}

	return ExecResult{
		Output:  string(stdout),
		WorkDir: workdir,
	}, nil
}

func (c *CLIClient) CreateSnapshot(ctx context.Context, sandboxID string, in SnapshotInput) (Snapshot, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return Snapshot{}, fmt.Errorf("snapshot sandbox: sandbox id is required")
	}

	args := append([]string{}, c.cfg.Args...)
	args = append(args, "snapshot")
	token := firstNonEmpty(in.Token, c.cfg.Token)
	project := firstNonEmpty(in.Project, c.cfg.Project)
	team := firstNonEmpty(in.Team, c.cfg.Team)
	if token != "" {
		args = append(args, "--token", token)
	}
	if project != "" {
		args = append(args, "--project", project)
	}
	if team != "" {
		args = append(args, "--scope", team)
	}
	args = append(args, sandboxID, "--stop")

	stdout, stderr, err := c.runner.Run(ctx, c.cfg.Binary, args, nil)
	if err != nil {
		return Snapshot{}, normalizeStageError("snapshot", err, stdout, stderr)
	}

	snapshotID, err := parseSnapshotID(append(stdout, stderr...))
	if err != nil {
		return Snapshot{}, normalizeStageError("snapshot", err, stdout, stderr)
	}

	return Snapshot{ID: snapshotID}, nil
}

func (c *CLIClient) Destroy(ctx context.Context, sandboxID string, in DestroyInput) error {
	if strings.TrimSpace(sandboxID) == "" {
		return fmt.Errorf("stop sandbox: sandbox id is required")
	}

	args := append([]string{}, c.cfg.Args...)
	args = append(args, "stop")
	token := firstNonEmpty(in.Token, c.cfg.Token)
	project := firstNonEmpty(in.Project, c.cfg.Project)
	team := firstNonEmpty(in.Team, c.cfg.Team)
	if token != "" {
		args = append(args, "--token", token)
	}
	if project != "" {
		args = append(args, "--project", project)
	}
	if team != "" {
		args = append(args, "--scope", team)
	}
	args = append(args, sandboxID)

	stdout, stderr, err := c.runner.Run(ctx, c.cfg.Binary, args, nil)
	if err != nil {
		return normalizeStageError("stop", err, stdout, stderr)
	}
	return nil
}

func (c *CLIClient) resolveWorkdir(workdir string) string {
	if strings.TrimSpace(workdir) != "" {
		return strings.TrimSpace(workdir)
	}
	return c.cfg.Workdir
}

func (c *StubClient) CreateOrConnect(_ context.Context, in CreateSandboxInput) (Sandbox, error) {
	id := "sandbox_v1"
	if in.BaseSnapshotID != "" {
		id = "sandbox_resume"
	}
	workdir := in.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	return Sandbox{ID: id, Workdir: workdir}, nil
}

func (c *StubClient) ExecTask(_ context.Context, _ string, in ExecInput) (ExecResult, error) {
	output := "executed task"
	if len(in.Command) > 0 {
		output = fmt.Sprintf("executed task %s", strings.Join(in.Command, " "))
	}
	workdir := in.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	return ExecResult{
		Output:    output,
		WorkDir:   workdir,
		Branch:    "main",
		SessionID: "",
	}, nil
}

func (c *StubClient) CreateSnapshot(_ context.Context, _ string, _ SnapshotInput) (Snapshot, error) {
	return Snapshot{
		ID:        "snapshot_v1",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}, nil
}

func (c *StubClient) Destroy(context.Context, string, DestroyInput) error { return nil }

func normalizeStageError(stage string, err error, stdout, stderr []byte) error {
	if err == nil {
		err = fmt.Errorf("unknown %s failure", stage)
	}
	detail := strings.TrimSpace(string(stderr))
	if detail == "" {
		detail = strings.TrimSpace(string(stdout))
	}
	if detail == "" {
		return fmt.Errorf("%s sandbox: %w", stage, err)
	}
	return fmt.Errorf("%s sandbox: %w: %s", stage, err, detail)
}

func parseSandboxID(output []byte) (string, error) {
	return parseID(output, []string{
		`"sandbox_id"\s*:\s*"([^"]+)"`,
		`"id"\s*:\s*"([^"]+)"`,
		`\b(sb_[A-Za-z0-9_-]+)\b`,
	})
}

func parseSnapshotID(output []byte) (string, error) {
	return parseID(output, []string{
		`"snapshot_id"\s*:\s*"([^"]+)"`,
		`"snapshotId"\s*:\s*"([^"]+)"`,
		`\b(snap_[A-Za-z0-9_-]+)\b`,
	})
}

func parseID(output []byte, patterns []string) (string, error) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return "", fmt.Errorf("could not parse id from empty cli output")
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(trimmed); len(matches) > 1 {
			return matches[1], nil
		}
	}

	if id, err := parseJSONID(trimmed); err == nil && id != "" {
		return id, nil
	}

	return "", fmt.Errorf("could not parse id from cli output")
}

func parseJSONID(raw string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", err
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return "", fmt.Errorf("cli output is not a json object")
	}
	for _, key := range []string{"sandbox_id", "snapshot_id", "snapshotId", "id"} {
		if id := jsonString(obj[key]); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("id not found in json output")
}

func jsonString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func formatSandboxDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	return d.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
