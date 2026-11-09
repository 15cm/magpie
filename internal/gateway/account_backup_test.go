package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

// backupPool serves four real saved Codex accounts through the gateway's
// production Front handler and a local ChatGPT backend. Allowances stand
// for vendor readings; requests and affinity go through the HTTP stack.
func backupPool(t *testing.T, group bool, affinity string) (*Server, func(string, string) Route, map[string]provider.Allowance) {
	t.Helper()
	codexSignedIn(t, "std-2", "prem-1", "prem-2")
	p, err := provider.Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	p.BackupNoFiveHour = true
	p.Affinity = affinity
	p.Models = []string{"gpt-5.5"}
	if err := provider.Save(*p); err != nil {
		t.Fatal(err)
	}
	model := "codex/gpt-5.5"
	if group {
		if err := provider.SaveGroup(provider.Group{ID: "backup", Name: "Backup", Members: []string{model}, Affinity: affinity}); err != nil {
			t.Fatal(err)
		}
		model = "group/backup"
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if r.URL.Path != "/backend-api/codex/responses" {
			http.NotFound(w, r)
			return
		}
		acct := r.Header.Get("chatgpt-account-id")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, sse(
			`data: {"type":"response.created","response":{"id":"r1","model":"gpt-5.5"}}`,
			`data: {"type":"response.output_text.delta","delta":"from `+acct+`"}`,
			`data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":3000,"output_tokens":1,"input_tokens_details":{"cached_tokens":2500}}}}`))
	}))
	t.Cleanup(up.Close)
	oldBase := provider.CodexBase
	provider.CodexBase = up.URL + "/backend-api/codex"
	t.Cleanup(func() { provider.CodexBase = oldBase })
	now := time.Now()
	week := func(u float64, h int) provider.Limit {
		return provider.Limit{Used: u, Span: 7 * 24 * time.Hour, Resets: now.Add(time.Duration(h) * time.Hour)}
	}
	a := map[string]provider.Allowance{
		"me@example.com": {{Used: 25, Span: 5 * time.Hour, Resets: now.Add(3 * time.Hour)}, week(20, 120)},
		"std-2":          {{Used: 30, Span: 5 * time.Hour, Resets: now.Add(3 * time.Hour)}, week(20, 144)},
		"prem-1":         {week(10, 96)}, "prem-2": {week(5, 72)},
	}
	old := allowances
	allowances = func(string) map[string]provider.Allowance { return a }
	t.Cleanup(func() { allowances = old })
	s := New()
	front := httptest.NewServer(s.Front())
	t.Cleanup(front.Close)
	ask := func(session, want string) Route {
		t.Helper()
		body := fmt.Sprintf(`{"model":%q,"stream":true,"input":[{"role":"user","content":"ping"}]}`, model)
		req, err := http.NewRequestWithContext(t.Context(), "POST", front.URL+"/v1/responses", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-session-id", session)
		res, err := front.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || !strings.Contains(string(b), "from "+want) {
			t.Fatalf("session %s: status %d body %s; want %s", session, res.StatusCode, b, want)
		}
		rs := s.Trace(t.Context(), 0, 0).Routes
		if len(rs) == 0 {
			t.Fatal("no routing trace")
		}
		return rs[len(rs)-1]
	}
	return s, ask, a
}

func TestGatewayBackupRouting(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(fmt.Sprintf("group=%v", group), func(t *testing.T) {
			_, ask, a := backupPool(t, group, provider.AffinityOff)
			// Even unused Premium quota with an earlier week stays behind Standard.
			a["prem-2"][0].Used = 0
			if err := provider.SetBackupNoFiveHour("codex", false); err != nil {
				t.Fatal(err)
			}
			r := ask("default", "acct-4")
			for _, w := range r.Order {
				if w.Backup {
					t.Fatal("default exposed backup flag")
				}
			}
			if err := provider.SetBackupNoFiveHour("codex", true); err != nil {
				t.Fatal(err)
			}
			r = ask("standard", "acct-1")
			backups := 0
			for _, w := range r.Order {
				if w.Backup {
					backups++
				}
			}
			if r.Order[0].Backup || backups != 2 {
				t.Fatalf("backup trace: %+v", r.Order)
			}
			// All short windows at 90%: Smart picks the earlier backup week.
			a["me@example.com"][0].Used = 90
			a["std-2"][0].Used = 92
			a["prem-2"][0].Used = 5
			r = ask("spillover", "acct-4")
			if !r.Order[0].Backup {
				t.Fatalf("spillover not traced as backup: %+v", r.Order)
			}
			// A primary at a configured cap also yields to backups.
			a["me@example.com"][0].Used = 80
			if err := provider.SetAccountCap("codex", "me@example.com", 80); err != nil {
				t.Fatal(err)
			}
			ask("cap", "acct-4")
			if err := provider.SetAccountCap("codex", "me@example.com", 0); err != nil {
				t.Fatal(err)
			}
			a["me@example.com"][0].Name = "5 hours"
			if err := provider.SetWindowCap("codex", "me@example.com", "5 hours", 80); err != nil {
				t.Fatal(err)
			}
			ask("window-cap", "acct-4")
			if err := provider.SetWindowCap("codex", "me@example.com", "5 hours", 0); err != nil {
				t.Fatal(err)
			}
			// A resting primary cannot take traffic from a usable backup.
			a["me@example.com"][0].Used = 25
			restingUntil.Lock()
			restingUntil.m["codex@me@example.com"] = time.Now().Add(time.Hour)
			restingUntil.Unlock()
			ask("rest", "acct-4")
			restingUntil.Lock()
			delete(restingUntil.m, "codex@me@example.com")
			restingUntil.Unlock()
			// The budget's near reset promotes it to compete as primary.
			a["prem-1"][0].Used = 70
			a["prem-1"][0].Resets = time.Now().Add(20 * time.Hour)
			r = ask("promoted", "acct-3")
			if r.Order[0].Backup {
				t.Fatal("promoted account still backup")
			}
			a["prem-1"][0].Resets = time.Now().Add(96 * time.Hour)
			a["std-2"][0].Used = 30
			if err := provider.SetAccountBackup("codex", "me@example.com", "backup"); err != nil {
				t.Fatal(err)
			}
			ask("manual-backup", "acct-2")
			if err := provider.SetAccountBackup("codex", "prem-2", "primary"); err != nil {
				t.Fatal(err)
			}
			r = ask("manual-primary", "acct-4")
			if r.Order[0].Backup {
				t.Fatal("manual primary traced as backup")
			}
		})
	}
}

func TestGatewayBackupWarmCacheAndRecovery(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(fmt.Sprintf("group=%v", group), func(t *testing.T) {
			_, ask, a := backupPool(t, group, "")
			a["me@example.com"][0].Used = 92
			a["std-2"][0].Used = 93
			ask("existing", "acct-4")
			a["me@example.com"][0].Used = 0
			r := ask("existing", "acct-4")
			if r.Affinity == nil || r.Affinity.Why != "cache" || !r.Affinity.Kept {
				t.Fatalf("warm backup not kept: %+v", r.Affinity)
			}
			ask("new", "acct-1")
			// Let the old conversation's cache expire without a wall-clock wait.
			sticks.Lock()
			for k, st := range sticks.m {
				st.at = time.Now().Add(-2 * time.Hour)
				sticks.m[k] = st
			}
			sticks.Unlock()
			ask("existing", "acct-1")
			a["me@example.com"][0].Used = 92
			ask("spent-backup", "acct-4")
			a["me@example.com"][0].Used = 0
			a["prem-2"][0].Used = 98
			ask("spent-backup", "acct-1")
		})
	}
}

func TestGatewayBackupSessionSpreading(t *testing.T) {
	for _, affinity := range []string{provider.AffinitySession, provider.AffinityStrict} {
		for _, group := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/group=%v", affinity, group), func(t *testing.T) {
				_, ask, a := backupPool(t, group, affinity)
				// Two occupied primaries never cause a new session to use a backup.
				ask("primary-1", "acct-1")
				ask("primary-2", "acct-2")
				ask("primary-3", "acct-1")
				a["me@example.com"][0].Used = 92
				a["std-2"][0].Used = 93
				// Once backups lead, session spreading works among them.
				ask("backup-1", "acct-4")
				ask("backup-2", "acct-3")
			})
		}
	}
}

func TestGatewayBackupModelWindows(t *testing.T) {
	fresh(t)
	now := time.Now()
	p := provider.Provider{ID: "multi", BackupNoFiveHour: true, Account: &provider.Account{Agent: "x", User: "me"}}
	cs := []candidate{{p: p, model: "sonnet", rest: "multi#me"}, {p: p, model: "opus", rest: "multi#me"}}
	a := provider.Allowance{{Used: 20, Span: 7 * 24 * time.Hour, Resets: now.Add(120 * time.Hour)}, {Used: 10, Span: 5 * time.Hour, Model: "opus", Resets: now.Add(time.Hour)}}
	old := allowances
	allowances = func(string) map[string]provider.Allowance { return map[string]provider.Allowance{"me": a} }
	t.Cleanup(func() { allowances = old })
	got, wg := weighRouted(provider.Provider{}, cs, "", provider.Chat)
	if got[0].model != "opus" || weighed(cs[0], p, wg, false, provider.Chat).Backup != true || weighed(cs[1], p, wg, false, provider.Chat).Backup {
		t.Fatalf("per-model classification: %+v, %+v", got, wg.backup)
	}
	// One model's expiring budget promotes only that model of the account.
	a = provider.Allowance{{Used: 20, Span: 7 * 24 * time.Hour, Model: "sonnet", Resets: now.Add(120 * time.Hour)}, {Used: 70, Span: 7 * 24 * time.Hour, Model: "opus", Resets: now.Add(20 * time.Hour)}}
	_, wg = weighRouted(provider.Provider{}, cs, "", provider.Chat)
	if !weighed(cs[0], p, wg, false, provider.Chat).Backup || weighed(cs[1], p, wg, false, provider.Chat).Backup {
		t.Fatalf("per-model promotion: %+v", wg.backup)
	}
	// Other routings preserve their ordering and omit the Smart trace flag.
	for _, routing := range []string{provider.Ordered, provider.Pace, provider.LeastUsed} {
		p.Routing = routing
		_, wg = weighRouted(p, cs, "", provider.Chat)
		if weighed(cs[0], p, wg, false, provider.Chat).Backup {
			t.Fatalf("%s exposed Smart backup", routing)
		}
	}
}

func TestBackupBehindTakingTurns(t *testing.T) {
	cs := []candidate{{rest: "primary"}, {rest: "backup"}}
	pl := planned{order: []Weighed{{ID: "primary"}, {ID: "backup", Backup: true}}}
	if readyForNew(pl.order, 1) {
		t.Fatal("backup ready ahead of primary")
	}
	got, _ := after(cs, pl, 0)
	if got[0].rest != "primary" {
		t.Fatal("turn moved onto backup")
	}
	pl.order[0].Backup = true
	if !readyForNew(pl.order, 1) {
		t.Fatal("backup not ready when backup leads")
	}
	got, _ = after(cs, pl, 0)
	if got[0].rest != "backup" {
		t.Fatal("turn did not spread among backups")
	}
}

func TestGatewayBackupGroupModelWindows(t *testing.T) {
	codexSignedIn(t)
	p, err := provider.Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	p.BackupNoFiveHour = true
	now := time.Now()
	a := provider.Allowance{
		{Used: 20, Span: 7 * 24 * time.Hour, Resets: now.Add(120 * time.Hour)},
		{Used: 10, Span: 5 * time.Hour, Model: "gpt-5.4", Resets: now.Add(time.Hour)},
	}
	old := allowances
	allowances = func(string) map[string]provider.Allowance { return map[string]provider.Allowance{"me@example.com": a} }
	t.Cleanup(func() { allowances = old })
	g := provider.Group{ID: "model-backup", Members: []string{"codex/gpt-5.5", "codex/gpt-5.4"}}
	ms := []provider.Member{
		{ID: g.Members[0], Path: []string{g.Members[0]}, Provider: *p, Model: "gpt-5.5"},
		{ID: g.Members[1], Path: []string{g.Members[1]}, Provider: *p, Model: "gpt-5.4"},
	}
	s := New()
	check := func() {
		t.Helper()
		cs, pl := s.planGroup(g, ms, provider.Responses)
		if len(cs) != 2 || len(pl.order) != 2 {
			t.Fatalf("group planned %d candidates, %d trace rows", len(cs), len(pl.order))
		}
		if cs[0].model != "gpt-5.4" || pl.order[0].Backup || !pl.order[1].Backup || pl.order[0].ID != pl.order[1].ID {
			t.Fatalf("same account's model classifications: %+v", pl.order)
		}
	}
	check()
	a = provider.Allowance{
		{Used: 20, Span: 7 * 24 * time.Hour, Model: "gpt-5.5", Resets: now.Add(120 * time.Hour)},
		{Used: 70, Span: 7 * 24 * time.Hour, Model: "gpt-5.4", Resets: now.Add(20 * time.Hour)},
	}
	check()
}
