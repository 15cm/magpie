package provider

import (
	"maps"
	"os"
	"testing"
	"time"
)

func TestAccountBackupClassification(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	week := func(used float64, hours int) Limit {
		return Limit{Used: used, Span: 7 * 24 * time.Hour, Resets: now.Add(time.Duration(hours) * time.Hour)}
	}
	short := Limit{Used: 25, Span: 5 * time.Hour, Resets: now.Add(3 * time.Hour)}
	wallet := week(20, 20)
	balance := 100.0
	wallet.Balance = &balance
	partial := week(20, 20)
	partial.partial = true
	other := week(20, 20)
	other.Model = "opus"
	for _, tc := range []struct {
		name, mode  string
		auto, known bool
		a           Allowance
		want        bool
	}{
		{"default", "", false, true, Allowance{week(0, 72)}, false},
		{"fresh premium", "", true, true, Allowance{week(0, 72)}, true},
		{"standard", "", true, true, Allowance{short, week(20, 120)}, false},
		{"unknown auto", "", true, false, nil, false},
		{"unknown manual", "backup", false, false, nil, true},
		{"manual standard", "backup", false, true, Allowance{short, week(20, 120)}, true},
		{"primary override", "primary", true, true, Allowance{week(0, 72)}, false},
		{"pace promotion", "", true, true, Allowance{week(20, 36)}, false},
		{"time promotion", "backup", false, true, Allowance{week(70, 20)}, false},
		{"spent budget", "backup", false, true, Allowance{week(98, 20)}, true},
		{"spent short", "backup", false, true, Allowance{{Used: 98, Span: 5 * time.Hour, Resets: now.Add(time.Hour)}, week(20, 20)}, true},
		{"unstarted", "", true, true, Allowance{{Span: 7 * 24 * time.Hour}}, true},
		{"past reset", "backup", false, true, Allowance{week(20, -1)}, true},
		{"short alone", "backup", false, true, Allowance{short}, true},
		{"wallet", "backup", false, true, Allowance{wallet}, true},
		{"partial", "backup", false, true, Allowance{partial}, true},
		{"partial auto", "", true, true, Allowance{partial}, false},
		{"wallet auto", "", true, true, Allowance{wallet}, false},
		{"other model does not promote", "backup", false, true, Allowance{week(20, 120), other}, true},
		{"other model short ignored", "", true, true, Allowance{week(20, 120), {Span: 5 * time.Hour, Model: "opus"}}, true},
		{"matching model short", "", true, true, Allowance{week(20, 120), {Span: 5 * time.Hour, Model: "sonnet"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Provider{BackupNoFiveHour: tc.auto, AccountBackup: map[string]string{"me": tc.mode}}
			if got := p.IsBackupAccount("ME", "SONNET", tc.a, tc.known, now); got != tc.want {
				t.Fatalf("backup=%v, want %v", got, tc.want)
			}
			if p.IsBackupAccount("", "SONNET", tc.a, tc.known, now) {
				t.Fatal("empty account is backup")
			}
		})
	}
}

func TestBackupPromotedBoundaries(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		used, pace float64
		lead       time.Duration
		want       bool
	}{
		{97.99, 2, 48 * time.Hour, true}, {98, 2, 48 * time.Hour, false},
		{20, 1.99, 48 * time.Hour, false}, {70, 1, 24 * time.Hour, true},
		{70, 1, 24*time.Hour + time.Nanosecond, false}, {20, 3, 0, false}, {20, 3, -time.Hour, false},
	} {
		if got := BackupPromoted(tc.used, tc.pace, now.Add(tc.lead), now); got != tc.want {
			t.Errorf("%+v: %v", tc, got)
		}
	}
	if BackupPromoted(0, 3, time.Time{}, now) {
		t.Fatal("unknown reset promoted")
	}
}

func TestAccountBackupSettings(t *testing.T) {
	signIn(t)
	if err := SetBackupNoFiveHour("codex", true); err != nil {
		t.Fatal(err)
	}
	if err := SetAccountName("codex", "me@example.com", "work"); err != nil {
		t.Fatal(err)
	}
	if err := SetAccountBackup("codex", "work", "BACKUP"); err != nil {
		t.Fatal(err)
	}
	p, err := Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !p.BackupNoFiveHour || p.AccountBackupMode("ME@example.com") != "backup" {
		t.Fatalf("settings: %+v", p)
	}
	copy := p.clone()
	copy.AccountBackup["me@example.com"] = "primary"
	if p.AccountBackupMode("me@example.com") != "backup" {
		t.Fatal("clone shares override map")
	}
	before, _ := os.ReadFile(Path())
	for _, tc := range []struct{ ref, mode string }{{"me@example.com", "bogus"}, {"nobody", "backup"}} {
		if err := SetAccountBackup("codex", tc.ref, tc.mode); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	after, _ := os.ReadFile(Path())
	if string(before) != string(after) {
		t.Fatal("invalid setting wrote providers")
	}
	if err := SetAccountBackup("codex", "work", "auto"); err != nil {
		t.Fatal(err)
	}
	p, _ = Find("codex")
	if len(p.AccountBackup) != 0 {
		t.Fatalf("override left: %v", p.AccountBackup)
	}
	normalized := normalAccountBackup(map[string]string{" Me ": "on", "bad": "invalid", "auto": "auto", " ": "backup"})
	if !maps.Equal(normalized, map[string]string{"me": "backup"}) {
		t.Fatalf("normalized: %v", normalized)
	}
	if err := Save(Provider{ID: "keybackup", Name: "Key", Key: "k", Chat: "http://localhost"}); err != nil {
		t.Fatal(err)
	}
	if SetAccountBackup("keybackup", "k", "backup") == nil || SetBackupNoFiveHour("keybackup", true) == nil {
		t.Fatal("key accepted account backup")
	}
}

func TestParseAccountBackup(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"backup", "backup"}, {" ON ", "backup"}, {"yes", "backup"}, {"primary", "primary"}, {"first", "primary"}, {"", ""}, {"auto", ""}, {"default", ""}, {"off", ""}, {"no", ""}, {"-", ""}} {
		got, err := ParseAccountBackup(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("%q = %q, %v", tc.input, got, err)
		}
	}
	if _, err := ParseAccountBackup("sometimes"); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestAccountBackupRenamed(t *testing.T) {
	signIn(t)
	p, err := Find("copilot")
	if err != nil {
		t.Fatal(err)
	}
	p.BackupNoFiveHour = true
	p.AccountBackup = map[string]string{"old": "backup"}
	if err := Save(*p); err != nil {
		t.Fatal(err)
	}
	renameSettled([]savedLogin{{Agent: "copilot", User: "new", was: "old"}})
	p, err = Find("copilot")
	if err != nil {
		t.Fatal(err)
	}
	if !p.BackupNoFiveHour || p.AccountBackupMode("new") != "backup" || p.AccountBackupMode("old") != "" {
		t.Fatalf("renamed settings: %+v", p.AccountBackup)
	}
}
