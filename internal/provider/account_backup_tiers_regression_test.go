package provider

import (
	"testing"
	"time"
)

// Uses only the original classification API so the same assertions run at base.
func TestBackupUsageDefaultRegression(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		used   float64
		lead   time.Duration
		backup bool
	}{
		{"low usage six days promotes", 10, 6 * 24 * time.Hour, false},
		{"high usage near reset stays backup", 85, 20 * time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Provider{BackupNoFiveHour: true}
			a := Allowance{{Used: tc.used, Span: 7 * 24 * time.Hour, Resets: now.Add(tc.lead)}}
			if got := p.IsBackupAccount("me", "gpt", a, true, now); got != tc.backup {
				t.Fatalf("backup=%v, want %v for %.0f%% used with %s until reset", got, tc.backup, tc.used, tc.lead)
			}
		})
	}
}
