package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

func TestBackupUsageTiersCLI(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	jwt := func(m map[string]any) string {
		b, _ := json.Marshal(m)
		return "h." + base64.RawURLEncoding.EncodeToString(b) + ".s"
	}
	auth, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"id_token": jwt(map[string]any{"email": "me@example.com"}), "access_token": jwt(map[string]any{"exp": time.Now().Add(time.Hour).Unix()}), "refresh_token": "r", "account_id": "acct-1"}})
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), auth, 0600); err != nil {
		t.Fatal(err)
	}
	provider.ForgetAccounts()
	t.Cleanup(provider.ForgetAccounts)
	run := func(args ...string) (string, error) {
		return stdoutOf(t, func() error { return providerCmd(append([]string{"provider"}, args...)) })
	}
	if out, err := run("backup-usage-tiers", "codex"); err != nil || !strings.Contains(out, "default") || !strings.Contains(out, "used <= 20% · reset within <= 7 days") {
		t.Fatalf("show: %s %v", out, err)
	}
	for _, tc := range []struct{ value, mode string }{{"15.5:6,55:2.5", "custom"}, {"off", "off"}, {"default", "default"}} {
		out, err := run("backup-usage-tiers", "codex", tc.value)
		if err != nil || !strings.Contains(out, "weekly promotion tiers · "+tc.mode) {
			t.Fatalf("%s: %s %v", tc.value, out, err)
		}
		p, _ := provider.Find("codex")
		if (p.BackupUsageTiers == nil) != (tc.mode == "default") {
			t.Fatal("saved mode lost")
		}
		if tc.mode == "custom" && (*p.BackupUsageTiers)[1].MaxResetDays != 2.5 {
			t.Fatal("fractional value lost")
		}
		if tc.mode == "off" && len(*p.BackupUsageTiers) != 0 {
			t.Fatal("off not empty")
		}
		out, err = run("pool", "codex")
		if err != nil || !strings.Contains(out, "weekly promotion tiers · "+tc.mode) || !strings.Contains(out, "magpie provider backup-usage-tiers codex") {
			t.Fatalf("pool: %s %v", out, err)
		}
	}
	if err := provider.Save(provider.Provider{ID: "tier-key", Name: "Key", Chat: "http://localhost", Key: "k"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(provider.Path())
	for _, args := range [][]string{
		{"backup-usage-tiers"}, {"backup-usage-tiers", "codex", "off", "extra"}, {"backup-usage-tiers", "codex", "--nope"},
		{"backup-usage-tiers", "codex", "20:1,"}, {"backup-usage-tiers", "codex", "20:8"}, {"backup-usage-tiers", "codex", "NaN:1"},
		{"backup-usage-tiers", "tier-key"}, {"backup-usage-tiers", "tier-key", "off"}, {"backup-usage-tiers", "missing", "default"},
	} {
		if _, err := run(args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if out, err := run("backup-usage-tiers", "--help"); err != nil || !strings.Contains(out, "elapsed 24-hour days") {
		t.Fatalf("help: %s %v", out, err)
	}
	after, _ := os.ReadFile(provider.Path())
	if !bytes.Equal(before, after) {
		t.Fatal("invalid input/help wrote policy")
	}
}
