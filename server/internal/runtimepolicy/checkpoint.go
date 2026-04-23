package runtimepolicy

import (
	"fmt"
	"strings"
	"time"
)

type Checkpoint struct {
	RunID             string
	RuntimeID         string
	AgentID           string
	IssueID           string
	ChatSessionID     string
	ExecutionState    CompletionState
	SandboxID         string
	SnapshotID        string
	SnapshotExpiresAt *time.Time
	WorkDir           string
	Branch            string
	SessionID         string
	LastStep          string
	PRURL             string
	Billable          bool
	BudgetState       BudgetState
}

func ValidateCheckpoint(cp Checkpoint) error {
	if strings.TrimSpace(cp.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(cp.RuntimeID) == "" {
		return fmt.Errorf("runtime_id is required")
	}
	if strings.TrimSpace(cp.AgentID) == "" {
		return fmt.Errorf("agent_id is required")
	}
	if strings.TrimSpace(cp.IssueID) == "" && strings.TrimSpace(cp.ChatSessionID) == "" {
		return fmt.Errorf("issue_id or chat_session_id is required")
	}
	if strings.TrimSpace(string(cp.ExecutionState)) == "" {
		return fmt.Errorf("execution_state is required")
	}
	return nil
}
