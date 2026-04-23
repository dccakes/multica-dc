package cloudrunner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/cloudrunner/provider"
)

// RunnerClient describes the cloudrunner calls needed by lifecycle loops.
type RunnerClient interface {
	SendHeartbeat(ctx context.Context, runtimeID string) error
	ClaimTask(ctx context.Context, runtimeID string) (*Task, error)
	StartTask(ctx context.Context, taskID string) error
	CompleteTask(ctx context.Context, taskID, output, branchName, sessionID, workDir string) error
	FailTask(ctx context.Context, taskID, errMsg, sessionID, workDir string) error
}

// Runner executes heartbeat and claim loops across configured runtimes.
type Runner struct {
	cfg      Config
	client   RunnerClient
	provider provider.Provider
	session  *SessionService
	logger   *slog.Logger
}

func NewRunner(cfg Config, client RunnerClient, p provider.Provider) *Runner {
	return &Runner{
		cfg:      cfg,
		client:   client,
		provider: p,
		logger:   slog.Default().With("component", "cloudrunner"),
	}
}

func (r *Runner) WithSessionService(session *SessionService) *Runner {
	r.session = session
	return r
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.cfg.Validate(); err != nil {
		return err
	}
	if r.client == nil {
		return fmt.Errorf("runner client is required")
	}
	if r.provider == nil {
		return fmt.Errorf("runner provider is required")
	}

	if err := r.provider.Start(ctx); err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.provider.Stop(stopCtx); err != nil {
			r.logger.Warn("provider stop failed", "error", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r.heartbeatLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		r.claimLoop(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (r *Runner) heartbeatLoop(ctx context.Context) {
	runtimeIDs := r.runtimeIDs()
	if len(runtimeIDs) == 0 {
		return
	}

	ticker := time.NewTicker(r.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, runtimeID := range runtimeIDs {
				if err := r.client.SendHeartbeat(ctx, runtimeID); err != nil {
					r.logger.Warn("heartbeat failed", "runtime_id", runtimeID, "error", err)
				}
			}
		}
	}
}

func (r *Runner) claimLoop(ctx context.Context) {
	runtimeIDs := r.runtimeIDs()
	if len(runtimeIDs) == 0 {
		return
	}

	ticker := time.NewTicker(r.cfg.ClaimInterval)
	defer ticker.Stop()
	nextRuntimeIdx := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtimeID := runtimeIDs[nextRuntimeIdx]
			nextRuntimeIdx = (nextRuntimeIdx + 1) % len(runtimeIDs)
			task, err := r.client.ClaimTask(ctx, runtimeID)
			if err != nil {
				r.logger.Warn("claim failed", "runtime_id", runtimeID, "error", err)
				continue
			}
			if task == nil {
				continue
			}
			if err := r.client.StartTask(ctx, task.ID); err != nil {
				r.logger.Warn("start failed", "task_id", task.ID, "error", err)
				continue
			}
			providerTask := provider.Task(*task)
			if r.session != nil && (providerTask.IssueID != "" || providerTask.ChatSessionID != "") {
				selection, err := r.session.ResolveSnapshot(ctx, ResolveSnapshotInput{
					RuntimeID:      providerTask.RuntimeID,
					AgentID:        providerTask.AgentID,
					IssueID:        providerTask.IssueID,
					ChatSessionID:  providerTask.ChatSessionID,
					BaseSnapshotID: "",
				})
				if err != nil {
					r.logger.Warn("snapshot continuity resolve failed", "task_id", task.ID, "error", err)
				} else {
					providerTask.ResumeSnapshotID = selection.SnapshotID
					providerTask.ResumeSource = selection.Source
					if providerTask.PriorWorkDir == "" {
						providerTask.PriorWorkDir = selection.LastWorkdir
					}
					if providerTask.PriorSessionID == "" {
						providerTask.PriorSessionID = selection.CodexSessionID
					}
				}
			}

			result, err := r.provider.ExecuteTask(ctx, providerTask)
			if err != nil {
				r.logger.Warn("task execution failed", "task_id", task.ID, "error", err)
				if reportErr := r.client.FailTask(ctx, task.ID, err.Error(), "", ""); reportErr != nil {
					r.logger.Warn("fail report failed", "task_id", task.ID, "error", reportErr)
				}
				continue
			}
			if err := r.client.CompleteTask(ctx, task.ID, result.Output, result.Branch, result.SessionID, result.WorkDir); err != nil {
				r.logger.Warn("complete report failed", "task_id", task.ID, "error", err)
			}
			if r.session != nil && result.SnapshotID != "" {
				if err := r.session.RecordSnapshot(ctx, RecordSnapshotInput{
					RuntimeID:      task.RuntimeID,
					AgentID:        task.AgentID,
					IssueID:        task.IssueID,
					ChatSessionID:  task.ChatSessionID,
					SandboxID:      result.SandboxID,
					SnapshotID:     result.SnapshotID,
					SnapshotExpiry: result.SnapshotExpiresAt,
					LastWorkdir:    result.WorkDir,
					LastBranch:     result.Branch,
					CodexSessionID: result.SessionID,
				}); err != nil {
					r.logger.Warn("snapshot continuity persist failed", "task_id", task.ID, "error", err)
				}
			}
		}
	}
}

func (r *Runner) runtimeIDs() []string {
	if len(r.cfg.RuntimeIDs) > 0 {
		return r.cfg.RuntimeIDs
	}
	if runtimeID := strings.TrimSpace(r.cfg.RuntimeID); runtimeID != "" {
		return []string{runtimeID}
	}
	return nil
}
