package runtimepolicy

func CanManageBudgets(role string) bool {
	return role == "admin"
}

func CanDelegateToAgent(actorType string) bool {
	return actorType == "member"
}

func CanCompleteIssue(isIssueOwner bool) bool {
	return isIssueOwner
}

func CanOverrideAgentAssumption(hasPermission bool) bool {
	return hasPermission
}

func ResolveIssueOwner(assigneeType, assigneeID, creatorType, creatorID string) (string, bool) {
	if assigneeType == "member" && assigneeID != "" {
		return assigneeID, true
	}
	if creatorType == "member" && creatorID != "" {
		return creatorID, true
	}
	return "", false
}

func IssueStatusReadyForReview() string {
	return "in_review"
}

func IssueStatusNeedsHumanIntervention() string {
	return "blocked"
}

func InterventionActionTargetStatus(action InterventionAction) (string, bool) {
	switch action {
	case InterventionActionResumeSandbox, InterventionActionResumeSnapshot, InterventionActionHandoffLocal:
		return "in_progress", true
	case InterventionActionArchive, InterventionActionForceClose:
		return "cancelled", true
	default:
		return "", false
	}
}
