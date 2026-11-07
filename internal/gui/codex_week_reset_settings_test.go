package gui

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

func TestSettingsStateCodexWeekResetMinHours(t *testing.T) {
	sandboxHome(t)
	for _, tt := range []struct {
		raw   string
		hours int
		bad   string
	}{
		{"", 0, ""}, {"0", 0, ""}, {" 24 ", 24, ""}, {"168", 168, ""}, {"abc", 0, "abc"},
		{"-1", 0, "-1"}, {"169", 0, "169"}, {"999999999999999999999999", 0, "999999999999999999999999"},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			t.Setenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS", tt.raw)
			s := settingsState()
			if s.CodexWeekResetMinHours != tt.hours || s.CodexWeekResetMinHoursBad != tt.bad {
				t.Fatalf("hours=%d bad=%q, want %d %q", s.CodexWeekResetMinHours, s.CodexWeekResetMinHoursBad, tt.hours, tt.bad)
			}
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(b, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["codexWeekResetMinHours"] != float64(tt.hours) {
				t.Fatalf("read-only hours missing from JSON: %s", b)
			}
			if _, ok := fields["codexWeekResetMinHoursBad"]; ok != (tt.bad != "") {
				t.Fatalf("unexpected invalid value presence: %s", b)
			}
		})
	}
}

// Both the initial state and ordinary refresh use this response, including
// cached states whose Settings field remains the persisted settings type.
func TestSettingsStateResponseCodexWeekResetMinHours(t *testing.T) {
	sandboxHome(t)
	if err := settings.Save(settings.Settings{Theme: "dark", CodexAutoReset: []string{"me@example.com"}}); err != nil {
		t.Fatal(err)
	}
	handler := Handler(nil, nil)
	for _, raw := range []string{"0", "24", "abc", "-1", "169", "999999999999999999999999", "0"} {
		t.Setenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS", raw)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/state", nil))
		if rec.Code != 200 {
			t.Fatalf("state status %d: %s", rec.Code, rec.Body)
		}
		var got struct {
			Settings struct {
				Theme string   `json:"theme"`
				Users []string `json:"codexAutoReset"`
				Hours *int     `json:"codexWeekResetMinHours"`
				Bad   string   `json:"codexWeekResetMinHoursBad"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		hours, bad := 0, ""
		if raw == "24" {
			hours = 24
		}
		if raw != "0" && raw != "24" {
			bad = raw
		}
		if got.Settings.Hours == nil || *got.Settings.Hours != hours || got.Settings.Bad != bad {
			t.Fatalf("state policy for %q: %s", raw, rec.Body)
		}
		if got.Settings.Theme != "dark" || len(got.Settings.Users) != 1 {
			t.Fatalf("persisted fields lost: %s", rec.Body)
		}
	}
}

func TestSettingsStateProfileWrapperAndCachedPolicy(t *testing.T) {
	sandboxHome(t)
	forgetLastState()
	t.Cleanup(forgetLastState)
	t.Setenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS", "24")
	s := state()
	b, err := json.Marshal(struct {
		stateJSON
		Changed int               `json:"changed"`
		Library map[string]string `json:"library"`
	}{s, 3, map[string]string{"result": "kept"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["changed"]) != "3" || string(got["library"]) != `{"result":"kept"}` {
		t.Fatalf("profile fields lost: %s", b)
	}
	var policy codexWeekResetJSON
	if err := json.Unmarshal(got["settings"], &policy); err != nil {
		t.Fatal(err)
	}
	if policy.CodexWeekResetMinHours != 24 {
		t.Fatalf("profile policy lost: %s", b)
	}
	keepState(s)
	t.Setenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS", "abc")
	cached, ok := lastKnown()
	if !ok || cached.Settings.CodexWeekResetMinHoursBad != "abc" || cached.Settings.CodexWeekResetMinHours != 0 {
		t.Fatalf("cached policy not current: ok=%v settings=%+v", ok, cached.Settings.codexWeekResetJSON)
	}
}
