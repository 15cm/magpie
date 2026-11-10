package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"
)

// Ways an account is classified for Smart backup routing (AccountBackup).
const (
	AccountBackupAuto    = ""
	AccountBackupOn      = "backup"
	AccountBackupPrimary = "primary"
)

// BackupUsageTier admits a backup within both inclusive weekly thresholds.
type BackupUsageTier struct {
	MaxUsed      float64 `json:"maxUsed"`
	MaxResetDays float64 `json:"maxResetDays"`
}

func DefaultBackupUsageTiers() []BackupUsageTier {
	return []BackupUsageTier{{20, 7}, {40, 5}, {60, 3}, {80, 1}}
}

func copyBackupUsageTiers(tiers *[]BackupUsageTier) *[]BackupUsageTier {
	if tiers == nil {
		return nil
	}
	out := append([]BackupUsageTier{}, (*tiers)...)
	return &out
}

// BackupPromotionTiers returns an owned effective policy; invalid saved values
// earn no promotion, while remaining available for repair in the editor.
func (p Provider) BackupPromotionTiers() []BackupUsageTier {
	if p.BackupUsageTiers == nil {
		return DefaultBackupUsageTiers()
	}
	if CheckBackupUsageTiers(*p.BackupUsageTiers) != nil {
		return []BackupUsageTier{}
	}
	return *copyBackupUsageTiers(p.BackupUsageTiers)
}

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func CheckBackupUsageTiers(tiers []BackupUsageTier) error {
	for i, t := range tiers {
		if !finite(t.MaxUsed) || t.MaxUsed < 0 || t.MaxUsed > 100 {
			return fmt.Errorf("weekly promotion tier %d: maxUsed must be a finite percentage from 0 to 100", i+1)
		}
		if !finite(t.MaxResetDays) || t.MaxResetDays <= 0 || t.MaxResetDays > 7 || time.Duration(t.MaxResetDays*float64(24*time.Hour)) < time.Nanosecond {
			return fmt.Errorf("weekly promotion tier %d: maxResetDays must be greater than 0 and at most 7 days, representing at least 1ns", i+1)
		}
	}
	return nil
}

// UnmarshalJSON rejects incomplete rows instead of treating missing fields as 0.
// Range checks are separate so invalid numeric disk settings can be repaired.
func (t *BackupUsageTier) UnmarshalJSON(data []byte) error {
	var row struct {
		MaxUsed      *float64 `json:"maxUsed"`
		MaxResetDays *float64 `json:"maxResetDays"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&row); err != nil {
		return err
	}
	if row.MaxUsed == nil || row.MaxResetDays == nil {
		return fmt.Errorf("weekly promotion tier needs numeric maxUsed and maxResetDays")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected content after weekly promotion tier")
	}
	*t = BackupUsageTier{*row.MaxUsed, *row.MaxResetDays}
	return nil
}

func ParseBackupUsageTiers(s string) (*[]BackupUsageTier, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "default":
		return nil, nil
	case "off":
		return copyBackupUsageTiers(new([]BackupUsageTier)), nil
	}
	tiers := []BackupUsageTier{}
	for _, row := range strings.Split(s, ",") {
		u, days, ok := strings.Cut(row, ":")
		if !ok {
			return nil, fmt.Errorf("weekly promotion tiers use used:days pairs, off or default")
		}
		used, err := strconv.ParseFloat(strings.TrimSpace(u), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weekly usage: %q", u)
		}
		reset, err := strconv.ParseFloat(strings.TrimSpace(days), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid reset days: %q", days)
		}
		tiers = append(tiers, BackupUsageTier{used, reset})
	}
	if err := CheckBackupUsageTiers(tiers); err != nil {
		return nil, err
	}
	return &tiers, nil
}

func SetBackupUsageTiers(id string, tiers *[]BackupUsageTier) error {
	p, err := Find(id)
	if err != nil {
		return err
	}
	if p.Account == nil {
		return fmt.Errorf("%s has keys, not subscription accounts", p.Name)
	}
	p.BackupUsageTiers = copyBackupUsageTiers(tiers)
	return Save(*p)
}

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

// BackupPromoted checks paired usage and elapsed time for any configured tier.
func BackupPromoted(used float64, due, now time.Time, tiers []BackupUsageTier) bool {
	if !finite(used) || used < 0 || used > 100 || used >= SpentShare || !due.After(now) || CheckBackupUsageTiers(tiers) != nil {
		return false
	}
	for _, tier := range tiers {
		if used <= tier.MaxUsed && due.Sub(now) <= time.Duration(tier.MaxResetDays*float64(24*time.Hour)) {
			return true
		}
	}
	return false
}

func (a Allowance) backupUsagePromoted(model string, now time.Time, tiers []BackupUsageTier) bool {
	used, _ := a.For(model, now)
	if !finite(used) || used >= SpentShare || len(tiers) == 0 {
		return false
	}
	model = strings.ToLower(model)
	found := false
	for _, l := range a {
		if !l.applies(model) || l.Span != weekSpan {
			continue
		}
		found = true
		if l.partial || l.Balance != nil || !BackupPromoted(l.Used, l.Resets, now, tiers) {
			return false
		}
	}
	return found
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
	if known && a.backupUsagePromoted(model, now, p.BackupPromotionTiers()) {
		return false
	}
	return true
}
