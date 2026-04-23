package provider

import (
	"context"
	"time"
)

// Task represents the daemon claim payload used by cloudrunner providers.
type Task struct {
	ID                    string         `json:"id"`
	AgentID               string         `json:"agent_id"`
	RuntimeID             string         `json:"runtime_id"`
	IssueID               string         `json:"issue_id"`
	WorkspaceID           string         `json:"workspace_id"`
	RuntimeMetadata       map[string]any `json:"runtime_metadata,omitempty"`
	PriorSessionID        string         `json:"prior_session_id,omitempty"`
	PriorWorkDir          string         `json:"prior_work_dir,omitempty"`
	TriggerCommentID      string         `json:"trigger_comment_id,omitempty"`
	TriggerCommentContent string         `json:"trigger_comment_content,omitempty"`
	ChatSessionID         string         `json:"chat_session_id,omitempty"`
	ChatMessage           string         `json:"chat_message,omitempty"`
	Repos                 []RepoData     `json:"repos,omitempty"`
	Agent                 *AgentData     `json:"agent,omitempty"`
	ResumeSnapshotID      string         `json:"resume_snapshot_id,omitempty"`
	ResumeSource          string         `json:"resume_source,omitempty"`
}

type RepoData struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type AgentData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Instructions string            `json:"instructions"`
	CustomEnv    map[string]string `json:"custom_env,omitempty"`
	CustomArgs   []string          `json:"custom_args,omitempty"`
	Model        string            `json:"model,omitempty"`
}

type Result struct {
	Output            string
	SessionID         string
	WorkDir           string
	Branch            string
	SandboxID         string
	SnapshotID        string
	SnapshotExpiresAt time.Time
}

// Provider executes claimed tasks. Implementations can map tasks into a
// sandbox-specific runtime similar to daemon execution providers.
type Provider interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	ExecuteTask(ctx context.Context, task Task) (Result, error)
}
