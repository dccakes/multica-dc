package runtimepolicy

func CanManageBudgets(role string) bool {
	return role == "admin"
}

func CanCompleteIssue(isIssueOwner bool) bool {
	return isIssueOwner
}

func CanOverrideAgentAssumption(hasPermission bool) bool {
	return hasPermission
}
