# Premium backup routing: two-tier Smart backup with weekly expiry promotion

Status: **Approved by the user (2026-11-09)**, with decisions D1 to D7
confirmed.

Base: `main` at `f2574bd41c8d4d5f76a1d831c553630d8f16959c` (worktree `feat-premium-backup`).

## Goal and acceptance criteria

### Goal

When a user pools multiple GPT Business Standard accounts (which report both a
5-hour rate-cap window and a 7-day weekly window) alongside multiple GPT
Premium accounts (which report only a 7-day weekly window and no 5-hour
window), `Smart` routing today ranks accounts by unused status (0% first) and
then by their longest window's reset time (`Soon[0]`, the week, before the
5-hour window). As a result, `Smart` routes traffic to Premium accounts
whenever a Premium account is at 0% or its weekly reset hour is earlier than a
Standard account's, and `Make first` only breaks ties within the same renewal
hour while `In order` loses `Smart` selection altogether.

After this change, Magpie supports **two-tier Smart backup routing** with an
**expiry safeguard** so weekly quota on backup accounts is not wasted:

1. **Classification (Auto + Manual override):**
   - A provider-level setting `BackupNoFiveHour` ("Use accounts without a
     5-hour limit as backup", off by default) automatically treats accounts
     whose known allowance for the requested model has a rolling budget window
     (`>= 24h`) and no short rate-cap window (`< 24h`) as backup accounts.
   - A per-account setting `AccountBackup` (`auto` / `backup` / `primary`) can
     explicitly mark any account as `backup` (even if it has a 5-hour window,
     or while `BackupNoFiveHour` is off) or `primary` (exempting it from
     `BackupNoFiveHour`).
2. **Two-tier Smart ordering & 90% spillover threshold:**
   - Under `Smart` routing, primary (non-backup) accounts in tier 0 (`< 90%` of
     any `< 24h` window and `< 98%` of any `>= 24h` window) go first, ordered
     among themselves by `Smart`'s normal rules (learning accounts, unused by
     fewest recent tokens, soonest renewal, lapse, wallet balance, listed
     order).
   - Backup accounts in tier 0 (`< 90%` of any `< 24h` window and `< 98%` of
     any `>= 24h` window) go next — **ahead** of tier 1 (`90%–97%` "low"
     rate-cap accounts) and tier 2 (`>= 98%` "spent" accounts) — and are
     ordered among themselves by `Smart`'s full rules.
   - When all Standard (primary) accounts reach `>= 90%` of their 5-hour
     window, hit a configured usage cap (`accountCaps` / `accountWindowCaps`),
     or are resting after a 429/error, **new conversations** automatically
     spill over to the backup pool (chosen by `Smart` among the Premium
     accounts).
3. **Weekly expiry promotion ("use before weekly reset wastes it"):**
   - A backup account (whether auto-detected or manually marked `backup`) with
     known allowance and quota left (`used < 98%`) is promoted to the primary
     tier when its budget (`>= 24h`) window has a known future reset (`due >
     now`) and either:
     - its remaining weekly pace is at least **2.0%/hour** (`Pace >= 2.0`, e.g.
       `>= 48%` left with 24h to go, or `>= 96%` left with 48h to go), **or**
     - its budget window renews within **24 hours** (`due.Sub(now) <= 24h`).
   - Once promoted, it competes in the primary tier under `Smart`'s normal
     "renews soonest" ordering so its remaining weekly quota is spent before
     reset.
4. **Affinity (`Stays: Auto`) & session spreading:**
   - Existing conversations on a backup account with a warm prompt cache (`< 5`
     minutes old, `>= 1,024` cache tokens) stay on that backup account under
     `Stays: Auto` (`Why: "cache"`) until 98% or until the cache goes cold,
     while new conversations immediately route to any primary Standard account
     that has reset below 90%.
   - Session spreading (`Stays: Session` or `Strict` via `leastHeld` /
     `readyForNew`, and `withLead` / `withSession`) never spreads a new
     conversation onto a backup account while a primary account leads (`< 90%`),
     and spreads across backup accounts once a backup account leads.
5. **Unknown quota & local sign-in switching (`NextLogin`):**
   - Accounts on `auto` whose quota is not known yet (`!known`) are not marked
     backup before their first reading, so `Learns` probes them once as today.
     Accounts manually set to `backup` stay in the backup tier even when their
     quota is unknown (and `Learns` orders them within the backup tier).
   - When the active local Codex or Claude account is spent (`>= 98%` or
     capped) and `NextLogin` selects a spare account to sign the local agent
     into, it prefers non-backup spares with room before switching to a backup
     spare.

### Decisions (confirmed by the user)

| # | Decision | Chosen |
| --- | --- | --- |
| D1 | Core routing policy | Two-tier `Smart` backup: `Smart` among primary (`< 90%`) accounts first, then `Smart` among backup (`< 98%` weekly) accounts, plus promotion before weekly reset wastes quota. |
| D2 | Account classification | Both: provider-level automatic detection for accounts lacking a `< 24h` window (`BackupNoFiveHour`), plus per-account manual override (`AccountBackup`: `auto` / `backup` / `primary`). |
| D3 | Spillover threshold | At **90%** ("running low", `routes.LowShare`) of a primary account's 5-hour window for new conversations, or when capped / resting. Existing warm-cache conversations on primary accounts stay until 98%. |
| D4 | Weekly expiry promotion | Promote a backup account with quota left (`used < 98%`) to the primary tier when its known budget window has `Pace >= 2.0` (%/h) **or** renews within `24h` (`due.Sub(now) <= 24h`). |
| D5 | Affinity (`Stays: Auto`) | Keep existing warm-cache conversations on a backup account (`Why: "cache"`) when a Standard account resets; route new (and cold-cache) conversations to the reset Standard account. |
| D6 | Configuration scope & defaults | Provider-level `BackupNoFiveHour` (off by default so existing setups are unchanged) + per-account `AccountBackup` (`auto`, `backup`, `primary`) in GUI and CLI; works both on single providers and when pooled in a `Smart` routing group. |
| D7 | Unknown quota, per-model windows & `NextLogin` | `auto` accounts wait for a known reading without a `< 24h` window for that model (probing once via `Learns` as today); manually set `backup` accounts stay in backup even when unknown; `NextLogin` prefers non-backup spares with room before backup spares. |

### Acceptance criteria

1. **Default compatibility:** With `BackupNoFiveHour` false and `AccountBackup`
   empty (the default), routing order, trace output, `NextLogin`, GUI, and CLI
   behave identically to `main`.
2. **Auto-detection by 5-hour window (`BackupNoFiveHour`):** With
   `BackupNoFiveHour` on, an account whose known allowance for `model` has a
   `>= 24h` window and no `< 24h` window has `IsBackupAccount == true`, while
   an account with both a `5h` and a `7d` window has `IsBackupAccount == false`.
   Even when the Premium (`7d`-only) account is at `0%` used or its weekly
   reset is earlier than the Standard (`5h` + `7d`) account's (with `> 24h` to
   go and `Pace < 2.0`), `Smart` puts the Standard account (`< 90%` on 5h)
   first.
3. **90% spillover & Smart ordering among backups:** Once all Standard accounts
   reach `>= 90%` on their 5-hour window (or are held at a usage cap, or are
   resting), `Smart` puts fresh (`< 98%`) Premium backup accounts ahead of the
   `90%–97%` Standard accounts, and orders multiple Premium backup accounts
   among themselves using `Smart` (unused with fewest tokens first, then
   soonest weekly reset, etc.).
4. **Weekly expiry promotion (`Pace >= 2%/h` or `<= 24h`):** When a backup
   account (auto or manual) has `used < 98%` and a known future budget reset
   `due > now` with either `pace >= 2.0` (e.g., `20%` used with `36h` left:
   `80/36 = 2.22%/h`) or `due.Sub(now) <= 24h` (e.g., `70%` used with `20h`
   left: `1.5%/h`, `20h <= 24h`), `IsBackupAccount` returns `false` and the
   account is weighed in the primary tier so its remaining quota is used before
   reset. An unstarted fresh week (`due.IsZero()`, `pace = 100/168 = 0.595%/h`)
   is not promoted.
5. **Manual per-account overrides (`AccountBackup`):**
   - Setting an account to `backup` treats it as backup even when
     `BackupNoFiveHour` is off, when it has a 5-hour window, or when its quota
     is unknown (unless promoted by criterion 4).
   - Setting a windowless account to `primary` keeps it in the primary tier
     even when `BackupNoFiveHour` is on.
   - Setting an account to `auto` removes the override.
6. **Affinity & session spreading:**
   - Under `Stays: Auto`, a conversation answered by a Premium backup account
     while Standard accounts were at `>= 90%` stays on that Premium account on
     its next turn while its cache is warm (`Why: "cache"`), even after a
     Standard account's 5-hour window resets to `0%`. A new conversation
     started at the same moment goes to the reset Standard account.
   - Under `Stays: Session` or `Strict`, `leastHeld` (`readyForNew`) does not
     spread a new conversation onto a backup account while a primary account
     leads, and does spread across backup accounts once a backup account leads.
7. **Local sign-in switching (`NextLogin`):** When the active account is spent
   (`>= 98%` or capped) and spares are `[prem-1 (backup, 10% used), std-2
   (primary, 20% used)]`, `NextLogin` chooses `std-2` first, and chooses
   `prem-1` only when all primary spares are also spent/capped.
8. **GUI, Routing trace & CLI:**
   - The Routing page (`client.js`) shows `"kept as backup"` on a backup
     account's row when it sits behind a primary account, and explains why in
     `asides`.
   - The Provider editor (`acctbackup.js`, lazy-loaded so `app.js` and
     `coreScriptBudget` need no budget raise) provides the per-account backup
     pill (`Automatic`, `Backup`, `Primary`) and the provider-level `"Use
     accounts without a 5-hour limit as backup"` checkbox, in all 5 locales
     (`en`, `zh-CN`, `zh-TW`, `ja`, `de`).
   - CLI supports `magpie provider account-backup <id> [account
     [backup|primary|auto]]`, `magpie provider pool <id> --backup-5h on|off`,
     and `magpie provider set <id> backup-5h=on|off`.

## Design

### Module boundaries and data flow

```
providers.json (Provider.BackupNoFiveHour, Provider.AccountBackup)
   │
   ├── internal/provider/account_backup.go
   │     ├── Provider.AccountBackupMode(user)
   │     ├── SetAccountBackup(id, ref, mode) / ParseAccountBackup(s)
   │     ├── SetBackupNoFiveHour(id, on)
   │     ├── Allowance.LacksShortWindow(model, now)
   │     ├── BackupPromoted(used, pace, due, now)
   │     └── Provider.IsBackupAccount(user, model, a, known, now)
   │
   ├── internal/provider/codex_switch.go
   │     └── NextLogin(ctx, agent) ──► prefers non-backup spares with room first
   │
   ├── internal/routes/routes.go & strategies.go
   │     ├── Seat.Backup bool
   │     └── Smart: within fine (seatTier 0) and low (seatTier 1),
   │                !Seat.Backup ranks before Seat.Backup; Smart's full
   │                tie-breaking runs within each tier
   │
   ├── internal/gateway/routing.go, trace.go & affinity.go
   │     ├── weighRouted: computes Seat.Backup & weighing.backup[c.rest]
   │     ├── Weighed.Backup bool (`json:"backup,omitempty"`)
   │     └── affinity.go: backupBehind(order, i) in readyForNew / leastHeld / after
   │
   ├── internal/cli/providers_cli.go & pool_cli.go
   │     ├── `magpie provider account-backup <id> [account [backup|primary|auto]]`
   │     ├── `magpie provider pool <id> [--backup-5h on|off]`
   │     └── `magpie provider set <id> backup-5h=on|off`
   │
   └── internal/gui/providers.go, assets/acctbackup.js, assets/app.js,
       assets/plugins/routing/client.js, assets/i18n.js
         ├── POST /api/provider/accountbackup & /api/provider/backupnofivehour
         ├── lazy-loaded acctbackup.js for per-account pill + provider checkbox
         └── routing trace row status & aside explanation in client.js
```

### Interfaces and exact signatures

#### 1. `internal/provider/provider.go` & `internal/provider/account_backup.go`

On `provider.Provider` (`internal/provider/provider.go`):

```go
	// BackupNoFiveHour, on a subscription, treats accounts whose known
	// allowance for the model has a budget window (a day or longer: the
	// week) and no short rate-cap window (the 5 hours) as backup under
	// Smart routing, behind accounts with a 5-hour window while those are
	// under 90%, unless the backup account's week is within 24h of reset
	// or has at least 2%/h left (see account_backup.go).
	BackupNoFiveHour bool `json:"backupNoFiveHour,omitempty"`
	// AccountBackup is, for a subscription, each account's manual backup
	// override under Smart routing, by its name in lower case: "backup" to
	// keep it as a backup even with a 5-hour window, "primary" to keep it
	// primary even without one; one not in it follows BackupNoFiveHour
	// (see account_backup.go).
	AccountBackup map[string]string `json:"accountBackup,omitempty"`
```

Propagate `BackupNoFiveHour` and `AccountBackup` across all provider lifecycle
paths:
- `Provider.clone()`: `p.AccountBackup = maps.Clone(p.AccountBackup)`
- `accountSettings(a, pk Provider)`:
  `a.BackupNoFiveHour, a.AccountBackup = pk.BackupNoFiveHour, pk.AccountBackup`
- `normalize(p Provider)`:
  `p.AccountBackup = normalAccountBackup(p.AccountBackup)`
- `renameSettled` (`internal/provider/copilot_accounts.go`):
  `moved = moveKey(p.AccountBackup, from, to) || moved`

In `internal/provider/account_backup.go`:

```go
package provider

import "time"

// Ways an account is classified for Smart backup routing (AccountBackup).
const (
	AccountBackupAuto    = ""
	AccountBackupOn      = "backup"
	AccountBackupPrimary = "primary"
)

// BackupPromotePace and BackupPromoteLead are when a backup account's
// budget window is promoted to the primary tier so what it has left isn't
// lost at its reset: at least 2% of the budget window left per hour until
// it renews, or renewing within 24 hours with quota left.
const (
	BackupPromotePace = 2.0
	BackupPromoteLead = 24 * time.Hour
)

// AccountBackupMode is how the account user of p is classified for Smart
// backup routing: AccountBackupAuto (""), AccountBackupOn ("backup") or
// AccountBackupPrimary ("primary").
func (p Provider) AccountBackupMode(user string) string

// SetAccountBackup sets how the account ref of the subscription id is
// classified for Smart backup routing: AccountBackupAuto (""),
// AccountBackupOn ("backup") or AccountBackupPrimary ("primary").
func SetAccountBackup(id, ref, mode string) error

// ParseAccountBackup reads an account backup mode as the CLI and GUI take
// it: "backup" ("on", "yes"), "primary" ("first"), or "" ("auto",
// "default", "off", "no", "-").
func ParseAccountBackup(s string) (string, error)

// SetBackupNoFiveHour turns on or off treating accounts of the
// subscription id that have a budget window and no short (< 24h) window
// as backup by default.
func SetBackupNoFiveHour(id string, on bool) error

// LacksShortWindow says whether a has at least one rolling budget window
// (budgetSpan, 24h or longer, not a wallet balance and not a partial
// reading) that counts model at now, and no short rate-cap window (< 24h)
// that counts model.
func (a Allowance) LacksShortWindow(model string, now time.Time) bool

// BackupPromoted says whether a backup account with used share, weekly
// pace and budget renewal due at now is promoted to the primary tier so
// its weekly quota isn't wasted before reset: used < SpentShare, due is
// after now, and either pace >= BackupPromotePace or
// due.Sub(now) <= BackupPromoteLead.
func BackupPromoted(used, pace float64, due, now time.Time) bool

// IsBackupAccount says whether the account user of p is a backup seat for
// model at now, given its allowance a and whether a is known.
func (p Provider) IsBackupAccount(user, model string, a Allowance, known bool, now time.Time) bool
```

Exact logic of `LacksShortWindow`:
```go
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
```

Exact logic of `IsBackupAccount`:
```go
func (p Provider) IsBackupAccount(user, model string, a Allowance, known bool, now time.Time) bool {
	if user == "" {
		return false
	}
	switch p.AccountBackupMode(user) {
	case AccountBackupPrimary:
		return false
	case AccountBackupOn:
		if known {
			u, _ := a.For(model, now)
			pc, due := a.Pace(model, now)
			if BackupPromoted(u, pc, due, now) {
				return false
			}
		}
		return true
	default:
		if !p.BackupNoFiveHour || !known || !a.LacksShortWindow(model, now) {
			return false
		}
		u, _ := a.For(model, now)
		pc, due := a.Pace(model, now)
		return !BackupPromoted(u, pc, due, now)
	}
}
```

#### 2. `internal/provider/codex_switch.go` (`NextLogin`)

In `NextLogin(ctx context.Context, agent string)`:
- Look up the subscription's stored `Provider` `p` from `storedFile().Providers`
  (matching `p.ID == agent`).
- When scanning `spares` for an account that is not spent (`!usedPast(q, share,
  now) && !capped(l.User, q)`):
  - Compute `a, ok := routedAllowance(agent, l.User, q, now)` and
    `backup := p.IsBackupAccount(l.User, "", a, ok, now)`.
  - If `!backup`, return `from, l.User, false, true` immediately.
  - If `backup` and `firstBackup == ""`, record `firstBackup = l.User`.
- After scanning `spares`, if `firstBackup != ""`, return
  `from, firstBackup, false, true` before falling back to `best` (`roomLeft`).

#### 3. `internal/routes/routes.go` & `internal/routes/strategies.go`

In `internal/routes/routes.go`, add to `type Seat struct`:
```go
	// Backup: an account kept as a backup behind primary accounts with
	// room (seatTier 0), tried ahead of accounts running low (seatTier 1)
	// or all but used up (seatTier 2), and ordered by Smart among backup
	// accounts (provider.Provider.IsBackupAccount).
	Backup bool
```

In `internal/routes/strategies.go`, in `Smart`:
- At the top of `fine`'s `sort.SliceStable` comparator (before `a.Learns != b.Learns`):
  ```go
  if a.Backup != b.Backup {
      return !a.Backup
  }
  ```
- In `low`'s `sort.SliceStable` comparator:
  ```go
  sort.SliceStable(low, func(i, j int) bool {
      if seats[low[i]].Backup != seats[low[j]].Backup {
          return !seats[low[i]].Backup
      }
      return seats[low[i]].Used < seats[low[j]].Used
  })
  ```

Why this works across every case:
- Primary `fine` (`seatTier == 0`, `!Backup`) comes first, sorted by `Smart`.
- Backup `fine` (`seatTier == 0`, `Backup`) comes second, sorted by `Smart`
  (including `Learns` if manually marked backup and unknown, `unused` by fewest
  `Tokens`, `Soon` weekly renewal, `Lapse`, `left`, listed order).
- Primary `low` (`seatTier == 1`, `90%–97%` of a `< 24h` rate-cap window,
  `!Backup`) comes third, sorted by `Used`.
- Backup `low` (`seatTier == 1`, `Backup` on an account that also has a `< 24h`
  window at `90%–97%`) comes fourth, sorted by `Used`.
- `spent` (`seatTier == 2`, `>= 98%`) comes last, sorted by `spentBefore`.

#### 4. `internal/gateway/routing.go`, `internal/gateway/trace.go` & `internal/gateway/affinity.go`

In `internal/gateway/routing.go`:
- Add `backup map[string]bool` to `type weighing struct` (keyed by `c.rest`).
- In `weighRouted`:
  - Populate `wg.backup = map[string]bool{}`: for each candidate `c` with
    `c.p.Account != nil`, compute:
    `a, ok := ...`
    `if c.p.IsBackupAccount(c.p.Account.User, c.model, a, ok, now) { wg.backup[c.rest] = true }`
  - When constructing `seats[i]` (`routes.Seat`), set `Backup: wg.backup[c.rest]`.

In `internal/gateway/trace.go`:
- Add `Backup bool` (`json:"backup,omitempty"`) to `type Weighed struct`.
- In `weighed(c, p, wg, fallback, from)`:
  `w.Backup = wg.backup[c.rest] && routingOf(p.Routing) == ""`
- In `planLevel` (`internal/gateway/fallback.go`), where `w.Routing = g.Routing`
  is set on group-weighed heads:
  `w.Backup = wg.backup[c.rest] && routingOf(g.Routing) == ""`

In `internal/gateway/affinity.go`:
- Add `backupBehind(order []Weighed, i int) bool`:
  ```go
  // backupBehind says whether order[i] is a backup account (Weighed.Backup)
  // while a primary account leads: Smart put it behind the primary accounts
  // with room, and spreading a new conversation or taking a turn never
  // brings it forward past them.
  func backupBehind(order []Weighed, i int) bool {
      return i > 0 && i < len(order) && order[i].Backup && !order[0].Backup
  }
  ```
- Include `!backupBehind(order, i)` in `readyForNew(order, i)`, `leastHeld`, and
  `after`.
- Do **not** add `backupBehind` to `affine`'s move-away switch: under `Stays:
  Auto`, an active conversation on a backup account with a warm cache reaches
  `default: a.Why = "cache"` and stays on the backup account, while new or
  cold-cache conversations use `order[0]` (the primary account).

#### 5. CLI (`internal/cli/providers_cli.go` & `internal/cli/pool_cli.go`)

- `magpie provider account-backup <id> [account [backup|primary|auto]]`:
  - Subcommand `"account-backup"`, `"account-backups"` in `providerCmd`.
  - Without `account`: lists each account of `<id>` and its backup status
    (`backup`, `primary`, or `auto (backup when no 5-hour window)` / `auto`).
  - With `account` and `mode`: calls `provider.ParseAccountBackup(mode)` and
    `provider.SetAccountBackup(p.ID, account, m)`, then prints the updated
    state.
  - Refuses key-based providers with:
    `"%s has keys, not subscription accounts"`.
- `magpie provider set <id> backup-5h=on|off` (also accepts `backup-no-5h=on|off`):
  - Calls `provider.SetBackupNoFiveHour(p.ID, on)` (refuses key-based providers).
- `magpie provider pool <id> [--backup-5h on|off]`:
  - Adds `--backup-5h` flag to `poolCmd` and prints the `backup without 5h
    limit · on/off` line and any per-account backup mode in `printPool`.

#### 6. GUI (`internal/gui/providers.go`, `internal/gui/assets/acctbackup.js`, `internal/gui/assets/app.js`, `internal/gui/assets/plugins/routing/client.js`, `internal/gui/assets/i18n.js`)

In `internal/gui/providers.go`:
- Add to `providerJSON`:
  ```go
  BackupNoFiveHour bool              `json:"backupNoFiveHour,omitempty"`
  AccountBackup    map[string]string `json:"accountBackup,omitempty"`
  ```
- Populate `BackupNoFiveHour: p.BackupNoFiveHour, AccountBackup: p.AccountBackup`
  in `providerInfoWith`.
- Preserve `in.BackupNoFiveHour = old.BackupNoFiveHour` and
  `in.AccountBackup = old.AccountBackup` on general `save` in `saveProvider`.
- Add `Backup string` (`json:"backup"`) to `req` in `saveProvider` and handle two
  actions:
  - `case "accountbackup"`:
    ```go
    mode, err := provider.ParseAccountBackup(req.Backup)
    if err != nil {
        fail(rw, err)
        return
    }
    if err := provider.SetAccountBackup(in.ID, req.Account, mode); err != nil {
        fail(rw, err)
        return
    }
    ```
  - `case "backupnofivehour"`:
    ```go
    if err := provider.SetBackupNoFiveHour(in.ID, in.BackupNoFiveHour); err != nil {
        fail(rw, err)
        return
    }
    ```

In `internal/gui/assets/acctbackup.js` (lazy-loaded by `app.js` when rendering
subscription accounts so `app.js` and `coreScriptBudget` stay within budget):
- `window.acctBackupFill(p, a, list, ls, quota)`:
  - For each account row in `list` (`row.dataset.accountId`), builds the
    per-account backup pill (`button.abackup`):
    - Reads explicit mode `p.accountBackup?.[user.toLowerCase()] || ""` and
      whether `p.backupNoFiveHour` auto-detects the account from `quota?.[user]`
      (has a window with `span >= 86400_000_000_000` / `>= 24h` and no window
      with `0 < span < 24h`).
    - When explicit (`"backup"` or `"primary"`) or auto-detected backup, adds
      `.set` and places the pill on `.acc-line` before `.grow`; otherwise places
      it inside `.acc-tools` (under `⋯`).
    - Clicking opens `openProtoMenu` with `"auto"` (`Automatic`), `"backup"`
      (`Backup`), and `"primary"` (`Primary`), posting to
      `provider/accountbackup`.
  - When `ls.length > 1 || p.backupNoFiveHour || Object.keys(p.accountBackup || {}).length`,
    appends the `.backup-5h` checkbox row (`"Use accounts without a 5-hour limit
    as backup"`) posting to `provider/backupnofivehour`.

In `internal/gui/assets/plugins/routing/client.js`:
- In `behindWhy(r, w)`:
  - When `x.backup && !f.backup`, return `"backup"`.
- In row status rendering (around line 2926):
  - When `b === "backup"`, render
    `quota(w, "{n} used · kept as backup", "{n} left · kept as backup")`.
- In `asides(r)`:
  - When `x.backup && f0 && !f0.backup && !x.rest && !x.sunk`, push:
    `t("{who} is kept as a backup while primary accounts have quota to spare.", { who: who(x) })`.

In `internal/gui/assets/i18n.js`:
- Add all new UI strings with matching placeholders across `zh-CN`, `zh-TW`,
  `ja`, and `de`.

### Error handling and edge cases

| Case | Behavior |
| --- | --- |
| Default config (`BackupNoFiveHour` false, `AccountBackup` nil) | `IsBackupAccount` is always `false`; zero change to existing routing, traces, or sign-in switching. |
| All accounts in pool are backup (or no primary account is `< 90%`) | Backup accounts in `fine` (`seatTier 0`) are ordered among themselves by `Smart`; `order[0]` is a backup account, so `backupBehind` is `false` and `leastHeld` spreads across them normally. |
| Unknown quota on `auto` account | `LacksShortWindow` requires a known reading (`known == true`), so the account is not marked backup before its first read and `Learns` probes it once as today. |
| Unknown quota on manual `backup` account | `IsBackupAccount` returns `true` even when `!known`, so it stays behind `< 90%` primary accounts; once routing reaches the backup tier, `Learns` orders it first within `fine` backup accounts. |
| Backup account with `< 24h` to weekly reset or `Pace >= 2%/h` | `BackupPromoted` returns `true` (provided `used < 98%` and `due > now`), promoting it to the primary tier so `Smart` spends its remaining weekly quota before reset. |
| Unstarted weekly window (`used == 0`, `Resets` zero) | `due` is zero and `pace` is `100/168 ≈ 0.595 < 2.0`, so `BackupPromoted` is `false` and the fresh backup account stays in backup. |
| Per-model windows | `LacksShortWindow(model, now)` filters windows by `l.applies(model)`, so an account is auto-backup only for models where it has a budget window and no `< 24h` window. |
| Invalid CLI / API mode for `account-backup` | `ParseAccountBackup` returns a descriptive error (`"an account's backup mode is backup, primary or auto, not %q"`); nothing is written. |
| Setting backup on a key-based provider | Refused with `"%s has keys, not subscription accounts"` before writing `providers.json`. |

## Tasks

### Wave 1

**T1 — Provider backup classification, `NextLogin` preference, `routes.Smart` two-tier ordering, gateway trace & affinity, and CLI**
- Goal: Implement account backup classification (`BackupNoFiveHour` +
  `AccountBackup` + `LacksShortWindow` + `BackupPromoted`), `NextLogin`
  primary-first spare selection, `routes.Smart` two-tier backup ordering,
  gateway `weighRouted` / `Weighed.Backup` / `backupBehind`, and CLI commands.
- Owned files:
  - `internal/provider/account_backup.go` (new)
  - `internal/provider/account_backup_test.go` (new)
  - `internal/provider/provider.go`
  - `internal/provider/copilot_accounts.go`
  - `internal/provider/account_settings_test.go`
  - `internal/provider/codex_switch.go`
  - `internal/provider/switch_next_test.go`
  - `internal/routes/routes.go`
  - `internal/routes/strategies.go`
  - `internal/routes/routes_test.go`
  - `internal/gateway/routing.go`
  - `internal/gateway/trace.go`
  - `internal/gateway/fallback.go`
  - `internal/gateway/affinity.go`
  - `internal/gateway/account_backup_test.go` (new)
  - `internal/cli/providers_cli.go`
  - `internal/cli/pool_cli.go`
  - `internal/cli/account_backup_cli_test.go` (new)
- Consumes: existing `provider.Allowance`, `routes.Seat`, `gateway.weighRouted`,
  `gateway.affine`.
- Produces:
  - `provider.AccountBackupAuto`, `provider.AccountBackupOn`,
    `provider.AccountBackupPrimary`, `provider.BackupPromotePace`,
    `provider.BackupPromoteLead`
  - `Provider.BackupNoFiveHour`, `Provider.AccountBackup`,
    `Provider.AccountBackupMode`, `Provider.IsBackupAccount`,
    `provider.SetAccountBackup`, `provider.ParseAccountBackup`,
    `provider.SetBackupNoFiveHour`, `Allowance.LacksShortWindow`,
    `provider.BackupPromoted`
  - `routes.Seat.Backup`, `gateway.Weighed.Backup`
- Dependencies: none.
- Proving command:
  `go test -tags nogui -count=1 -run 'Backup|AccountSettingsKept|NextLogin|Smart' ./internal/provider ./internal/routes ./internal/gateway ./internal/cli`

### Wave 2 (after Wave 1)

**T2 — GUI controls, Routing page explanations, translations, and Playwright tests**
- Goal: Expose `backupNoFiveHour` and `accountBackup` in `/api/providers` and
  `/api/provider/{action}`, add lazy-loaded `acctbackup.js` for the provider
  editor's per-account pill and provider checkbox, update the Routing page
  (`client.js`) to show `"kept as backup"` on rows and in `asides`, and add
  translations (`zh-CN`, `zh-TW`, `ja`, `de`) and Playwright tests.
- Owned files:
  - `internal/gui/providers.go`
  - `internal/gui/account_backup_test.go` (new)
  - `internal/gui/assets/app.js`
  - `internal/gui/assets/acctbackup.js` (new)
  - `internal/gui/assets/plugins/routing/client.js`
  - `internal/gui/assets/i18n.js`
  - `internal/gui/tests/account-backup.test.cjs` (new)
- Consumes: `provider.SetAccountBackup`, `provider.ParseAccountBackup`,
  `provider.SetBackupNoFiveHour`, `gateway.Weighed.Backup` from T1.
- Produces: `/api/provider/accountbackup`, `/api/provider/backupnofivehour`,
  `acctbackup.js` UI controls, Routing page backup status and explanations.
- Dependencies: T1.
- Proving command:
  `go test -tags nogui -count=1 -run 'AccountBackup|CoreScriptBudget' ./internal/gui && node --test internal/gui/tests/account-backup.test.cjs internal/gui/tests/i18n-keys.test.cjs && BROWSER=webkit node --test internal/gui/tests/account-backup.test.cjs`

**T3 — Subsystem and user reference documentation**
- Goal: Update `docs/subsystems/gateway-routing.md`,
  `docs/subsystems/providers-accounts.md`, and `docs/reference.md` to document
  two-tier Smart backup routing, `BackupNoFiveHour`, `AccountBackup`, and the
  weekly expiry promotion rule.
- Owned files:
  - `docs/subsystems/gateway-routing.md`
  - `docs/subsystems/providers-accounts.md`
  - `docs/reference.md`
- Consumes: symbols and behavior from T1 and T2.
- Produces: updated documentation with verified source links.
- Dependencies: T1 (disjoint from T2, can run in parallel with T2 in Wave 2).
- Proving command:
  `git diff --check -- docs/subsystems/gateway-routing.md docs/subsystems/providers-accounts.md docs/reference.md`

## Test and verification

All tests and verification commands must run inside a temporary `HOME` with Go's
caches pinned (`docs/subsystems/provider-plugins.md#verification`), never
touching `~/.config/magpie`, real agent configs, or a running magpie instance:

```sh
set -o pipefail
GOPATH=$(go env GOPATH) GOMODCACHE=$(go env GOMODCACHE) GOCACHE=$(go env GOCACHE)
export GOPATH GOMODCACHE GOCACHE
h=$(mktemp -d); export HOME=$h XDG_CONFIG_HOME=$h/.config XDG_CACHE_HOME=$h/.cache XDG_DATA_HOME=$h/.local/share XDG_STATE_HOME=$h/.local/state
unset MAGPIE_ADDR
# … run checks …
chmod -R u+w "$h"; rm -rf "$h"
```

Required verification checks:

1. **Regression check (red on old code):**
   - Temporarily bypass `a.Backup != b.Backup` in `routes.Smart` and
     `IsBackupAccount` in `provider`: verify `TestSmartBackupAccounts`,
     `TestGatewayBackupRouting`, and `TestNextLoginPrefersPrimaryBeforeBackup`
     fail with a real `FAIL`, then restore and verify they pass.
2. **Formatting and cross-platform vet & build:**
   - `gofmt -l $(git diff --name-only main -- '*.go')` (prints nothing)
   - `go vet ./...`
   - `GOOS=linux go vet -tags nogui ./...`
   - `GOOS=windows go vet ./...`
   - `go build ./...`
   - `GOOS=linux go build -tags nogui .`
   - `GOOS=windows go build .`
3. **Targeted unit, race, and full Go suite:**
   - `go test -race -count=5 -run 'Backup|AccountSettingsKept|NextLogin|Smart' ./internal/provider ./internal/routes ./internal/gateway ./internal/cli ./internal/gui`
   - `go test -tags nogui -timeout 45m ./...` (one suite at a time on the host).
4. **GUI & i18n verification in both engines:**
   - `node --test internal/gui/tests/account-backup.test.cjs internal/gui/tests/i18n-keys.test.cjs internal/gui/tests/gui-zh-tw.test.cjs internal/gui/tests/gui-ja.test.cjs internal/gui/tests/gui-de.test.cjs`
   - `BROWSER=webkit node --test internal/gui/tests/account-backup.test.cjs`
   - `make test-ui` (Chromium and WebKit).

## E2E journey

> **Sandbox E2E exemption:** Per repository `AGENTS.md` ("This project never
> requires end-to-end sandbox tests, including when using the `e2e-feature`
> skill. Treat the skill's sandbox E2E phase as **not needed** for this
> repository and skip it."), the standalone sandbox E2E phase is **not needed**
> and is skipped. The user journeys below are verified through Go gateway
> integration tests (`httptest` upstream + `gateway.New`) and Playwright browser
> tests (`account-backup.test.cjs` in Chromium and WebKit across all locales).

| # | Journey | Expected behavior |
| --- | --- | --- |
| J1 | Pool of 2 Standard accounts (`5h` + `7d`) and 2 Premium accounts (`7d` only) with `BackupNoFiveHour` on; `prem-1` is 0% used and renews in 3d (`Pace < 2%/h`), `std-1` is 25% used on 5h and renews in 5d | New conversation routes to `std-1` (or `std-2` by `Smart` among Standard); `prem-1` and `prem-2` sit behind with `Weighed.Backup = true` and Routing page status `"0% used · kept as backup"`. |
| J2 | Both Standard accounts reach `>= 90%` on their 5-hour window (or one is at 92% and the other is held at its 80% usage cap or resting); `prem-1` is at 10% (renews in 4d) and `prem-2` is at 5% (renews in 3d) | New conversation spills over to `prem-2` (`Smart` picks the backup account whose week renews soonest among usable backups), ahead of the 92% Standard account. |
| J3 | Active conversation on `prem-2` (warm cache `< 5m`, `>= 1,024` tokens); `std-1`'s 5-hour window resets to `0%` | Under `Stays: Auto`, the next turn of the existing conversation stays on `prem-2` (`Why: "cache"`), while a new conversation started at the same time routes to `std-1`. |
| J4 | `prem-1` has `60%` of its week left (`40%` used) and its weekly reset is `20h` away (`<= 24h`, `Pace = 3.0%/h >= 2.0%/h`) while `std-1` renews in `5d` | `BackupPromoted` promotes `prem-1` (`Weighed.Backup = false`); `Smart` routes new conversations to `prem-1` ahead of `std-1` so `prem-1`'s remaining weekly quota is not wasted at reset. |
| J5 | Manual per-account override in GUI & CLI (`account-backup`) | Setting `std-2` to `backup` puts it in the backup tier even though it has a 5-hour window; setting `prem-1` to `primary` keeps it in the primary tier even with `BackupNoFiveHour` on. |
| J6 | Local sign-in switching (`NextLogin`) when active `std-1` reaches 98% | With spares `[prem-1 (backup, 10%), std-2 (primary, 20%)]`, `NextLogin` switches the local sign-in to `std-2` first, and only switches to `prem-1` once `std-2` is also `>= 98%` or capped. |
