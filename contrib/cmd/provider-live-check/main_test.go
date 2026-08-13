package main

import "testing"

func TestOverallStatusDoesNotTreatSkippedEvidenceAsSuccess(t *testing.T) {
	tests := []struct {
		name      string
		attempted bool
		statuses  []providerStatus
		want      string
	}{
		{
			name: "no credentials",
			statuses: []providerStatus{
				{Provider: "stripe", Status: "skipped_no_credentials"},
			},
			want: "skipped_no_credentials",
		},
		{
			name:      "all attempted checks pass",
			attempted: true,
			statuses: []providerStatus{
				{Provider: "stripe", Status: "passed"},
				{Provider: "resend", Status: "passed"},
			},
			want: "passed",
		},
		{
			name:      "one failed check fails evidence",
			attempted: true,
			statuses: []providerStatus{
				{Provider: "stripe", Status: "failed"},
				{Provider: "resend", Status: "passed"},
			},
			want: "failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := overallStatus(test.statuses, test.attempted); got != test.want {
				t.Fatalf("overallStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
