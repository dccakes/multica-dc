package runtimepolicy

import (
	"testing"
	"time"
)

func TestRuntimeTypeClassification(t *testing.T) {
	if got := ClassifyRuntimeType("local"); got != RuntimeTypeLocal {
		t.Fatalf("ClassifyRuntimeType(local) = %q, want %q", got, RuntimeTypeLocal)
	}
	if got := ClassifyRuntimeType("cloud"); got != RuntimeTypeVercel {
		t.Fatalf("ClassifyRuntimeType(cloud) = %q, want %q", got, RuntimeTypeVercel)
	}
	if got := ClassifyRuntimeType("vercel"); got != RuntimeTypeVercel {
		t.Fatalf("ClassifyRuntimeType(vercel) = %q, want %q", got, RuntimeTypeVercel)
	}
	if !IsBillableRuntime("cloud") {
		t.Fatal("cloud runtime must be billable")
	}
	if IsBillableRuntime("local") {
		t.Fatal("local runtime must be non-billable")
	}
}

func TestThresholdTransitions(t *testing.T) {
	if got := ThresholdState(0, 0); got != BudgetStateOK {
		t.Fatalf("ThresholdState(0, 0) = %q, want ok", got)
	}
	if got := ThresholdState(79, 100); got != BudgetStateOK {
		t.Fatalf("ThresholdState(79, 100) = %q, want ok", got)
	}
	if got := ThresholdState(80, 100); got != BudgetStateWarn80 {
		t.Fatalf("ThresholdState(80, 100) = %q, want warn_80", got)
	}
	if got := ThresholdState(90, 100); got != BudgetStateWarn90 {
		t.Fatalf("ThresholdState(90, 100) = %q, want warn_90", got)
	}
	if got := ThresholdState(100, 100); got != BudgetStateBlocked {
		t.Fatalf("ThresholdState(100, 100) = %q, want blocked_100", got)
	}
	if !ShouldPauseAtCheckpoint(BudgetStateBlocked, true) {
		t.Fatal("blocked billable state must pause")
	}
	if ShouldPauseAtCheckpoint(BudgetStateBlocked, false) {
		t.Fatal("non-billable state must not pause")
	}
}

func TestPermissionHelpers(t *testing.T) {
	if !CanManageBudgets("owner") || !CanManageBudgets("admin") {
		t.Fatal("owner/admin must manage budgets")
	}
	if CanManageBudgets("member") {
		t.Fatal("member must not manage budgets")
	}
	if !CanCompleteIssue(true) {
		t.Fatal("issue owner must complete issue")
	}
	if CanCompleteIssue(false) {
		t.Fatal("non-owner must not complete issue")
	}
}

func TestCheckpointValidation(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	valid := Checkpoint{
		RunID:             "run-1",
		RuntimeID:         "rt-1",
		AgentID:           "agent-1",
		IssueID:           "issue-1",
		ExecutionState:    CompletionStateReadyForReview,
		SnapshotExpiresAt: &expiry,
	}
	if err := ValidateCheckpoint(valid); err != nil {
		t.Fatalf("ValidateCheckpoint(valid) error = %v", err)
	}

	invalid := valid
	invalid.RunID = ""
	if err := ValidateCheckpoint(invalid); err == nil {
		t.Fatal("ValidateCheckpoint should fail when run_id missing")
	}
}
