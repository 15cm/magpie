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

func TestQuotaAutoResetMinHours(t *testing.T) {
	groupsHome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	jwt := func(m map[string]any) string {
		b, _ := json.Marshal(m)
		return "h." + base64.RawURLEncoding.EncodeToString(b) + ".s"
	}
	auth, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{
		"id_token":      jwt(map[string]any{"email": "me@example.com"}),
		"access_token":  jwt(map[string]any{"exp": time.Now().Add(time.Hour).Unix()}),
		"refresh_token": "r", "account_id": "acct-1"}})
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), auth, 0o600); err != nil {
		t.Fatal(err)
	}
	provider.ForgetAccounts()
	t.Cleanup(provider.ForgetAccounts)
	for _, c := range []struct{ raw, words string }{
		{"", ""}, {"0", ""}, {"24", " and refresh is at least 24h ahead"}, {"abc", " (paused: MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=\"abc\" is not whole hours from 0 to 168)"},
	} {
		t.Setenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS", c.raw)
		out, err := runOut(t, "quota", "auto-reset", "me@example.com", "week", "on")
		if err != nil || !strings.Contains(out, "once its week is used up"+c.words+" and no other account") {
			t.Fatalf("set %q: %q %v", c.raw, out, err)
		}
		out, err = runOut(t, "quota", "auto-reset", "me@example.com")
		want := "uses a reset by itself when its week runs out" + c.words
		if err != nil || !strings.Contains(out, want) {
			t.Fatalf("read %q: %q %v", c.raw, out, err)
		}
		if err := provider.SetCodexAutoReset("me@example.com", true); err != nil {
			t.Fatal(err)
		}
		if got := autoResetSay("me@example.com"); got != want+", or before one expires" {
			t.Fatalf("both: %q", got)
		}
		if err := provider.SetCodexAutoResetCase("me@example.com", provider.ResetCaseWeek, false); err != nil {
			t.Fatal(err)
		}
		if got := autoResetSay("me@example.com"); got != "uses a reset by itself before one expires" {
			t.Fatalf("expiry: %q", got)
		}
		if err := provider.SetCodexAutoReset("me@example.com", false); err != nil {
			t.Fatal(err)
		}
	}
}
