package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cloudrunner/provider"
)

type ProviderConfig struct {
	Token          string
	Project        string
	Team           string
	BaseSnapshotID string
	Runtime        string
	Timeout        time.Duration
	Workdir        string
	Command        []string
	Env            map[string]string
}

type ProviderOption func(*Provider)

type Provider struct {
	client Client
	logger *slog.Logger
	cfg    ProviderConfig
}

func WithProviderConfig(cfg ProviderConfig) ProviderOption {
	return func(p *Provider) {
		p.cfg = normalizeProviderConfig(cfg)
	}
}

func NewProvider(client Client, logger *slog.Logger, opts ...ProviderOption) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	p := &Provider{
		client: client,
		logger: logger.With("provider", "vercel_sandbox"),
		cfg: normalizeProviderConfig(ProviderConfig{
			Runtime: "node24",
			Workdir: "/workspace",
			Command: []string{"codex"},
		}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	p.cfg = normalizeProviderConfig(p.cfg)
	return p
}

func normalizeProviderConfig(cfg ProviderConfig) ProviderConfig {
	if strings.TrimSpace(cfg.Runtime) == "" {
		cfg.Runtime = "node24"
	}
	if strings.TrimSpace(cfg.Workdir) == "" {
		cfg.Workdir = "/workspace"
	}
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"codex"}
	}
	return cfg
}

func (p *Provider) Start(context.Context) error {
	if p.client == nil {
		return fmt.Errorf("vercel provider client is required")
	}
	return nil
}

func (p *Provider) Stop(context.Context) error { return nil }

func (p *Provider) ExecuteTask(ctx context.Context, task provider.Task) (provider.Result, error) {
	creds := p.resolveRuntimeCredentials(task)
	createInput := CreateSandboxInput{
		Token:   creds.Token,
		Project: creds.Project,
		Team:    creds.Team,
		Runtime: p.cfg.Runtime,
		Timeout: p.cfg.Timeout,
		Workdir: p.cfg.Workdir,
	}
	if task.ResumeSnapshotID != "" {
		createInput.BaseSnapshotID = task.ResumeSnapshotID
	} else {
		createInput.BaseSnapshotID = firstNonEmpty(creds.BaseSnapshotID, p.cfg.BaseSnapshotID)
	}

	sbx, err := p.client.CreateOrConnect(ctx, createInput)
	if err != nil {
		return provider.Result{}, fmt.Errorf("create sandbox: %w", err)
	}
	defer func() {
		if err := p.client.Destroy(context.Background(), sbx.ID, DestroyInput{
			Token:   creds.Token,
			Project: creds.Project,
			Team:    creds.Team,
		}); err != nil {
			p.logger.Warn("destroy sandbox failed", "sandbox_id", sbx.ID, "error", err)
		}
	}()

	execInput := ExecInput{
		Token:   creds.Token,
		Project: creds.Project,
		Team:    creds.Team,
		Workdir: firstNonEmpty(task.PriorWorkDir, sbx.Workdir, p.cfg.Workdir),
		Command: append([]string{}, p.cfg.Command...),
		Env:     p.runtimeEnv(task),
	}
	if task.Agent != nil && len(task.Agent.CustomArgs) > 0 {
		execInput.Command = append(execInput.Command, task.Agent.CustomArgs...)
	}
	if task.Agent != nil && len(task.Agent.CustomEnv) > 0 {
		if execInput.Env == nil {
			execInput.Env = map[string]string{}
		}
		for key, value := range task.Agent.CustomEnv {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			if strings.HasPrefix(key, "MULTICA_") {
				continue
			}
			execInput.Env[key] = value
		}
	}

	execRes, err := p.client.ExecTask(ctx, sbx.ID, execInput)
	if err != nil {
		return provider.Result{}, fmt.Errorf("exec task: %w", err)
	}

	result := provider.Result{
		Output:    execRes.Output,
		WorkDir:   firstNonEmpty(execRes.WorkDir, execInput.Workdir, sbx.Workdir),
		Branch:    execRes.Branch,
		SessionID: execRes.SessionID,
		SandboxID: sbx.ID,
	}

	snapshot, err := p.client.CreateSnapshot(ctx, sbx.ID, SnapshotInput{
		Token:   creds.Token,
		Project: creds.Project,
		Team:    creds.Team,
	})
	if err != nil {
		// Snapshot persistence is best effort in v1.
		p.logger.Warn("create snapshot failed", "sandbox_id", sbx.ID, "error", err)
		return result, nil
	}
	result.SnapshotID = snapshot.ID
	result.SnapshotExpiresAt = snapshot.ExpiresAt
	return result, nil
}

func (p *Provider) runtimeEnv(task provider.Task) map[string]string {
	env := map[string]string{
		"MULTICA_TASK_ID":            task.ID,
		"MULTICA_WORKSPACE_ID":       task.WorkspaceID,
		"MULTICA_AGENT_ID":           task.AgentID,
		"MULTICA_RUNTIME_ID":         task.RuntimeID,
		"MULTICA_ISSUE_ID":           task.IssueID,
		"MULTICA_CHAT_SESSION_ID":    task.ChatSessionID,
		"MULTICA_RESUME_SNAPSHOT_ID": task.ResumeSnapshotID,
		"MULTICA_RESUME_SOURCE":      task.ResumeSource,
	}
	if task.PriorSessionID != "" {
		env["MULTICA_PRIOR_SESSION_ID"] = task.PriorSessionID
	}
	if task.PriorWorkDir != "" {
		env["MULTICA_PRIOR_WORKDIR"] = task.PriorWorkDir
	}
	if task.TriggerCommentID != "" {
		env["MULTICA_TRIGGER_COMMENT_ID"] = task.TriggerCommentID
	}
	if task.TriggerCommentContent != "" {
		env["MULTICA_TRIGGER_COMMENT_CONTENT"] = task.TriggerCommentContent
	}
	if task.ChatMessage != "" {
		env["MULTICA_CHAT_MESSAGE"] = task.ChatMessage
	}
	if len(task.Context) > 0 {
		env["MULTICA_TASK_CONTEXT"] = string(task.Context)
	}
	if len(task.RuntimeMetadata) > 0 {
		if raw, err := json.Marshal(task.RuntimeMetadata); err == nil {
			env["MULTICA_RUNTIME_METADATA"] = string(raw)
		}
	}
	if task.Agent != nil {
		if task.Agent.Name != "" {
			env["MULTICA_AGENT_NAME"] = task.Agent.Name
		}
		if task.Agent.Instructions != "" {
			env["MULTICA_AGENT_INSTRUCTIONS"] = task.Agent.Instructions
		}
		if task.Agent.Model != "" {
			env["MULTICA_AGENT_MODEL"] = task.Agent.Model
		}
	}
	for key, value := range p.cfg.Env {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if strings.HasPrefix(key, "MULTICA_") {
			continue
		}
		env[key] = value
	}
	return env
}

type runtimeCredentials struct {
	Token          string
	Project        string
	Team           string
	BaseSnapshotID string
}

func (p *Provider) resolveRuntimeCredentials(task provider.Task) runtimeCredentials {
	out := runtimeCredentials{
		Token:          p.cfg.Token,
		Project:        p.cfg.Project,
		Team:           p.cfg.Team,
		BaseSnapshotID: p.cfg.BaseSnapshotID,
	}
	if len(task.RuntimeMetadata) == 0 {
		return out
	}

	if v := metadataString(task.RuntimeMetadata, "token", "vercel_token", "api_token"); v != "" {
		out.Token = v
	}
	if v := metadataString(task.RuntimeMetadata, "project_id", "project"); v != "" {
		out.Project = v
	}
	if v := metadataString(task.RuntimeMetadata, "team_id", "team", "scope"); v != "" {
		out.Team = v
	}
	if v := metadataString(task.RuntimeMetadata, "base_snapshot_id", "snapshot_id"); v != "" {
		out.BaseSnapshotID = v
	}
	return out
}

func metadataString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := values[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
