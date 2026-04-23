package service

import "testing"

func TestNextIssueStatusAfterTaskCompletion(t *testing.T) {
	if got := nextIssueStatusAfterTaskCompletion("in_progress"); got != "in_review" {
		t.Fatalf("nextIssueStatusAfterTaskCompletion(in_progress) = %q, want in_review", got)
	}
	if got := nextIssueStatusAfterTaskCompletion("done"); got != "" {
		t.Fatalf("nextIssueStatusAfterTaskCompletion(done) = %q, want empty", got)
	}
	if got := nextIssueStatusAfterTaskCompletion("cancelled"); got != "" {
		t.Fatalf("nextIssueStatusAfterTaskCompletion(cancelled) = %q, want empty", got)
	}
}

func TestNextIssueStatusAfterTaskFailure(t *testing.T) {
	if got := nextIssueStatusAfterTaskFailure("in_progress"); got != "blocked" {
		t.Fatalf("nextIssueStatusAfterTaskFailure(in_progress) = %q, want blocked", got)
	}
	if got := nextIssueStatusAfterTaskFailure("done"); got != "" {
		t.Fatalf("nextIssueStatusAfterTaskFailure(done) = %q, want empty", got)
	}
	if got := nextIssueStatusAfterTaskFailure("cancelled"); got != "" {
		t.Fatalf("nextIssueStatusAfterTaskFailure(cancelled) = %q, want empty", got)
	}
}
