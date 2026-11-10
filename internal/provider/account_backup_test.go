package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"slices"
	"strings"
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
		{"fresh premium", "", true, true, Allowance{week(0, 72)}, false},
		{"standard", "", true, true, Allowance{short, week(20, 120)}, false},
		{"unknown auto", "", true, false, nil, false},
		{"unknown manual", "backup", false, false, nil, true},
		{"manual standard", "backup", false, true, Allowance{short, week(20, 120)}, false},
		{"primary override", "primary", true, true, Allowance{week(0, 72)}, false},
		{"low weekly usage promotion", "", true, true, Allowance{week(20, 36)}, false},
		{"last default tier", "backup", false, true, Allowance{week(70, 20)}, false},
		{"spent budget", "backup", false, true, Allowance{week(98, 20)}, true},
		{"spent short", "backup", false, true, Allowance{{Used: 98, Span: 5 * time.Hour, Resets: now.Add(time.Hour)}, week(20, 20)}, true},
		{"unstarted", "", true, true, Allowance{{Span: 7 * 24 * time.Hour}}, true},
		{"past reset", "backup", false, true, Allowance{week(20, -1)}, true},
		{"short alone", "backup", false, true, Allowance{short}, true},
		{"wallet", "backup", false, true, Allowance{wallet}, true},
		{"partial", "backup", false, true, Allowance{partial}, true},
		{"partial auto", "", true, true, Allowance{partial}, false},
		{"wallet auto", "", true, true, Allowance{wallet}, false},
		{"other model does not promote", "backup", false, true, Allowance{week(50, 120), other}, true},
		{"other model short ignored", "", true, true, Allowance{week(50, 120), {Span: 5 * time.Hour, Model: "opus"}}, true},
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
	p.BackupUsageTiers = &[]BackupUsageTier{{15.5, 6}}
	if err := Save(*p); err != nil {
		t.Fatal(err)
	}
	renameSettled([]savedLogin{{Agent: "copilot", User: "new", was: "old"}})
	p, err = Find("copilot")
	if err != nil {
		t.Fatal(err)
	}
	if p.BackupUsageTiers == nil || (*p.BackupUsageTiers)[0].MaxUsed != 15.5 {
		t.Fatal("account rename lost tier policy")
	}
	if !p.BackupNoFiveHour || p.AccountBackupMode("new") != "backup" || p.AccountBackupMode("old") != "" {
		t.Fatalf("renamed settings: %+v", p.AccountBackup)
	}
}

func TestBackupUsageTierBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	tiers := append(DefaultBackupUsageTiers(), BackupUsageTier{15.5, 2.5}, BackupUsageTier{0, 1.0 / 24})
	for _, tier := range tiers {
		lead := time.Duration(tier.MaxResetDays * float64(24*time.Hour))
		for _, tc := range []struct {
			name string
			used float64
			lead time.Duration
			want bool
		}{
			{"equal", tier.MaxUsed, lead, true},
			{"below usage", math.Nextafter(tier.MaxUsed, 0), lead, true},
			{"above usage", math.Nextafter(tier.MaxUsed, math.Inf(1)), lead, false},
			{"before time", tier.MaxUsed, lead - 1, true},
			{"after time", tier.MaxUsed, lead + 1, false},
			{"usage alone", 0, lead + 1, false},
			{"time alone", tier.MaxUsed + 1, time.Nanosecond, false},
		} {
			t.Run(fmt.Sprintf("%g:%g/%s", tier.MaxUsed, tier.MaxResetDays, tc.name), func(t *testing.T) {
				if got := BackupPromoted(tc.used, now.Add(tc.lead), now, []BackupUsageTier{tier}); got != tc.want {
					t.Fatalf("promoted=%v, want %v", got, tc.want)
				}
				p := Provider{AccountBackup: map[string]string{"me": "backup"}, BackupUsageTiers: &[]BackupUsageTier{tier}}
				a := Allowance{{Used: tc.used, Span: weekSpan, Resets: now.Add(tc.lead)}}
				if promoted := !p.IsBackupAccount("me", "", a, true, now); promoted != tc.want {
					t.Fatalf("classifier promoted=%v, want %v", promoted, tc.want)
				}
			})
		}
	}
	for _, policy := range [][]BackupUsageTier{DefaultBackupUsageTiers(), {{80, 1}, {60, 3}, {40, 5}, {20, 7}}} {
		if !BackupPromoted(55, now.Add(2*24*time.Hour), now, policy) {
			t.Fatal("OR or row order changed matching")
		}
	}
	for _, used := range []float64{-1, 98, 100, 101, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if BackupPromoted(used, now.Add(time.Hour), now, []BackupUsageTier{{100, 7}}) {
			t.Fatalf("invalid/spent %v promoted", used)
		}
	}
	for _, due := range []time.Time{{}, now, now.Add(-time.Nanosecond)} {
		if BackupPromoted(0, due, now, DefaultBackupUsageTiers()) {
			t.Fatal("nonfuture reset promoted")
		}
	}
}

func TestBackupUsageWeeklyWindows(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	week := func(u float64, d float64, model string) Limit {
		return Limit{Used: u, Span: weekSpan, Model: model, Resets: now.Add(time.Duration(d * float64(24*time.Hour)))}
	}
	balance := 10.0
	for _, tc := range []struct {
		name, model string
		a           Allowance
		promote     bool
	}{
		{"shared", "SONNET", Allowance{week(10, 6, "")}, true},
		{"shared and model different rows", "SONNET", Allowance{week(10, 6, ""), week(70, .8, "sonnet")}, true},
		{"shared fails", "sonnet", Allowance{week(85, .8, ""), week(10, 6, "sonnet")}, false},
		{"model fails", "sonnet", Allowance{week(10, 6, ""), week(85, .8, "sonnet")}, false},
		{"paired windows", "sonnet", Allowance{week(10, 8, ""), week(85, .8, "sonnet")}, false},
		{"another model ignored", "SONNET", Allowance{week(10, 6, ""), week(100, .8, "opus")}, true},
		{"model only", "sonnet", Allowance{week(10, 6, "sonnet")}, true},
		{"model-less isolation", "", Allowance{week(10, 6, "sonnet")}, false},
		{"model-less shared", "", Allowance{week(10, 6, ""), week(100, 1, "sonnet")}, true},
		{"missing", "sonnet", nil, false},
		{"short", "sonnet", Allowance{{Used: 0, Span: 5 * time.Hour, Resets: now.Add(time.Hour)}}, false},
		{"day", "sonnet", Allowance{{Used: 0, Span: 24 * time.Hour, Resets: now.Add(time.Hour)}}, false},
		{"month", "sonnet", Allowance{{Used: 0, Span: 30 * 24 * time.Hour, Resets: now.Add(time.Hour)}}, false},
		{"weekly exact span", "sonnet", Allowance{{Used: 0, Span: weekSpan + 1, Resets: now.Add(time.Hour)}}, false},
		{"expired alongside complete", "sonnet", Allowance{week(10, 6, ""), week(0, -1, "")}, false},
		{"now alongside complete", "sonnet", Allowance{week(10, 6, ""), week(0, 0, "")}, false},
		{"unknown reset alongside complete", "sonnet", Allowance{week(10, 6, ""), {Span: weekSpan}}, false},
		{"partial alongside complete", "sonnet", Allowance{week(10, 6, ""), {Span: weekSpan, partial: true, Resets: now.Add(time.Hour)}}, false},
		{"wallet alongside complete", "sonnet", Allowance{week(10, 6, ""), {Span: weekSpan, Balance: &balance, Resets: now.Add(time.Hour)}}, false},
		{"short spent guard", "sonnet", Allowance{week(10, 6, ""), {Span: time.Hour, Used: 98, Resets: now.Add(time.Hour)}}, false},
		{"nonweekly cannot grant", "sonnet", Allowance{week(85, 1, ""), {Span: time.Hour, Used: 0, Resets: now.Add(time.Hour)}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Provider{AccountBackup: map[string]string{"me": "backup"}}
			if got := !p.IsBackupAccount("me", tc.model, tc.a, true, now); got != tc.promote {
				t.Fatalf("promoted=%v, want %v", got, tc.promote)
			}
		})
	}
	for _, u := range []float64{-1, 101, math.NaN(), math.Inf(1), math.Inf(-1)} {
		p := Provider{BackupNoFiveHour: true}
		if !p.IsBackupAccount("me", "", Allowance{week(u, 1, "")}, true, now) {
			t.Fatalf("invalid weekly reading %v promoted", u)
		}
	}
}

func TestBackupUsagePolicyStates(t *testing.T) {
	now := time.Now()
	a := Allowance{{Span: weekSpan, Used: 10, Resets: now.Add(6 * 24 * time.Hour)}}
	for _, tc := range []struct {
		name   string
		policy *[]BackupUsageTier
		backup bool
	}{
		{"absent", nil, false}, {"empty", &[]BackupUsageTier{}, true}, {"nil slice", new([]BackupUsageTier), true},
		{"custom passes", &[]BackupUsageTier{{15.5, 6.5}}, false}, {"custom fails", &[]BackupUsageTier{{9, 7}}, true},
		{"invalid runtime policy", &[]BackupUsageTier{{101, 7}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Provider{BackupNoFiveHour: true, BackupUsageTiers: tc.policy}
			if got := p.IsBackupAccount("me", "", a, true, now); got != tc.backup {
				t.Fatalf("backup=%v want %v", got, tc.backup)
			}
			p.BackupNoFiveHour = false
			if p.IsBackupAccount("me", "", a, true, now) {
				t.Fatal("ordinary primary changed")
			}
			p.AccountBackup = map[string]string{"me": "primary"}
			p.BackupNoFiveHour = true
			if p.IsBackupAccount("me", "", a, true, now) {
				t.Fatal("primary override changed")
			}
		})
	}
	defaults := DefaultBackupUsageTiers()
	defaults[0].MaxUsed = 100
	if DefaultBackupUsageTiers()[0].MaxUsed != 20 {
		t.Fatal("defaults share state")
	}
}

func TestBackupUsageTiersJSON(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `[{"maxUsed":0,"maxResetDays":0.5}]`, `[{"maxUsed":101,"maxResetDays":8}]`} {
		var p Provider
		if err := json.Unmarshal([]byte(`{"backupUsageTiers":`+raw+`}`), &p); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if raw == "null" {
			if p.BackupUsageTiers != nil {
				t.Fatal("null not defaults")
			}
			continue
		}
		b, err := json.Marshal(p)
		if err != nil || !bytes.Contains(b, []byte(`"backupUsageTiers":`+raw)) {
			t.Fatalf("round trip %s: %s %v", raw, b, err)
		}
	}
	for _, raw := range []string{`[null]`, `[{}]`, `[{"maxUsed":0}]`, `[{"maxResetDays":1}]`, `[{"maxUsed":null,"maxResetDays":1}]`, `[{"maxUsed":"20","maxResetDays":1}]`, `[{"maxUsed":20,"maxResetDays":true}]`, `[{"maxUsed":20,"maxResetDays":1,"other":1}]`, `{}`, `[1]`, `[[20,1]]`} {
		var tiers *[]BackupUsageTier
		if json.Unmarshal([]byte(raw), &tiers) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	var row BackupUsageTier
	if row.UnmarshalJSON([]byte(`{"maxUsed":20,"maxResetDays":1} {}`)) == nil {
		t.Fatal("trailing content accepted")
	}
	for _, row := range []BackupUsageTier{{-1, 1}, {101, 1}, {math.NaN(), 1}, {math.Inf(1), 1}, {1, 0}, {1, -1}, {1, 7.1}, {1, math.NaN()}, {1, math.Inf(1)}, {1, 1e-20}} {
		if CheckBackupUsageTiers([]BackupUsageTier{row}) == nil {
			t.Errorf("accepted %+v", row)
		}
	}
	for _, text := range []string{"", "20", "20:", ":1", "20:1,", "20:1:2", "20%:1", "20:1d", "NaN:1", "20:Inf", "-1:1", "101:1", "20:0", "20:8", "off,", "--help"} {
		if _, err := ParseBackupUsageTiers(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
	for _, text := range []string{"default", "off", " 15.5 : 6 , 55 : 2.5 ", "0:0.0000000000001", "100:7"} {
		if _, err := ParseBackupUsageTiers(text); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}
}

func TestBackupUsageTiersSettings(t *testing.T) {
	signIn(t)
	for _, state := range []string{"default", "off", "15.5:6,55:2.5"} {
		tiers, err := ParseBackupUsageTiers(state)
		if err != nil {
			t.Fatal(err)
		}
		if err := SetBackupUsageTiers("codex", tiers); err != nil {
			t.Fatal(err)
		}
		p, err := Find("codex")
		if err != nil {
			t.Fatal(err)
		}
		if (p.BackupUsageTiers == nil) != (tiers == nil) {
			t.Fatal("default/off state lost")
		}
		if tiers != nil && !slices.Equal(*p.BackupUsageTiers, *tiers) {
			t.Fatalf("policy lost: %+v", p.BackupUsageTiers)
		}
		if tiers != nil && len(*tiers) > 0 {
			(*tiers)[0].MaxUsed = 99
			if (*p.BackupUsageTiers)[0].MaxUsed != 15.5 {
				t.Fatal("setter shares input")
			}
			q := p.clone()
			(*q.BackupUsageTiers)[0].MaxUsed = 98
			if (*p.BackupUsageTiers)[0].MaxUsed != 15.5 {
				t.Fatal("clone shares policy")
			}
			out := p.BackupPromotionTiers()
			out[0].MaxUsed = 97
			if p.BackupPromotionTiers()[0].MaxUsed != 15.5 {
				t.Fatal("accessor shares policy")
			}
		}
		p.Name = "unrelated save"
		if err := Save(*p); err != nil {
			t.Fatal(err)
		}
		ForgetAccounts()
		again, err := Find("codex")
		if err != nil {
			t.Fatal(err)
		}
		if (again.BackupUsageTiers == nil) != (p.BackupUsageTiers == nil) || !slices.Equal(again.BackupPromotionTiers(), p.BackupPromotionTiers()) {
			t.Fatal("reload/save lost policy")
		}
	}
	if err := SetBackupUsageTiers("codex", new([]BackupUsageTier)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(Path())
	if !bytes.Contains(b, []byte(`"backupUsageTiers": []`)) {
		t.Fatalf("nil slice not canonical empty: %s", b)
	}
	if err := Save(Provider{ID: "tier-key", Name: "key", Key: "k", Chat: "http://localhost"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(Path())
	for _, id := range []string{"codex", "tier-key", "missing"} {
		if SetBackupUsageTiers(id, &[]BackupUsageTier{{101, 1}}) == nil {
			t.Fatalf("invalid policy accepted %s", id)
		}
	}
	if SetBackupUsageTiers("tier-key", nil) == nil {
		t.Fatal("key accepted defaults")
	}
	p, _ := Find("tier-key")
	p.BackupUsageTiers = &[]BackupUsageTier{}
	if Save(*p) == nil {
		t.Fatal("key Save accepted policy")
	}
	p, _ = Find("codex")
	p.BackupUsageTiers = &[]BackupUsageTier{{20, 8}}
	if Save(*p) == nil {
		t.Fatal("Save accepted invalid policy")
	}
	after, _ := os.ReadFile(Path())
	if !bytes.Equal(before, after) {
		t.Fatal("failure wrote providers")
	}
	// Numeric disk policy remains repairable, but does not become defaults.
	invalid := strings.Replace(string(after), `"backupUsageTiers": []`, `"backupUsageTiers": [{"maxUsed":101,"maxResetDays":7}]`, 1)
	if err := os.WriteFile(Path(), []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	if p.BackupUsageTiers == nil || len(p.BackupPromotionTiers()) != 0 {
		t.Fatal("invalid disk policy fell back")
	}
	if Save(*p) == nil {
		t.Fatal("unchanged invalid save accepted")
	}
	if b, _ := os.ReadFile(Path()); string(b) != invalid {
		t.Fatal("invalid disk policy mutated")
	}
	// Without a valid recovery generation, a broken shared reader refuses edits.
	if err := os.Remove(Path() + ".bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(`{"providers":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SetBackupUsageTiers("codex", nil); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("broken file setter: %v", err)
	}
	if b, _ := os.ReadFile(Path()); string(b) != `{"providers":` {
		t.Fatal("broken file overwritten")
	}
}
