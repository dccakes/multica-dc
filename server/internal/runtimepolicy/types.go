package runtimepolicy

import "strings"

type RuntimeType string

const (
	RuntimeTypeLocal   RuntimeType = "local"
	RuntimeTypeVercel  RuntimeType = "vercel"
	RuntimeTypeRemote  RuntimeType = "remote"
	RuntimeTypeCloud   RuntimeType = "cloud"
	RuntimeTypeUnknown RuntimeType = "unknown"
)

type BudgetState string

const (
	BudgetStateOK      BudgetState = "ok"
	BudgetStateWarn80  BudgetState = "warn_80"
	BudgetStateWarn90  BudgetState = "warn_90"
	BudgetStateBlocked BudgetState = "blocked_100"
)

type InterventionAction string

const (
	InterventionActionResumeSandbox  InterventionAction = "resume_from_sandbox"
	InterventionActionResumeSnapshot InterventionAction = "resume_from_snapshot"
	InterventionActionHandoffLocal   InterventionAction = "handoff_to_local"
	InterventionActionArchive        InterventionAction = "archive"
	InterventionActionForceClose     InterventionAction = "force_close"
)

type CompletionState string

const (
	CompletionStateReadyForReview CompletionState = "ready_for_review"
	CompletionStateNeedsHuman     CompletionState = "needs_human_intervention"
	CompletionStateDone           CompletionState = "done"
)

func ClassifyRuntimeType(raw string) RuntimeType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RuntimeTypeLocal):
		return RuntimeTypeLocal
	case string(RuntimeTypeRemote), string(RuntimeTypeCloud), string(RuntimeTypeVercel):
		return RuntimeTypeVercel
	default:
		return RuntimeTypeUnknown
	}
}

func IsBillableRuntime(raw string) bool {
	return ClassifyRuntimeType(raw) == RuntimeTypeVercel
}
