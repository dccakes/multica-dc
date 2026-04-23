package service

import "testing"

func TestResolveIssueEstimate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		title     string
		body      string
		wantHours float64
	}{
		{
			name:      "test writing",
			title:     "Write tests for sandbox resume behavior",
			body:      "Add assertions for the resume flow",
			wantHours: 8,
		},
		{
			name:      "bug fix",
			title:     "Fix resume crash",
			body:      "Add regression coverage for the crash",
			wantHours: 4,
		},
		{
			name:      "architecture",
			title:     "Design the sandbox runtime architecture",
			body:      "Write an RFC and plan the integration",
			wantHours: 16,
		},
		{
			name:      "feature default",
			title:     "Implement issue estimate persistence",
			body:      "Add the minimal backend support",
			wantHours: 40,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ResolveIssueEstimate(tc.title, tc.body); got != tc.wantHours {
				t.Fatalf("expected %v hours, got %v", tc.wantHours, got)
			}
		})
	}
}
