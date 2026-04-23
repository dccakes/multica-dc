package runtimepolicy

import "testing"

func TestIsSafeCheckpointBoundary(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		want    bool
	}{
		{name: "command step", summary: "Command step completed", want: true},
		{name: "git step", summary: "Git step finished", want: true},
		{name: "pr opened", summary: "PR opened", want: true},
		{name: "pr updated", summary: "PR updated", want: true},
		{name: "checkpoint", summary: "Checkpoint persisted", want: true},
		{name: "save point", summary: "Save point reached", want: true},
		{name: "non-boundary", summary: "Working on implementation", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsSafeCheckpointBoundary(tc.summary); got != tc.want {
				t.Fatalf("IsSafeCheckpointBoundary(%q) = %v, want %v", tc.summary, got, tc.want)
			}
		})
	}
}
