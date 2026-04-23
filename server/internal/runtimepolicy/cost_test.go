package runtimepolicy

import (
	"testing"
	"time"
)

func TestEstimateSandboxCostCents(t *testing.T) {
	t.Parallel()

	t.Run("local runtime is free", func(t *testing.T) {
		t.Parallel()

		if got := EstimateSandboxCostCents(RuntimeTypeLocal, 30*time.Minute, 4, 8); got != 0 {
			t.Fatalf("expected local runtime cost 0, got %d", got)
		}
	})

	t.Run("vercel runtime uses pro pricing", func(t *testing.T) {
		t.Parallel()

		if got := EstimateSandboxCostCentsForProfile(RuntimeTypeVercel, 30*time.Minute, 4, 8); got != 34 {
			t.Fatalf("expected 34 cents, got %d", got)
		}
	})
}

func TestEstimateTokenCostCents(t *testing.T) {
	t.Parallel()

	t.Run("known claude model uses frontend pricing table", func(t *testing.T) {
		t.Parallel()

		if got := EstimateTokenCostCents("claude-sonnet-4-6", 1_000_000, 0, 0, 0); got != 300 {
			t.Fatalf("expected 300 cents, got %d", got)
		}
	})

	t.Run("unknown model falls back to zero", func(t *testing.T) {
		t.Parallel()

		if got := EstimateTokenCostCents("made-up-model", 1_000, 2_000, 500, 250); got != 0 {
			t.Fatalf("expected unknown model to cost 0, got %d", got)
		}
	})
}

func TestEstimateCombinedCostCents(t *testing.T) {
	t.Parallel()

	sandbox := EstimateSandboxCostCentsForProfile(RuntimeTypeVercel, 30*time.Minute, 4, 8)
	tokens := EstimateTokenCostCents("claude-sonnet-4-6", 1_000_000, 0, 0, 0)

	if got := EstimateCombinedCostCents(RuntimeTypeVercel, 30*time.Minute, 4, 8, "claude-sonnet-4-6", 1_000_000, 0, 0, 0); got != sandbox+tokens {
		t.Fatalf("expected combined cost %d, got %d", sandbox+tokens, got)
	}
}
