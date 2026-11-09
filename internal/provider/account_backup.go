package provider

import (
	"fmt"
	"maps"
	"strings"
	"time"
)

// Ways an account is classified for Smart backup routing (AccountBackup).
const (
	AccountBackupAuto    = ""
	AccountBackupOn      = "backup"
	AccountBackupPrimary = "primary"
)

// BackupPromotePace and BackupPromoteLead promote remaining budget to the
// primary tier before a reset would waste it.
const (
	BackupPromotePace = 2.0
	BackupPromoteLead = 24 * time.Hour
)

// AccountBackupMode is the account user's manual Smart backup override.
func (p Provider) AccountBackupMode(user string) string {
	return p.AccountBackup[accountKey(user)]
}

// ParseAccountBackup reads the backup mode accepted by the CLI and GUI.
func ParseAccountBackup(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "backup", "on", "yes":
		return AccountBackupOn, nil
	case "primary", "first":
		return AccountBackupPrimary, nil
	case "", "auto", "default", "off", "no", "-":
		return AccountBackupAuto, nil
	}
	return "", fmt.Errorf("an account's backup mode is backup, primary or auto, not %q", s)
}

// SetAccountBackup sets the manual Smart backup override for account ref.
func SetAccountBackup(id, ref, mode string) error {
	mode, err := ParseAccountBackup(mode)
	if err != nil {
		return err
	}
	p, err := Find(id)
	if err != nil {
		return err
	}
	if p.Account == nil {
		return fmt.Errorf("%s has keys, not subscription accounts", p.Name)
	}
	r, ok := p.accountRef(ref)
	if !ok {
		return fmt.Errorf("%s has no account %q", p.Name, ref)
	}
	if p.AccountBackupMode(r) == mode {
		return nil
	}
	p.AccountBackup = maps.Clone(p.AccountBackup)
	if mode == AccountBackupAuto {
		delete(p.AccountBackup, accountKey(r))
	} else {
		if p.AccountBackup == nil {
			p.AccountBackup = map[string]string{}
		}
		p.AccountBackup[accountKey(r)] = mode
	}
	return Save(*p)
}

// SetBackupNoFiveHour enables automatic Smart backup classification for
// known accounts with a budget window and no short rate-cap window.
func SetBackupNoFiveHour(id string, on bool) error {
	p, err := Find(id)
	if err != nil {
		return err
	}
	if p.Account == nil {
		return fmt.Errorf("%s has keys, not subscription accounts", p.Name)
	}
	p.BackupNoFiveHour = on
	return Save(*p)
}

func normalAccountBackup(m map[string]string) map[string]string {
	var out map[string]string
	for user, mode := range m {
		user = accountKey(user)
		mode, err := ParseAccountBackup(mode)
		if user == "" || err != nil || mode == AccountBackupAuto {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[user] = mode
	}
	return out
}

// LacksShortWindow reports a complete rolling budget for model without
// any short rate-cap window counting that model.
func (a Allowance) LacksShortWindow(model string, now time.Time) bool {
	model = strings.ToLower(model)
	budget := false
	for _, l := range a {
		if !l.applies(model) {
			continue
		}
		if l.Span > 0 && l.Span < budgetSpan {
			return false
		}
		if l.Span >= budgetSpan && l.Balance == nil && !l.partial {
			budget = true
		}
	}
	return budget
}

// BackupPromoted reports quota left with a known future budget reset and
// either enough remaining pace or a reset within BackupPromoteLead.
func BackupPromoted(used, pace float64, due, now time.Time) bool {
	return used < SpentShare && due.After(now) && (pace >= BackupPromotePace || due.Sub(now) <= BackupPromoteLead)
}

// IsBackupAccount classifies user for Smart routing of model at now.
func (p Provider) IsBackupAccount(user, model string, a Allowance, known bool, now time.Time) bool {
	if user == "" {
		return false
	}
	switch p.AccountBackupMode(user) {
	case AccountBackupPrimary:
		return false
	case AccountBackupOn:
	default:
		if !p.BackupNoFiveHour || !known || !a.LacksShortWindow(model, now) {
			return false
		}
	}
	if known {
		// Pace also falls back to short windows and wallets. Only a
		// complete budget window's future reset can promote a backup.
		var budget Allowance
		for _, l := range a {
			if l.applies(strings.ToLower(model)) && l.Span >= budgetSpan && l.Balance == nil && !l.partial {
				budget = append(budget, l)
			}
		}
		if len(budget) > 0 {
			u, _ := a.For(model, now)
			pace, due := budget.Pace(model, now)
			if BackupPromoted(u, pace, due, now) {
				return false
			}
		}
	}
	return true
}
