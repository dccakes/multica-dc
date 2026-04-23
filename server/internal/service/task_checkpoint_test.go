package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestReportProgress_EmitsCheckpointEventOnSafeBoundary(t *testing.T) {
	bus := events.New()
	svc := &TaskService{Bus: bus}

	checkpoints := make([]protocol.TaskCheckpointPayload, 0, 1)
	bus.Subscribe(protocol.EventTaskCheckpoint, func(e events.Event) {
		payload, ok := e.Payload.(protocol.TaskCheckpointPayload)
		if !ok {
			t.Fatalf("checkpoint payload type = %T, want protocol.TaskCheckpointPayload", e.Payload)
		}
		checkpoints = append(checkpoints, payload)
	})

	svc.ReportProgress(context.Background(), "task-1", "workspace-1", "PR opened", 2, 4)

	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint events = %d, want 1", len(checkpoints))
	}
	if checkpoints[0].Reason != "safe_checkpoint_boundary" {
		t.Fatalf("checkpoint reason = %q, want safe_checkpoint_boundary", checkpoints[0].Reason)
	}
	if checkpoints[0].Summary != "PR opened" {
		t.Fatalf("checkpoint summary = %q, want PR opened", checkpoints[0].Summary)
	}
}
