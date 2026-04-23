package runtimepolicy

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) ClassifyRuntimeType(raw string) RuntimeType { return ClassifyRuntimeType(raw) }

func (s *Service) IsBillableRuntime(raw string) bool { return IsBillableRuntime(raw) }

func (s *Service) ThresholdState(spendCents, capCents int64) BudgetState {
	return ThresholdState(spendCents, capCents)
}

func (s *Service) ShouldPauseAtCheckpoint(state BudgetState, billable bool) bool {
	return ShouldPauseAtCheckpoint(state, billable)
}

func (s *Service) CanManageBudgets(role string) bool { return CanManageBudgets(role) }

func (s *Service) CanCompleteIssue(isIssueOwner bool) bool { return CanCompleteIssue(isIssueOwner) }

func (s *Service) ValidateCheckpoint(cp Checkpoint) error { return ValidateCheckpoint(cp) }
