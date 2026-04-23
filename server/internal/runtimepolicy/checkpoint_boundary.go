package runtimepolicy

import "strings"

// IsSafeCheckpointBoundary returns true when a progress summary indicates a
// natural pause point where the task can safely persist a checkpoint.
func IsSafeCheckpointBoundary(summary string) bool {
	s := strings.ToLower(strings.TrimSpace(summary))
	if s == "" {
		return false
	}
	return strings.Contains(s, "command") ||
		strings.Contains(s, "git") ||
		strings.Contains(s, "pr opened") ||
		strings.Contains(s, "pr sync") ||
		strings.Contains(s, "pr-sync") ||
		strings.Contains(s, "pr update") ||
		strings.Contains(s, "checkpoint") ||
		strings.Contains(s, "save point")
}
