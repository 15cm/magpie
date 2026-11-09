package gui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

func TestAccountBackupAPI(t *testing.T) {
	home := sandboxHome(t)
	codexHome := filepath.Join(home, ".codex")
	t.Setenv("CODEX_HOME", codexHome)
	jwt := func(c any) string {
		b, _ := json.Marshal(c)
		return "e30." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
	}
	auth, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{
		"id_token":     jwt(map[string]any{"email": "a@x.y", "https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "plus", "chatgpt_account_id": "acc"}}),
		"access_token": jwt(map[string]any{"exp": time.Now().Add(24 * time.Hour).Unix()}), "refresh_token": "r", "account_id": "acc"}})
	if err := os.MkdirAll(codexHome, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), auth, 0600); err != nil {
		t.Fatal(err)
	}
	provider.LoginUsageVia(func(_ context.Context, _ string) map[string]provider.SubscriptionQuota { return nil })
	t.Cleanup(func() { provider.LoginUsageVia(nil) })
	mux := http.NewServeMux()
	providerRoutes(mux, nil)
	post := func(action, body string, ok bool) {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider/"+action, strings.NewReader(body)))
		if (w.Code == 200) != ok {
			t.Fatalf("%s: %d %s", action, w.Code, w.Body)
		}
	}
	post("backupnofivehour", `{"id":"codex","backupNoFiveHour":true}`, true)
	for _, mode := range []string{"backup", "primary", "auto"} {
		post("accountbackup", `{"id":"codex","account":"A@X.Y","backup":"`+mode+`"}`, true)
		p, err := provider.Find("codex")
		if err != nil {
			t.Fatal(err)
		}
		want := mode
		if mode == "auto" {
			want = ""
		}
		if p.AccountBackupMode("a@x.y") != want {
			t.Fatalf("mode = %q, want %q", p.AccountBackupMode("a@x.y"), want)
		}
	}
	post("accountbackup", `{"id":"codex","account":"a@x.y","backup":"backup"}`, true)
	post("accountbackup", `{"id":"codex","account":"a@x.y","backup":"bogus"}`, false)
	post("accountbackup", `{"id":"codex","account":"missing","backup":"backup"}`, false)
	post("save", `{"id":"codex","from":"codex","name":"Codex","backupNoFiveHour":false,"accountBackup":{"a@x.y":"primary"}}`, true)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/providers", nil))
	var listed struct {
		Providers []providerJSON `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range listed.Providers {
		if p.ID == "codex" {
			found = true
			if !p.BackupNoFiveHour || p.AccountBackup["a@x.y"] != "backup" {
				t.Fatalf("settings lost in listing/save: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("Codex missing from /api/providers")
	}
	if err := provider.Save(provider.Provider{ID: "backup-key", Name: "Key", Key: "k", Chat: "http://127.0.0.1:1/v1"}); err != nil {
		t.Fatal(err)
	}
	post("accountbackup", `{"id":"backup-key","account":"k","backup":"backup"}`, false)
	post("backupnofivehour", `{"id":"backup-key","backupNoFiveHour":true}`, false)
	post("backupnofivehour", `{"id":"codex","backupNoFiveHour":false}`, true)
	p, _ := provider.Find("codex")
	if p.BackupNoFiveHour {
		t.Fatal("automatic backup did not turn off")
	}
}

func TestAccountBackupUsageHint(t *testing.T) {
	now := time.Now()
	balance := 4.0
	qs := map[string]provider.SubscriptionQuota{}
	allowances := map[string]provider.Allowance{
		"standard": {{Span: 5 * time.Hour}, {Span: 7 * 24 * time.Hour}},
		"premium":  {{Span: 7 * 24 * time.Hour}},
		"wallet":   {{Span: 7 * 24 * time.Hour, Balance: &balance}},
		"short":    {{Span: 5 * time.Hour}},
	}
	for _, user := range []string{"standard", "premium", "wallet", "short", "unknown"} {
		qs[user] = provider.SubscriptionQuota{User: user}
	}
	for _, on := range []bool{false, true} {
		for _, mode := range []string{"", "backup", "primary"} {
			got := accountBackupUsage(provider.Provider{BackupNoFiveHour: on, AccountBackup: map[string]string{"premium": mode}}, qs, allowances, now)
			for user, q := range got {
				if q.ConfiguredAutoBackup != (on && user == "premium") {
					t.Errorf("%s auto=%v mode=%s: hint=%v", user, on, mode, q.ConfiguredAutoBackup)
				}
				original, _ := json.Marshal(qs[user])
				projected, _ := json.Marshal(q)
				if !q.ConfiguredAutoBackup && string(original) != string(projected) {
					t.Errorf("default quota changed: %s / %s", original, projected)
				}
			}
		}
	}
}
