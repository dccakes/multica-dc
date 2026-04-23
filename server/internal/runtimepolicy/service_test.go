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
	if !CanManageBudgets("admin") {
		t.Fatal("admin must manage budgets")
	}
	if CanManageBudgets("owner") {
		t.Fatal("owner must not manage budgets")
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
	if !CanOverrideAgentAssumption(true) {
		t.Fatal("permissioned human must override agent assumptions")
	}
	if CanOverrideAgentAssumption(false) {
		t.Fatal("unpermissioned human must not override agent assumptions")
	}
}

func TestServicePermissionFacade(t *testing.T) {
	svc := NewService()
	if svc.CanManageBudgets("admin") != true {
		t.Fatal("service must expose admin budget management")
	}
	if svc.CanOverrideAgentAssumption(true) != true {
		t.Fatal("service must expose permissioned override helper")
	}
}

func TestResolveIssueOwner(t *testing.T) {
	if got, ok := ResolveIssueOwner("member", "member-1", "member", "creator-1"); !ok || got != "member-1" {
		t.Fatalf("ResolveIssueOwner(member assignee) = %q, %v; want member-1, true", got, ok)
	}
	if got, ok := ResolveIssueOwner("agent", "agent-1", "member", "creator-1"); !ok || got != "creator-1" {
		t.Fatalf("ResolveIssueOwner(agent assignee) = %q, %v; want creator-1, true", got, ok)
	}
	if got, ok := ResolveIssueOwner("", "", "member", "creator-1"); !ok || got != "creator-1" {
		t.Fatalf("ResolveIssueOwner(creator fallback) = %q, %v; want creator-1, true", got, ok)
	}
	if got, ok := ResolveIssueOwner("agent", "agent-1", "agent", "creator-1"); ok || got != "" {
		t.Fatalf("ResolveIssueOwner(agent/agent) = %q, %v; want empty, false", got, ok)
	}
}

func TestLifecycleIssueStatusHelpers(t *testing.T) {
	if got := IssueStatusReadyForReview(); got != "in_review" {
		t.Fatalf("IssueStatusReadyForReview() = %q, want in_review", got)
	}
	if got := IssueStatusNeedsHumanIntervention(); got != "blocked" {
		t.Fatalf("IssueStatusNeedsHumanIntervention() = %q, want blocked", got)
	}
	if !CanDelegateToAgent("member") {
		t.Fatal("member must be allowed to delegate to agent")
	}
	if CanDelegateToAgent("agent") {
		t.Fatal("agent must not be allowed to delegate to agent")
	}
}

func TestInterventionActionTargetStatus(t *testing.T) {
	cases := map[InterventionAction]string{
		InterventionActionResumeSandbox:  "in_progress",
		InterventionActionResumeSnapshot: "in_progress",
		InterventionActionHandoffLocal:   "in_progress",
		InterventionActionArchive:        "cancelled",
		InterventionActionForceClose:     "cancelled",
	}
	for action, want := range cases {
		if got, ok := InterventionActionTargetStatus(action); !ok || got != want {
			t.Fatalf("InterventionActionTargetStatus(%q) = %q, %v; want %q, true", action, got, ok, want)
		}
	}
	if got, ok := InterventionActionTargetStatus("unknown"); ok || got != "" {
		t.Fatalf("InterventionActionTargetStatus(unknown) = %q, %v; want empty, false", got, ok)
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
