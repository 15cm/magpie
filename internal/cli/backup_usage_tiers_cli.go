package cli

import (
	"fmt"
	"strings"

	"github.com/yetone/magpie/internal/provider"
)

const backupUsageTiersUsage = `magpie provider backup-usage-tiers <id> [default|off|used:days,...]
Weekly promotion of configured backups: Used % <= used AND future reset within <= days
(elapsed 24-hour days); any row may match, every applicable weekly window must qualify.
No value shows the policy; default restores server defaults; off saves an empty list.`

func backupUsageTiersCmd(rest []string) error {
	for _, arg := range rest {
		if arg == "--help" || arg == "-h" {
			fmt.Println(backupUsageTiersUsage)
			return nil
		}
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("%s", backupUsageTiersUsage)
		}
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("%s", backupUsageTiersUsage)
	}
	p, err := provider.Find(rest[0])
	if err != nil {
		return err
	}
	if p.Account == nil {
		return fmt.Errorf("%s has keys, not subscription accounts", p.Name)
	}
	if len(rest) == 2 {
		tiers, err := provider.ParseBackupUsageTiers(rest[1])
		if err != nil {
			return err
		}
		if err := provider.SetBackupUsageTiers(p.ID, tiers); err != nil {
			return err
		}
		p, err = provider.Find(p.ID)
		if err != nil {
			return err
		}
	}
	printBackupUsageTiers(*p)
	return nil
}

func printBackupUsageTiers(p provider.Provider) {
	mode := "default"
	tiers := p.BackupPromotionTiers()
	if p.BackupUsageTiers != nil {
		mode = "custom"
		tiers = *p.BackupUsageTiers
		if len(tiers) == 0 {
			mode = "off"
		}
	}
	fmt.Printf("weekly promotion tiers · %s\n", mode)
	for _, tier := range tiers {
		fmt.Printf("  used <= %g%% · reset within <= %g days\n", tier.MaxUsed, tier.MaxResetDays)
	}
	if err := provider.CheckBackupUsageTiers(tiers); err != nil {
		fmt.Println("  invalid:", err)
	}
}
