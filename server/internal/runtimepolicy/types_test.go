package runtimepolicy

import "testing"

func TestRuntimePolicyEnumContracts(t *testing.T) {
	if RuntimeTypeLocal != "local" {
		t.Fatalf("RuntimeTypeLocal = %q, want local", RuntimeTypeLocal)
	}
	if RuntimeTypeRemote != "remote" {
		t.Fatalf("RuntimeTypeRemote = %q, want remote", RuntimeTypeRemote)
	}
	if RuntimeTypeVercel != "vercel" {
		t.Fatalf("RuntimeTypeVercel = %q, want vercel", RuntimeTypeVercel)
	}
	if BudgetStateOK != "ok" || BudgetStateWarn80 != "warn_80" || BudgetStateWarn90 != "warn_90" || BudgetStateBlocked != "blocked_100" {
		t.Fatalf("budget state constants drifted: %q %q %q %q", BudgetStateOK, BudgetStateWarn80, BudgetStateWarn90, BudgetStateBlocked)
	}
	if InterventionActionResumeSandbox != "resume_from_sandbox" ||
		InterventionActionResumeSnapshot != "resume_from_snapshot" ||
		InterventionActionHandoffLocal != "handoff_to_local" ||
		InterventionActionArchive != "archive" ||
		InterventionActionForceClose != "force_close" {
		t.Fatalf("intervention action constants drifted")
	}
	if CompletionStateReadyForReview != "ready_for_review" ||
		CompletionStateNeedsHuman != "needs_human_intervention" ||
		CompletionStateDone != "done" {
		t.Fatalf("completion state constants drifted")
	}
}

func TestRuntimeTypeClassificationContract(t *testing.T) {
	cases := map[string]RuntimeType{
		"local":    RuntimeTypeLocal,
		"remote":   RuntimeTypeVercel,
		"cloud":    RuntimeTypeVercel,
		"vercel":   RuntimeTypeVercel,
		" VERCEL ": RuntimeTypeVercel,
		"unknown":  RuntimeTypeUnknown,
	}

	for input, want := range cases {
		if got := ClassifyRuntimeType(input); got != want {
			t.Fatalf("ClassifyRuntimeType(%q) = %q, want %q", input, got, want)
		}
	}
}
