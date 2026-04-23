package runtimepolicy

import (
	"math"
	"strings"
	"time"
)

const (
	proSandboxActiveCPUUSDPerHour = 0.128
	proSandboxMemoryUSDPerGBHour  = 0.0212
	sandboxCreationUSDPerSandbox  = 0.60 / 1_000_000
)

type modelPricing struct {
	input      float64
	output     float64
	cacheRead  float64
	cacheWrite float64
}

var claudeModelPricing = map[string]modelPricing{
	"claude-haiku-4-5":  {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25},
	"claude-sonnet-4-5": {input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75},
	"claude-sonnet-4-6": {input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75},
	"claude-opus-4-5":   {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
	"claude-opus-4-6":   {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25},
}

// EstimateSandboxCostCents estimates Vercel sandbox spend using the simple Pro
// pricing assumptions we agreed on for internal planning.
func EstimateSandboxCostCents(runtimeType RuntimeType, duration time.Duration, vCPUs int, memoryGB float64) int64 {
	return usdToCents(sandboxCostUSD(runtimeType, duration, vCPUs, memoryGB))
}

// EstimateSandboxCostCentsForProfile is the same estimator with a more explicit
// name for callers that want to emphasize the runtime profile inputs.
func EstimateSandboxCostCentsForProfile(runtimeType RuntimeType, duration time.Duration, vCPUs int, memoryGB float64) int64 {
	return EstimateSandboxCostCents(runtimeType, duration, vCPUs, memoryGB)
}

// EstimateTokenCostCents estimates model/API spend from token counts for a
// single model. Unknown models return zero instead of failing.
func EstimateTokenCostCents(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64) int64 {
	pricing, ok := lookupClaudePricing(model)
	if !ok {
		return 0
	}

	return usdToCents(tokenCostUSD(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, pricing))
}

// EstimateCombinedCostCents returns the total estimated spend for one task by
// summing sandbox and model/API costs.
func EstimateCombinedCostCents(runtimeType RuntimeType, duration time.Duration, vCPUs int, memoryGB float64, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64) int64 {
	return usdToCents(
		sandboxCostUSD(runtimeType, duration, vCPUs, memoryGB) +
			tokenCostUSDForModel(model, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens),
	)
}

func sandboxCostUSD(runtimeType RuntimeType, duration time.Duration, vCPUs int, memoryGB float64) float64 {
	if runtimeType != RuntimeTypeVercel {
		return 0
	}
	if duration <= 0 || vCPUs <= 0 || memoryGB <= 0 {
		return 0
	}

	hours := duration.Hours()
	return hours*float64(vCPUs)*proSandboxActiveCPUUSDPerHour +
		hours*memoryGB*proSandboxMemoryUSDPerGBHour +
		sandboxCreationUSDPerSandbox
}

func tokenCostUSDForModel(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64) float64 {
	pricing, ok := lookupClaudePricing(model)
	if !ok {
		return 0
	}
	return tokenCostUSD(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, pricing)
}

func tokenCostUSD(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64, pricing modelPricing) float64 {
	if inputTokens < 0 || outputTokens < 0 || cacheReadTokens < 0 || cacheWriteTokens < 0 {
		return 0
	}

	tokenUSD := float64(inputTokens)*pricing.input +
		float64(outputTokens)*pricing.output +
		float64(cacheReadTokens)*pricing.cacheRead +
		float64(cacheWriteTokens)*pricing.cacheWrite
	return tokenUSD / 1_000_000
}

func lookupClaudePricing(model string) (modelPricing, bool) {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return modelPricing{}, false
	}

	for key, pricing := range claudeModelPricing {
		if normalized == key || strings.HasPrefix(normalized, key) {
			return pricing, true
		}
	}

	return modelPricing{}, false
}

func usdToCents(amountUSD float64) int64 {
	if amountUSD <= 0 {
		return 0
	}
	return int64(math.Round(amountUSD * 100))
}
