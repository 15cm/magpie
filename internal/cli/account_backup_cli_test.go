package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

func TestAccountBackupCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DSH_HOME", "")
	jwt := func(m map[string]any) string {
		b, _ := json.Marshal(m)
		return "h." + base64.RawURLEncoding.EncodeToString(b) + ".s"
	}
	auth, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"id_token": jwt(map[string]any{"email": "me@example.com"}), "access_token": jwt(map[string]any{"exp": time.Now().Add(time.Hour).Unix()}), "refresh_token": "r", "account_id": "acct-1"}})
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), auth, 0o600); err != nil {
		t.Fatal(err)
	}
	provider.ForgetAccounts()
	t.Cleanup(provider.ForgetAccounts)
	run := func(args ...string) (string, error) {
		return stdoutOf(t, func() error { return providerCmd(append([]string{"provider"}, args...)) })
	}
	if out, err := run("account-backup", "codex"); err != nil || !strings.Contains(out, "me@example.com · auto") {
		t.Fatalf("list %q %v", out, err)
	}
	if err := provider.SetAccountName("codex", "me@example.com", "work"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"backup", "primary", "auto"} {
		out, err := run("account-backup", "codex", "work", mode)
		if err != nil || !strings.Contains(out, "me@example.com · "+mode) {
			t.Fatalf("%s: %q %v", mode, out, err)
		}
		p, _ := provider.Find("codex")
		want := mode
		if mode == "auto" {
			want = ""
		}
		if p.AccountBackupMode("me@example.com") != want {
			t.Fatalf("mode not persisted: %v", p.AccountBackup)
		}
	}
	if out, err := run("pool", "codex", "--backup-5h", "on"); err != nil || !strings.Contains(out, "backup without 5h limit · on") {
		t.Fatalf("pool %q %v", out, err)
	}
	if out, err := run("account-backups", "codex"); err != nil || !strings.Contains(out, "auto (backup when no 5-hour window)") {
		t.Fatalf("auto %q %v", out, err)
	}
	for _, key := range []string{"backup-5h", "backup-no-5h"} {
		if _, err := run("set", "codex", key+"=off"); err != nil {
			t.Fatal(err)
		}
		p, _ := provider.Find("codex")
		if p.BackupNoFiveHour {
			t.Fatal("set off not persisted")
		}
		if _, err := run("set", "codex", key+"=on"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := run("account-backup", "codex", "work", "backup"); err != nil {
		t.Fatal(err)
	}
	if out, err := run("pool", "codex"); err != nil || !strings.Contains(out, "· backup") {
		t.Fatalf("pool override: %q %v", out, err)
	}
	before, _ := os.ReadFile(provider.Path())
	for _, args := range [][]string{{"account-backup"}, {"account-backup", "codex", "work", "maybe"}, {"account-backup", "codex", "nobody", "backup"}, {"account-backup", "codex", "work", "backup", "extra"}, {"pool", "codex", "--backup-5h", "maybe"}, {"set", "codex", "backup-5h=maybe"}} {
		if _, err := run(args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	after, _ := os.ReadFile(provider.Path())
	if string(before) != string(after) {
		t.Fatal("invalid command changed settings")
	}
	if err := provider.Save(provider.Provider{ID: "keybackup", Name: "Key", Key: "k", Chat: "http://localhost"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"account-backup", "keybackup"}, {"set", "keybackup", "backup-5h=on"}, {"pool", "keybackup", "--backup-5h", "on"}} {
		if _, err := run(args...); err == nil || !strings.Contains(err.Error(), "has keys, not subscription accounts") {
			t.Errorf("key provider: %v: %v", args, err)
		}
	}
}
