package runtimepolicy

// Compatibility aliases keep the in-flight service facade compiling while the
// policy package still exposes the original BudgetState/role primitives.
type BudgetBlockState = BudgetState

type Role = string
