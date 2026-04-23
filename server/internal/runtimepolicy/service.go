package runtimepolicy

// Service provides a small facade over the policy primitives so callers can
// depend on one package-level entrypoint as the policy layer grows.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) ClassifyRuntimeType(raw string) RuntimeType {
	return ClassifyRuntimeType(raw)
}

func (s *Service) IsBillableRuntime(raw string) bool {
	return IsBillableRuntime(raw)
}

func (s *Service) ThresholdState(spendCents, capCents int64) BudgetState {
	return ThresholdState(spendCents, capCents)
}

func (s *Service) ShouldPauseAtCheckpoint(state BudgetState, billable bool) bool {
	return ShouldPauseAtCheckpoint(state, billable)
}

func (s *Service) CanManageBudgets(role string) bool {
	return CanManageBudgets(role)
}

func (s *Service) CanDelegateToAgent(actorType string) bool {
	return CanDelegateToAgent(actorType)
}

func (s *Service) CanCompleteIssue(isIssueOwner bool) bool {
	return CanCompleteIssue(isIssueOwner)
}

func (s *Service) CanOverrideAgentAssumption(hasPermission bool) bool {
	return CanOverrideAgentAssumption(hasPermission)
}

func (s *Service) ResolveIssueOwner(assigneeType, assigneeID, creatorType, creatorID string) (string, bool) {
	return ResolveIssueOwner(assigneeType, assigneeID, creatorType, creatorID)
}

func (s *Service) IssueStatusReadyForReview() string {
	return IssueStatusReadyForReview()
}

func (s *Service) IssueStatusNeedsHumanIntervention() string {
	return IssueStatusNeedsHumanIntervention()
}

func (s *Service) InterventionActionTargetStatus(action InterventionAction) (string, bool) {
	return InterventionActionTargetStatus(action)
}

func (s *Service) ValidateCheckpoint(cp Checkpoint) error {
	return ValidateCheckpoint(cp)
}
