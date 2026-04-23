package runtimepolicy

func ThresholdState(spendCents, capCents int64) BudgetState {
	if capCents <= 0 {
		return BudgetStateOK
	}
	if spendCents >= capCents {
		return BudgetStateBlocked
	}
	if spendCents*100 >= capCents*90 {
		return BudgetStateWarn90
	}
	if spendCents*100 >= capCents*80 {
		return BudgetStateWarn80
	}
	return BudgetStateOK
}

func ShouldPauseAtCheckpoint(state BudgetState, billable bool) bool {
	return billable && state == BudgetStateBlocked
}
