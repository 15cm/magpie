package provider

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
)

// TestAccountSettingsKept: what the user sets on a signed-in account's
// provider is written by Save and read back by Find, every setting alike.
// Save and withPicks kept two lists, and a setting on one alone was lost
// (#4738: `magpie provider set codex additional-tools=on` was on neither,
// so the setting seemed taken and nothing changed; an account's extraBody
// and reply headers were read back but never written; its rests were
// written but never read back, so the next Save cleared them).
func TestAccountSettingsKept(t *testing.T) {
	signIn(t)
	for _, id := range []string{"codex", "copilot"} {
		p, err := Find(id)
		if err != nil {
			t.Fatal(err)
		}
		q := *p
		q.AdditionalTools = true
		q.BackupNoFiveHour = true
		q.BackupUsageTiers = &[]BackupUsageTier{{15.5, 6}, {55, 2.5}}
		q.AccountBackup = map[string]string{"me@example.com": AccountBackupOn}
		q.ExtraBody = ExtraBody{"service_tier": json.RawMessage(`"priority"`)}
		q.ReplyHeaders = []string{"X-Trace-Id"}
		q.QuotaRest = 90
		q.RestCap = 20
		q.LimitRestOff = true
		q.Patience = 30
		if err := Save(q); err != nil {
			t.Fatal(err)
		}
		got, err := Find(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Account == nil {
			t.Fatalf("%s: no longer the signed-in account: %+v", id, got)
		}
		if !got.BackupNoFiveHour || !maps.Equal(got.AccountBackup, q.AccountBackup) {
			t.Errorf("%s: backup settings not kept: %v %v", id, got.BackupNoFiveHour, got.AccountBackup)
		}
		if got.BackupUsageTiers == nil || !slices.Equal(*got.BackupUsageTiers, *q.BackupUsageTiers) {
			t.Errorf("%s: tier policy lost", id)
		}
		if !got.AdditionalTools {
			t.Errorf("%s: additional-tools not kept", id)
		}
		if !maps.EqualFunc(got.ExtraBody, q.ExtraBody, func(a, b json.RawMessage) bool { return string(a) == string(b) }) {
			t.Errorf("%s: extraBody %v, want %v", id, got.ExtraBody, q.ExtraBody)
		}
		if !slices.Equal(got.ReplyHeaders, q.ReplyHeaders) {
			t.Errorf("%s: reply headers %v, want %v", id, got.ReplyHeaders, q.ReplyHeaders)
		}
		if got.QuotaRest != 90 || got.RestCap != 20 || !got.LimitRestOff || got.Patience != 30 {
			t.Errorf("%s: rests %d %d %v, patience %d", id, got.QuotaRest, got.RestCap, got.LimitRestOff, got.Patience)
		}
		// set back, each is the default again
		r := *got
		r.AdditionalTools, r.ExtraBody, r.ReplyHeaders = false, nil, nil
		r.BackupNoFiveHour, r.AccountBackup = false, nil
		r.BackupUsageTiers = nil
		r.QuotaRest, r.RestCap, r.LimitRestOff, r.Patience = 0, 0, false, 0
		if err := Save(r); err != nil {
			t.Fatal(err)
		}
		if got, _ := Find(id); got.BackupNoFiveHour || len(got.AccountBackup) > 0 || got.AdditionalTools || len(got.ExtraBody) > 0 || len(got.ReplyHeaders) > 0 || got.QuotaRest != 0 || got.RestCap != 0 || got.LimitRestOff || got.Patience != 0 {
			t.Errorf("%s: left after clearing: %+v", id, got)
		}
	}
}

func TestBackupUsageTiersAccountViews(t *testing.T) {
	for _, agent := range []string{"codex", "claude"} {
		t.Run(agent, func(t *testing.T) {
			threeAccounts(t, agent)
			for _, state := range []string{"default", "off", "15.5:6,55:2.5"} {
				tiers, err := ParseBackupUsageTiers(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := SetBackupUsageTiers(agent, tiers); err != nil {
					t.Fatal(err)
				}
				p, err := Find(agent)
				if err != nil {
					t.Fatal(err)
				}
				views := p.AlsoOn()
				if len(views) != 2 {
					t.Fatalf("need two secondary views, got %d", len(views))
				}
				for _, q := range append(views, *p) {
					if (q.BackupUsageTiers == nil) != (tiers == nil) || !slices.Equal(q.BackupPromotionTiers(), p.BackupPromotionTiers()) {
						t.Fatalf("%s: view lost %s", q.Account.User, state)
					}
				}
				if tiers != nil && len(*tiers) > 0 {
					(*views[0].BackupUsageTiers)[0].MaxUsed = 99
					if p.BackupPromotionTiers()[0].MaxUsed != 15.5 || views[1].BackupPromotionTiers()[0].MaxUsed != 15.5 {
						t.Fatal("secondary views share policy")
					}
					all := All()
					for i := range all {
						if all[i].ID == agent {
							(*all[i].BackupUsageTiers)[0].MaxUsed = 98
						}
					}
					again, _ := Find(agent)
					if again.BackupPromotionTiers()[0].MaxUsed != 15.5 {
						t.Fatal("All shares policy with catalog")
					}
				}
				// Stored subscription picks carry no credential Account pointer. Save
				// must recognize the subscription from Accounts, as for unrelated edits.
				f, err := read()
				if err != nil {
					t.Fatal(err)
				}
				for _, q := range f.Providers {
					if q.ID == agent {
						if q.Account != nil {
							t.Fatal("stored credential pointer")
						}
						if err := Save(q); err != nil {
							t.Fatal(err)
						}
					}
				}
				again, _ := Find(agent)
				if (again.BackupUsageTiers == nil) != (tiers == nil) || !slices.Equal(again.BackupPromotionTiers(), p.BackupPromotionTiers()) {
					t.Fatal("stored picks Save lost policy")
				}
			}
		})
	}
}
