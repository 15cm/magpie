# Premium GPT weekly usage tiers for primary-pool admission

Status: **Approved by the user** via “Approve plan and hand off.” Ready for implementation handoff to the orchestrator. This design tab does not implement or commit.

Workspace: `/home/sinkerine/.paseo/worktrees/33uo4m1c/sordid-bird` (`wks_069ad8f9d92c81ec`), branch `sordid-bird`. Inspected base: `ea728dcc9416d2f91af0922ed2eb6a152502ca75`; Paseo base ref: `refs/remotes/origin/main` (`main`). Do not edit `/home/sinkerine/dev/magpie`. This document is the only design deliverable; do not implement or commit in the design phase.

## 1. Goal and acceptance criteria

Allow an account already classified as a Smart backup to compete in the primary pool when its reported seven-day usage and time until reset meet a configurable tier. This is a routing preference, not permission to use an account otherwise barred by its ordinary eligibility, caps, rests or model restrictions. An account that fails promotion remains available as a backup.

The user selected these defaults, configurable per subscription provider:

| Maximum weekly usage **used** | Maximum time **until** reset |
| --- | --- |
| 20% | 7 days |
| 40% | 5 days |
| 60% | 3 days |
| 80% | 1 day |

For one window, match `(used <= maxUsed) AND (0 < reset - now <= maxResetDays * 24h)`. A window may match **any** tier (OR). Evaluate every weekly window that applies to the requested model, including its shared window: every such window must pass, possibly through different tiers. A day is an elapsed 24 hours, not a calendar date or rounded day count.

### Decisions and evidence

D1–D6 record explicit user decisions from this design conversation. D7–D9 are concrete engineering choices approved with the whole plan. Evidence labels below are planned verification, not claims that implementation tests have run.

| ID | Decision | Decision evidence | Implementation evidence |
| --- | --- | --- | --- |
| D1 | Compare percentage **used <= X** and future reset **within <= N days**; OR across tiers; failure leaves a backup. | User: “use the suggested rule but remove previous rule no longer needed.” | E1, E3 |
| D2 | Remove the current pace >= 2 percentage-points/hour and reset-within-24h promotion rules completely, including as fallback for absent settings. Keep pace calculations used by other routing strategies. | User: “remove today rules.” | E0, E1, E3, R1 |
| D3 | Configurable defaults are 20%/7d, 40%/5d, 60%/3d, 80%/1d. | User: “use the proposal but make it configurable.” | E0–E3, E6, R1 |
| D4 | Per-provider list, shared across that provider's subscription accounts; expose GUI editor, CLI and provider API. | User selected “Per provider: GUI + CLI + API.” | E2, E4–E6, R1 |
| D5 | Absent setting uses the defaults for existing and new providers. An explicitly empty list disables promotion. No rewrite on startup. | User selected “Use the approved tiers by default”; the option explicitly described empty-list disabling. | E1, E2, E4, E5 |
| D6 | Apply only after existing automatic or explicit Backup classification, never to ordinary non-backups or Primary overrides. Require every applicable weekly window to pass independently; other models' windows do not participate. | User clarified “only the account that enable the backup rule,” then selected “Yes: automatic and explicit backups” with the conservative weekly-window rule in the option/context. | E1, E3, E5 |
| D7 | Store nullable `backupUsageTiers` with numeric `maxUsed` and `maxResetDays`; accept fractional percentages/days; validate rather than clamp. Range: 0..100 used and >0..7 elapsed days, representable as at least one nanosecond. | Engineering proposal approved with the whole plan. | E1, E2, E4, R1 |
| D8 | A weekly window is exactly `Span == 7*24*time.Hour`; incomplete, wallet, missing/future-unknown or expired weekly readings cannot earn promotion. Keep the existing model-wide <98% spent guard. Unknown automatic classification and ordinary reset handling stay as they are today. | Engineering proposal implementing weekly-only scope and preserving existing safety gates; approved with the whole plan. | E1, E3, E5 |
| D9 | Use one CLI subcommand and one dedicated provider API action. General provider Save preserves this setting, as it currently preserves account backup controls; it is not a second way to edit tiers. | Engineering proposal following existing editor lifecycle; approved with the whole plan. | E2, E4, E6, R1 |

### Observable acceptance criteria

Every criterion names its entry paths and evidence. Tests and source reviews must report their actual results at the implemented tree.

- **AC1 — Admission and boundaries.** For direct provider Smart routing and pooled Smart routing groups, both automatic and explicitly configured backups compete as primary only when all applicable weekly windows match a tier. Test equality and just-over values for every default and custom tier, OR across tiers, independent usage/time conjunction, fractional settings, list reordering, empty list, and the hard 98% guard even with a 100% configured tier. Evidence: E0, E1, E3.
- **AC2 — Removed shortcuts and deliberate compatibility change.** Through `IsBackupAccount`, direct/group gateway routing and local Codex/Claude `NextLogin`, an account failing all tiers is not promoted just because pace is >=2 or reset is within 24h. Absent settings use the four defaults; existing and new providers follow the same policy. For example, 85% used with 20h remaining stays backup; 10% used with 6d remaining is promoted. Other uses of pace remain unchanged. Evidence: E0, E1, E3, E5, R1.
- **AC3 — Settings survive lifecycle.** CLI and dedicated API updates, provider `Save`/`Find`, subscription `accountSettings`/`withPicks`, `All`/`AlsoOn` account views, serialization/reload and unrelated provider saves preserve absent/default, custom and explicitly empty states distinctly. Copies of a Provider do not share the tier slice or pointer. Provider/account rename does not reset a provider's policy. Evidence: E2, E4 and R1 for editor Save and DTO wiring.
- **AC4 — Validation and failure atomicity.** CLI, the dedicated provider action and `provider.Save` reject invalid ranges, nonfinite numbers, invalid/null rows, absent row members, wrong types, malformed list syntax and unsupported key-based providers without changing `providers.json`. Dedicated API requests omitting the property fail; explicit null resets to default. Invalid on-disk numeric policy never silently falls back to permissive defaults. A broken/unreadable providers file retains existing shared-reader protections and cannot be overwritten by the setter. Evidence: E1, E2, E4; R1 for HTTP status/error wiring. Existing full business tests cover shared-reader fallback.
- **AC5 — Quota scope and missing data.** Through `IsBackupAccount`, direct/group routing and `NextLogin`'s existing model-less view, promotion needs at least one applicable weekly window and every applicable weekly window must qualify independently. Shared + model-specific windows cannot mix one window's low usage with another's near reset. Missing, exactly-now, past, partial, wallet or nonfinite weekly readings do not grant promotion. Nonweekly windows and windows for another model cannot earn promotion. Complete unknown-reset weeks still support existing backup detection, but cannot earn promotion. Automatic unknown allowances still follow today's primary learning path; explicit unknown backups stay backup. Evidence: E1, E3, E5.
- **AC6 — Ordinary routing contracts.** Through direct/group gateway requests, ordinary primary accounts and explicit Primary overrides keep their classification. Smart's within-tier ordering, 90–97% short-window treatment, weekly-low treatment, 98% spent handling, caps, per-window caps, rests, account on/off/model/key restrictions, credits-first and provider Use first remain effective. Non-Smart modes keep their ordering and omit Smart's backup mark. Traces mark a current backup and clear the mark after promotion, without changing saved account mode. Evidence: E3 and the full business suite; R1 for unchanged restrictions and trace contracts.
- **AC7 — Affinity and runtime changes.** Through direct/group requests, Stays Auto preserves the existing warm-cache/within-turn behavior when a backup promotes or demotes, when a policy is edited, and when a Standard account recovers. New/cold conversations follow current Smart classification; spent/capped/unavailable accounts retain existing behavior. Stays Off, Within a turn, Session and Strict retain their current guarantees and spreading rules. ChatGPT's existing one-hour cache lifetime stays intact. Evidence: E3; existing `TestAffinityKeepsAnHourCachePastFiveMinutes`, affinity and cap coverage in the full suite.
- **AC8 — Local account switching.** For Codex and Claude spare selection, `NextLogin` uses the same tier policy as gateway classification, while keeping readable-spare, quota, held, cap and fallback-to-most-room rules. `NextInOrder` stays an explicit rotation. Model-less selection must not borrow model-specific quota windows. Evidence: E5 and existing switch tests in the full suite.
- **AC9 — Configuration surfaces.** The subscription provider editor allows viewing/editing multiple rows, adding/removing rows, saving, disabling promotion, and restoring defaults, with clear Used % and Reset within days labels. CLI can show/set/off/default, and pool output shows the policy as configured, not a live per-model result. `/api/providers` provides saved policy and server defaults; the dedicated mutation action validates and saves through the same setter. Key-based providers have no tier editor and mutations are refused. Evidence: E4 for CLI/provider contracts; R1 for GUI/API source review. **GUI, browser and HTTP tests in `internal/gui` are excluded by repository policy; editor/API runtime behavior is not claimed as tested.**
- **AC10 — Documentation and verification record.** DOC1–DOC3 are updated with exact policy, entry points, examples, compatibility change and verification limits. Run V1–V6 as applicable with isolated state and retain full logs. No GUI-related tests or sandbox E2E are prescribed. Evidence: E6 and verification log.

Required documentation items:

| ID | Required update | Evidence mapping |
| --- | --- | --- |
| DOC1 | `docs/subsystems/providers-accounts.md`: policy source, weekly/model aggregation, new types/setter/CLI, default/empty/invalid states, persistence and `NextLogin`; remove old promotion constants/contract. | E6 + E1/E2/E4/E5; AC1–AC5, AC8 |
| DOC2 | `docs/subsystems/gateway-routing.md`: tier admission changes current Backup classification only; direct/group and model keying; affinity follows current source, including ChatGPT's one-hour cache exception. | E6 + E3; AC1/AC2/AC5–AC7 |
| DOC3 | `docs/reference.md`, Premium backup and CLI usage: four defaults, used/time comparisons, OR and multi-window behavior, editor/CLI/API usage, disable/default examples, removal of old behavior for existing users, unknown quota and cache caveats. New CLI help stays consistent. | E6 + E4 and R1; AC2/AC9/AC10 |

## 2. Design

### Verified current contracts

Sources were inspected at the base above, after reading `AGENTS.md`, `LESSONS.md`, `docs/code-standards.md`, the subsystem index and the owning gateway-routing/providers-accounts references. The old `docs/features/premium-backup-plan.md` is historical, not the runtime specification.

- [`internal/provider/account_backup.go`](../../internal/provider/account_backup.go): `IsBackupAccount(user, model string, a Allowance, known bool, now time.Time) bool` first applies manual Primary/Backup overrides or automatic `BackupNoFiveHour` + `LacksShortWindow`. Auto detection requires a complete non-wallet budget >=24h with no positive short window. It then filters budget windows and calls `BackupPromoted(used, pace float64, due, now time.Time) bool`. Current promotion is model-wide usage <98%, future budget reset and (pace >=2 OR reset <=24h). Unknown auto is primary; unknown explicit Backup is backup.
- [`internal/provider/routing.go`](../../internal/provider/routing.go): `Limit.applies` provides model matching, `Allowance.For` supplies maximum applicable usage and treats expired readings as empty; `Pace` aggregates budget pace and can substitute automatic-reset/credit-expiry deadlines. Tiers must read each weekly `Limit.Resets` directly, not infer reset timing from `Pace` or combine unrelated windows.
- [`internal/gateway/routing.go`](../../internal/gateway/routing.go): `weighRouted` asks each candidate's Provider to classify its own model. Both `weighing.backup` and `lefts` use `allowanceKey{rest, model}`. `Seat.Backup` affects Smart only. Other allowance calculations remain independent.
- [`internal/routes/strategies.go`](../../internal/routes/strategies.go): Smart ranks usable primaries before usable backups, then short-window-low accounts, then spent accounts, while applying its existing rules within each tier. Promotion means competing under those rules, not guaranteed first choice.
- [`internal/gateway/fallback.go`](../../internal/gateway/fallback.go): `planLevel` weighs pooled group heads and carries the candidate/model-specific backup value into each trace row. It must continue using the candidate provider's policy, not a synthetic group Provider's defaults.
- [`internal/gateway/affinity.go`](../../internal/gateway/affinity.go): affinity runs after weighing; `backupBehind` prevents inappropriate spreading while a primary leads. `coldAfter` currently allows one hour for ChatGPT accounts (`chatGPTCold`), five minutes for the general case, plus existing longer-cache handling. Preserve these actual contracts; the backup prose's blanket five-minute statement is stale. Preserve group stand-in recovery as well as cache retention.
- [`internal/provider/codex_switch.go`](../../internal/provider/codex_switch.go): `NextLogin` reads the saved Provider and calls `IsBackupAccount(..., model="", ...)` after its ordinary spare checks. Both Codex and Claude consume this preference. `NextInOrder` does not.
- [`internal/provider/provider.go`](../../internal/provider/provider.go): `accountSettings` is the shared persistence/view-copy list; `clone` owns mutable copies. `storedFile` shares immutable parsed generations, `read` preserves errors/last-good recovery and `store` atomically replaces the file and calls `catalog.Touched`. Never mutate shared tiers in place.
- [`internal/gui/providers.go`](../../internal/gui/providers.go) and [`internal/gui/assets/acctbackup.js`](../../internal/gui/assets/acctbackup.js): existing backup actions save independently from the main editor, which preserves backup settings. The account quota hint describes configured automatic classification, not live model-specific promotion. Keep that distinction.

### Settings representation and interfaces

Add to `internal/provider/account_backup.go`:

```go
type BackupUsageTier struct {
    MaxUsed      float64 `json:"maxUsed"`
    MaxResetDays float64 `json:"maxResetDays"`
}

func DefaultBackupUsageTiers() []BackupUsageTier
func (p Provider) BackupPromotionTiers() []BackupUsageTier
func CheckBackupUsageTiers(tiers []BackupUsageTier) error
func ParseBackupUsageTiers(s string) (*[]BackupUsageTier, error)
func SetBackupUsageTiers(id string, tiers *[]BackupUsageTier) error
func (t *BackupUsageTier) UnmarshalJSON(data []byte) error

// Replaces the old pace-based helper; update all callers/tests.
func BackupPromoted(used float64, due, now time.Time, tiers []BackupUsageTier) bool

// Package-private quota aggregation used by IsBackupAccount.
func (a Allowance) backupUsagePromoted(model string, now time.Time, tiers []BackupUsageTier) bool
```

Add to `Provider`:

```go
BackupUsageTiers *[]BackupUsageTier `json:"backupUsageTiers,omitempty"`
```

A nil pointer (absent or JSON null) means use the server's four defaults. A nonnil pointer to an empty slice means promotion disabled and must serialize as `[]`, never be lost through `omitempty` or become null. A nonnil nonempty list is the custom policy. Canonicalize any Go pointer-to-nil-slice to pointer-to-empty-slice when copying for Save/setter, so empty intent survives JSON. `DefaultBackupUsageTiers` and `BackupPromotionTiers` return independent slices; no exported mutable global defaults. Invalid nonnil settings yield no effective promotion tiers at runtime, not the defaults.

Example custom persistence:

```json
{
  "backupUsageTiers": [
    {"maxUsed": 20, "maxResetDays": 7},
    {"maxUsed": 40, "maxResetDays": 5},
    {"maxUsed": 60, "maxResetDays": 3},
    {"maxUsed": 80, "maxResetDays": 1}
  ]
}
```

List order has no routing priority; matching is OR. Preserve row order for editing. Duplicate or dominated rows are allowed because they do not change OR semantics; do not silently delete or reorder user settings.

Validation is shared by the setter, `Save` and effective-policy resolution: finite `maxUsed` in [0,100], finite `maxResetDays` in (0,7], converting to at least 1ns. Fractional days are supported and converted once per comparison to `time.Duration(days * float64(24*time.Hour))`, truncated to nanoseconds; no whole-day rounding. The unchanged <98% model-wide guard still applies even to a tier configured above it. Each JSON row must be a nonnull object with both numeric members present; the row unmarshaller rejects missing/null/wrong-type/unknown members and trailing content rather than turning an omitted threshold into zero. Range validation remains in `CheckBackupUsageTiers` so syntactically valid out-of-range disk settings can be shown and repaired.

`ParseBackupUsageTiers` accepts `default` => nil, `off` => explicit empty, or comma-separated `maxUsed:maxResetDays` numeric pairs. Whitespace around tokens is accepted. Empty text, missing pieces, trailing comma, unexpected units/flags, NaN/Inf and invalid numbers fail before writing. Do not add aliases or additional set/pool mutation paths in this change.

### Classification algorithm and quota boundaries

Keep the `IsBackupAccount` signature and its classification preamble unchanged. After it establishes backup status:

1. If the allowance is unknown, stay backup. Otherwise obtain effective tiers; empty or invalid means stay backup.
2. Use `a.For(model, now)` for the existing maximum applicable usage guard; require a finite value below `SpentShare` (98). A short/nonweekly window cannot earn promotion, but still contributes to ordinary spent/cap protection as it does today.
3. Lower-case the requested model before `Limit.applies`, matching the existing convention. Scan only applicable windows with `Span == weekSpan` (exactly 168h). For a model-less local switch, preserve current `applies("")`: do not aggregate named model-specific windows into shared quota.
4. Require at least one applicable weekly window. Every applicable weekly window must be complete (`!partial`), non-wallet (`Balance == nil`), have finite Used in [0,100], and have a strictly future `Resets`. A partial/wallet weekly window cannot be skipped to let a different complete weekly window grant admission.
5. For each weekly window, `BackupPromoted(l.Used, l.Resets, now, tiers)` must return true. It validates actual usage and a future reset and checks OR across tiers using paired used/reset values. Never take usage from one window and reset from another.
6. When every weekly window passes, return false from `IsBackupAccount` (currently primary). Otherwise return true (currently backup).

Expired weekly readings do not promote even though `Allowance.For` treats their ordinary usage as reset. Wait for an existing quota refresh/response header to provide the next future reset. A week not started with no reset stays backup. Nonweekly windows cannot promote; automatic classification of nonweekly budget accounts still uses existing `LacksShortWindow`. Removing the old rule therefore also removes its former promotion of nonweekly budgets. The policy is quota-based, with no GPT plan-name guesses and no synthetic windows.

Delete `BackupPromotePace` and `BackupPromoteLead`. Replace the old `BackupPromoted` signature and its callers/tests. Leave `Allowance.Pace`, `Restarts`, credit expiry and routing pace data untouched: tiers use the vendor's weekly reset, not those alternative deadlines.

### Persistence and lifecycle

`SetBackupUsageTiers` finds a real subscription provider, rejects a key-based provider, clones/canonicalizes the supplied setting, validates it and calls existing `Save`. `Save` also validates nonnil tier configuration before normalization/storage so direct business callers cannot persist invalid settings. Integrate the subscription/key check with Save's existing subscription-account detection; a subscription's stored picks intentionally have no credential Account object, and must remain valid on reload/unrelated save.

Add the field to `accountSettings` in both effective directions, deep-copy its pointer and slice in `clone`, and keep nil/default versus explicit empty through normalization, account views and JSON round-trip. Provider-level configuration does not need account-name map entries, rename migration or a new settings file. Account renames leave it alone; provider renames carry the existing Provider value. Existing generic export/restore/sync of Provider values carries this nonsecret setting; do not add credential copying or a new synchronization mechanism.

No startup migration writes the four defaults into older providers. Existing users without this field deliberately switch from the old promotion formula to these defaults on upgrade; document that change. An old binary ignores the new field and returns to its old rule, and may drop an unknown field on a later save; do not promise downgrade preservation.

Retain the shared reader's unreadable/last-good behavior. A syntactically valid but out-of-range list is preserved for display, earns no promotion, and is rejected by an attempted unchanged Save until repaired. A syntactically malformed field follows existing providers-file decode/fallback behavior. Neither case becomes an empty catalog or an automatic write of defaults from this feature.

Updates use existing `store`/`catalog.Touched` invalidation. Subsequent provider lookups/requests and local switch looks consume the new setting; in-flight requests may complete with their captured Provider. No timers, quota fetches, cache flushes, credential changes or persistent per-account admission state are added. Time-based transitions are computed on each look with one `now` for the classification. Policy saves do not erase affinity or usage caches.

### Entry points and observable behavior

**CLI:** add `magpie provider backup-usage-tiers <id> [default|off|used:days,...]` through the existing provider dispatcher/help, implemented with one command handler (a small dedicated CLI file is appropriate). No argument prints effective rows and the mode (`default`, `custom`, `off`); a mutation prints the resulting saved policy. Usage must define Used %, elapsed reset days, inclusive comparisons, OR and empty-list semantics. Reject extra positional arguments and unknown flags. `magpie provider pool <id>` adds a configured-policy summary with a pointer to this command; existing `--backup-5h` and `provider set backup-5h=` remain detection controls only.

Examples:

```sh
magpie provider backup-usage-tiers codex
magpie provider backup-usage-tiers codex '20:7,40:5,60:3,80:1'
magpie provider backup-usage-tiers codex '15.5:6,55:2.5'
magpie provider backup-usage-tiers codex off
magpie provider backup-usage-tiers codex default
```

**Provider API:** extend the existing subscription provider DTO on `GET /api/providers` with saved `backupUsageTiers` (null/absent = default, [] = off) and `backupUsageTiersDefault` containing the server's current default rows. Do not duplicate default values in JavaScript. Preserve invalid-but-decodable saved numeric values so the editor can repair them; never describe such a policy as active defaults.

Add `POST /api/provider/backupusagetiers` with `{ "id": "codex", "backupUsageTiers": [...] }`, [] for off or null for defaults. Capture that property with `json.RawMessage` in the existing action request to distinguish omission (400) from explicit null (default). Decode rows through the strict shared type and call `SetBackupUsageTiers`. Return the existing action success/reload shape; errors use existing `fail` (400 with JSON `error`). Keep the existing authentication/mutation guards. The general `provider/save` preserves the saved tier value alongside `BackupNoFiveHour`/`AccountBackup`, even if an old/stale editor body omits or sends the property. Only the dedicated action edits tiers.

**GUI:** extend `acctbackup.js` beside the current backup controls with a compact provider-level “Weekly promotion tiers” editor. Show it for subscription providers, including one-account providers and explicit Backup users with automatic detection off. Explain that it affects accounts configured as backups, that passing promotes into primary competition, and that all applicable weekly windows must qualify. Use two numeric inputs per row (“Weekly usage used, at most (%)”, “Reset within (days)”), add/remove row actions, an explicit Save, “Disable promotion” and “Use defaults”. Removing every row + Save means off; Use defaults posts null. Start from `backupUsageTiers ?? backupUsageTiersDefault` and visibly indicate Default/Custom/Off. Retain unsaved input and show the server's error on failure; reload policy only after success. Do not use `Number("")` to turn an empty input into a valid 0. Labels/help/errors go through existing translation conventions, with translations updated; no native select or unrelated layout redesign.

Keep account Automatic/Backup/Primary controls and the configured-auto hint intact. Live classification continues to be shown by existing per-model routing traces: current backups have `Backup: true` under Smart; promoted accounts do not. No new “eligible” badge guesses a single live state across all models. No TUI editor, global policy, per-account tier overrides, new trace schema or new gateway API is in scope.

### Runtime callers and unchanged interfaces

The gateway continues calling `Provider.IsBackupAccount` in `weighRouted`, using each candidate's own policy and `c.model`. `planLevel` continues carrying the matching account/model's backup value. `NextLogin` keeps calling that same method with its existing model-less allowance. There must be no second copy of the tier algorithm in gateway, GUI, CLI or switching.

`allowanceKey`, `routes.Seat`, `Weighed`, `LacksShortWindow`, normal quota/cap eligibility, model restrictions, sorting within Smart tiers and affinity persistence retain their interfaces. Business tests must demonstrate these unchanged contracts where defaults alter old fixture assumptions.

## 3. Tasks

One task, **T1**, owns implementation, wiring, tests and documentation. No layer split: the same small policy schema touches Save/view copies, both runtime consumers and editing surfaces; separate workers would contend on `provider.go`, `account_backup.go`, `providers_cli.go` and `providers.go` while needing the same quota context.

Graph: **user approval → T1 → implementation review/verification record → orchestrator completion gate**. There are no parallel producer tasks and no external implementation dependencies. Do not launch T1 from this design tab.

### T1 — Add configurable weekly tier admission end to end

Owned files:

- `internal/provider/account_backup.go`, `provider.go`, `account_backup_test.go`, `account_settings_test.go`, `switch_next_test.go`; new `account_backup_tiers_regression_test.go` containing only baseline-compatible regression tests. Add other provider tests only where they exercise an actual lifecycle path above.
- `internal/cli/providers_cli.go`, `pool_cli.go`; new `backup_usage_tiers_cli.go` and `backup_usage_tiers_cli_test.go`; maintain `account_backup_cli_test.go` expectations where its output changes.
- `internal/gateway/account_backup_test.go` for direct/group/model/affinity integration. Production `routing.go`, `fallback.go`, `affinity.go` are interface consumers expected to remain unchanged; change one only if verified wiring requires it, and keep account/model keys intact. `internal/routes/strategies.go` is consumed unchanged.
- `internal/gui/providers.go`, `internal/gui/assets/acctbackup.js`, `internal/gui/assets/i18n.js` and existing language resources required by translation conventions. **Do not add or run GUI tests.**
- `docs/subsystems/providers-accounts.md`, `docs/subsystems/gateway-routing.md`, `docs/reference.md`, and this plan's status/evidence record as needed. Historical backup plan is not rewritten as current truth.

Consumed interfaces: `Limit.applies`, `Allowance.For`, `LacksShortWindow`, Provider Save/Find/clone/accountSettings, existing CLI provider dispatcher, provider DTO/action and `accountAction`, `weighRouted` and group trace propagation, `NextLogin`, existing `backupPool`/sign-in/testenv fixtures.

Produced interfaces: `BackupUsageTier`, nullable `Provider.BackupUsageTiers`, default/accessor/validation/parser/setter, replacement `BackupPromoted`, private weekly aggregator, CLI command, provider DTO fields and dedicated action described in section 2. Keep all signatures and JSON names consistent across surfaces.

Order within T1: add baseline-compatible regression; implement shared policy/persistence and consumer behavior; wire CLI/API/editor; adjust existing fixture policies; add behavioral and validation coverage; update DOC1–DOC3; run verification and self-review. This is one worker's sequential checklist, not separate tasks.

Exact focused unit/integration command, inside the isolation wrapper in section 4:

```sh
go test -tags nogui -count=1 -timeout 45m -run 'Backup|AccountSettingsKept|NextLogin|NextInOrder' ./internal/provider ./internal/gateway ./internal/cli ./internal/routes
```

Conflicts: one writer in this worktree. All new tests modifying environment, provider globals, allowance hooks, routing/affinity state or stdout must remain serial; do not add `t.Parallel`. Do not run simultaneous full suites or race runs on this host. Build outputs and logs belong to the worker's temporary verification directory; never overwrite a live `magpie` binary or clear shared Go caches.

## 4. Test and verification

### Evidence inventory

These are proposed test names/files for implementation, except explicitly marked existing. GUI/runtime API coverage is intentionally limited by repository policy.

| Evidence | Required behavioral coverage and location |
| --- | --- |
| **E0** | New `TestBackupUsageDefaultRegression` in `internal/provider/account_backup_tiers_regression_test.go`, using only pre-existing Provider/Allowance/IsBackupAccount APIs. Table: default 10%/6d promotes (old code stays backup), default 85%/20h stays backup (old code promotes). Run the same file against base and implementation; baseline must produce named assertion FAIL, not compile failure. |
| **E1** | `TestBackupUsageTierBoundaries`, `TestBackupUsageWeeklyWindows`, `TestBackupUsagePolicyStates`, and updated `TestAccountBackupClassification` in provider tests. Exact equal/just-below/just-above thresholds for each default and custom row; time equality/+1ns; user-used vs remaining; AND/OR; fractions and order independence; <98 guard; multiple shared/model windows with their own paired values; no applicable week; expired/equal-now/unknown reset; partial/wallet/short/daily/monthly/model-mismatch; invalid quota usage including NaN/Inf; unknown automatic/manual; Primary override; auto detection off. Old-promotion-only examples stay backup; a low-used week too far from a tier's time limit stays backup. |
| **E2** | `TestBackupUsageTiersSettings`, `TestBackupUsageTiersJSON`, extended existing `TestAccountSettingsKept`/rename tests: absent/null, custom, explicit [], pointer-to-nil canonicalization, clone independence, first and secondary account views, reload, unrelated save, rename; strict JSON shape, range/finite validation, parser errors, unsupported keys; byte-identical provider file after failure; unreadable file refused; invalid numeric disk list gives no promotion without mutation. Test caller mutations after setter/accessor do not mutate shared state. |
| **E3** | Extend existing `TestGatewayBackupRouting`, `TestGatewayBackupWarmCacheAndRecovery`, `TestGatewayBackupSessionSpreading`, `TestGatewayBackupModelWindows`, `TestGatewayBackupGroupModelWindows`; add `TestGatewayBackupUsageTiers` and `TestGatewayBackupUsagePolicyChange`. Use production `Server.Front()` with httptest vendor and saved fake accounts for direct provider and pooled group variants. Assert selected upstream account and trace Backup flags for matching/failing/default/custom/off policies, live edit, reset/fresh reading, shared+model windows and no cross-model map leakage. Preserve cap/window-cap/rest/recovery, non-Smart, Auto/Off/Within-turn/Session/Strict behavior and saved modes. Existing cap, affinity, credits-first and group tests in full business suite provide surrounding compatibility coverage. |
| **E4** | `TestBackupUsageTiersCLI` exercises actual provider command dispatch, show/set/off/default, fractional values, saved modes, pool summary, extra args/flags/help and errors with no write. Provider E2 verifies the shared decoding/validation/persistence used by the API. No HTTP handler test is moved out of `internal/gui` to evade the GUI-test exclusion. |
| **E5** | Extend `TestNextLoginPrefersPrimaryBeforeBackup` for both Codex/Claude: default/custom/off tiers, automatic/explicit Backup, failing old shortcuts, model-less isolation, held/capped/unknown/spent spares and most-room fallback. Existing `TestNextInOrder` confirms explicit rotation is unaffected. |
| **E6** | Source/doc review record for DOC1–DOC3 and CLI help: name exact changed symbols and check source links resolve; explain before/after for existing users, default vs off, known quota and cache behavior. This is documentation review, with no translation or asset test commands. |
| **R1** | Source review at implemented revision: GUI DTO/raw-property presence, strict decode/setter and 400 mapping; general Save preservation; editor add/remove/save/off/default requests, empty numeric inputs and failure state; server-supplied defaults; no live per-model claim; translation additions; copy/rename wiring; no remaining pace/24h promotion branch; source-based restrictions/cache invariants. Record GUI/API runtime verification as **not run under AGENTS policy**, not as covered by E2. |

Existing backup integration fixtures put Premium accounts at 5–10% with 3–4d left and expect them to stay backup; those accounts now match defaults. For ordering-only tests, explicitly set promotion off (`[]`), or use nonmatching quota comfortably away from thresholds. For promotion tests, select a policy and quota that exercise the intended tier. Do not change expected priority to disguise a fixture that no longer represents a backup.

Provider boundary tests use a fixed `now` passed through the actual classifier; HTTP integration fixtures use `time.Now` with wide margins. Advance recorded affinity timestamps/controlled reset inputs instead of sleeping across a boundary. No live credentials or vendor requests are needed to verify this local policy, which consumes existing allowance shapes.

### Runner discovery and prerequisites

Read recipes: `docs/subsystems/provider-plugins.md#verification`, `internal/testenv/testenv.go`, `internal/provider/kiro_main_test.go`, `internal/cli/main_test.go`, `internal/gateway/main_test.go`, Makefile, `.github/workflows/test.yml`, `.envrc` and `flake.nix`.

Discovery completed during design (no tests/builds run): ambient shell has no Go or Bun. `nix develop --command bash` resolves Go **1.26.8 linux/amd64**, Bun **1.3.13**, a C compiler, bash/coreutils and ripgrep; CGO is enabled. `go.mod` requires Go 1.26.8. Current CI installs Bun 1.4.2; the flake's older Bun is an existing environment difference, not a feature dependency. Verify plugin-related full-suite results with the supplied runtime; report incompatibilities/skips, and use CI's Bun 1.4.2 if a failure requires the CI version rather than silently skipping it. Dependencies must already be available or populated in the pinned Go/Nix caches before offline tests. No npm install/latest resolution is required for this feature.

Discovered Go cache locations before HOME substitution: GOPATH `/home/sinkerine/go`, GOMODCACHE `/home/sinkerine/go/pkg/mod`, GOCACHE `/home/sinkerine/.cache/go-build`. Re-query at execution rather than hard-code them. There is no `suite-lock` command on the discovered PATH; coordinate host-wide suites and use a common advisory flock file for this implementation, without claiming it controls unrelated runners.

Package discovery is always `go list -tags nogui ./... | rg -v '/internal/(gui|fonts)(/|$)'`. Never use `make test`, unfiltered `go test ./...`, `make test-ui`, browser/Playwright/layout/translation/asset/budget checks or tests in `internal/gui`/`internal/fonts`. The policy overrides broader recipes and applies even though GUI code is implemented. `nogui` still compiles the web API imported by the CLI; it avoids the desktop webview dependencies.

### Isolation and command inventory

Run the following in the worktree via `nix develop --command bash`, as one coordinated verification session after implementation. The wrapper adapts the repository's existing pinned-cache recipe and extends outer-home isolation for every Go invocation. Package TestMain/testenv additionally clears agent variables, substitutes failing keychain/agent tools, and provider/CLI tests enable offline transport. Gateway uses its own isolated TestMain and local fake upstreams. Keep fixtures local and join/clean up fake processes and servers.

```sh
set -euo pipefail
verification_root=$(mktemp -d "$PWD/.premium-tiers-verification.XXXXXX")
verification_home="$verification_root/home"
mkdir -p "$verification_home"
trap 'chmod -R u+w "$verification_root"; rm -rf "$verification_root"' EXIT
verification_gopath=$(go env GOPATH)
verification_modcache=$(go env GOMODCACHE)
verification_buildcache=$(go env GOCACHE)
export GOPATH="$verification_gopath" GOMODCACHE="$verification_modcache" GOCACHE="$verification_buildcache"
export HOME="$verification_home" USERPROFILE="$verification_home"
export XDG_CONFIG_HOME="$verification_home/.config" XDG_CACHE_HOME="$verification_home/.cache"
export XDG_DATA_HOME="$verification_home/.local/share" XDG_STATE_HOME="$verification_home/.local/state"
export XDG_CONFIG_DIRS="$verification_home/etc/xdg"
export APPDATA="$verification_home/AppData/Roaming" LOCALAPPDATA="$verification_home/AppData/Local"
exec 9>"${TMPDIR:-/tmp}/magpie-business-suite-${UID}.lock"
flock -n 9
```

If the lock is held, return control/coordinate and retry later; do not block this design tab or launch a second suite. Choose an owned durable log directory before execution and copy the verification logs there before the cleanup trap runs. Do not delete other sessions' logs or shared module/build caches.

- **V1 — Regression and focused business smoke.** After E0 has been written, create a temporary baseline source copy **inside this worktree** using `git archive ea728dcc9416d2f91af0922ed2eb6a152502ca75 | tar -x -C "$verification_root/baseline"` (create directory first), copy only `account_backup_tiers_regression_test.go` into its provider package and run `go test -tags nogui -count=1 -timeout 45m -run '^TestBackupUsageDefaultRegression$' ./internal/provider` there. Capture expected nonzero exit and the named assertion failures, then run the identical test in the implementation and require success. No checkout/reset/stash of this branch. New type-dependent tests do not constitute baseline evidence. Run T1's exact focused command and retain its full output; E3's httptest journey is the isolated business smoke. Do not launch a live gateway.
- **V2 — Full applicable unit/integration suite**, with pipefail and a complete log:

  ```sh
  go list -tags nogui ./... | rg -v '/internal/(gui|fonts)(/|$)' | xargs go test -tags nogui -count=1 -timeout 45m 2>&1 | tee "$verification_root/business-tests.log"
  ```

  This is the repository-required discovery/suite pipeline with an uncached run and the LESSONS timeout. It includes unit and business integration fixtures, including plugin fixtures when available; no separate sandbox E2E. Log package and skip results and the implemented revision/diff identity. `-count=1` prevents an earlier pass from being reused as fresh evidence.
- **V3 — Race checks for touched shared state and runtime integration**, sequential with V2:

  ```sh
  go test -tags nogui -race -count=1 -timeout 45m -run 'Backup|AccountSettingsKept|NextLogin|NextInOrder' ./internal/provider ./internal/gateway ./internal/cli ./internal/routes
  ```

  This specifically covers mutable setting copies, allowance hooks, switching and affinity. No new goroutine is planned. Full gateway/redact CI race sharding remains CI's existing independent gate; do not require a redundant full race suite locally for this synchronous policy. If implementation introduces concurrent behavior, expand checks to that behavior and record it. `-race` requires native CGO/C compiler; do not combine Windows cross-compilation with `-race`.
- **V4 — Formatting and static analysis.** `gofmt -l` over all changed/new `.go` files must print nothing; do not run `gofmt -w` until implementation edits are ready. Use `git diff --name-only --diff-filter=ACM -- '*.go'` plus the task's new Go files to include untracked files before staging. Native and cross-OS business vet commands:

  ```sh
  go list -tags nogui ./... | rg -v '/internal/(gui|fonts)(/|$)' | xargs go vet -tags nogui
  GOOS=linux go list -tags nogui ./... | rg -v '/internal/(gui|fonts)(/|$)' | xargs env GOOS=linux go vet -tags nogui
  GOOS=windows go list -tags nogui ./... | rg -v '/internal/(gui|fonts)(/|$)' | xargs env GOOS=windows go vet -tags nogui
  ```

  Native Linux vet is equivalent to the Linux command on this host and can be recorded once for both. These vet commands compile applicable `_test.go` paths without executing excluded GUI tests. Do not claim Windows native runtime verification from cross-vet. No new lint package/tool or GUI script check is required.
- **V5 — Business/CLI builds**, artifacts isolated, no working-tree binary replacement:

  ```sh
  go build -tags nogui -o "$verification_root/magpie-native" .
  CGO_ENABLED=0 GOOS=linux go build -tags nogui -o "$verification_root/magpie-linux" .
  CGO_ENABLED=0 GOOS=windows go build -tags nogui -o "$verification_root/magpie-windows.exe" .
  ```

  Build the full CLI/web application to compile provider API wiring without GTK/Wails desktop checks. Native Linux CGO and static Linux builds have distinct build modes and both apply. Windows build compiles only; runtime behavior is covered by platform-independent business tests. macOS native/GUI builds, desktop packaging and GUI asset tooling are outside this feature's verification under the repository override.
- **V6 — Source/documentation review.** Perform E6 and R1, `git diff --check`, check the final status/diff contains only T1-owned changes, and record the actual commands, tree identity, failures, skips and limitations. Trace every AC/D/DOC to its evidence. If a business suite failure appears, investigate/fix the relevant cause or record an owned existing issue per code standards; do not call it a pass or hide it through a filter. Review the implemented tree after any final edit; reuse checks only if their inputs/build mode have not changed.

Generated inputs are fake credentials in temporary homes, saved Provider JSON variants (including malformed/invalid fixtures), controlled allowance windows and httptest streams modeled on the existing `backupPool`. No generated vendor data or fixtures from production are required. Commands that write source are limited to implementation edits, intentional `gofmt -w` and the temporary baseline regression copy; none of the verification commands regenerates translations/assets or dependency lockfiles. `go build` writes only explicit temporary artifacts; Go/Nix may populate their pinned caches. Preserve `flake.lock`, `go.mod` and `go.sum` unless a separately justified dependency change is approved.

Host/process conflicts: one full suite/race job at a time; no parallel requests mutating package-global fixture maps, no shared baseline source edits, no fixed production ports. Reuse the existing E3 local HTTP fixtures rather than create a separate daemon or smoke script. The focused regression and full-suite result are complementary; once full suite/vet/builds pass, repeat only checks invalidated by later edits or findings.

Design-phase verification completed: source review and development-tool discovery only. **Implementation tests, race checks, vet and builds have not run because this is a design-only task.**

## 5. E2E journey

**Sandbox E2E: skipped — not needed under this repository's explicit AGENTS.md exemption.** This applies to new features as well as fixes. All GUI/browser/layout/translation/assets/budget and `internal/gui`/`internal/fonts` tests are also skipped. No real account, production gateway, VM sandbox, deployment or GUI session is needed for this plan.

Map the user journey to business integration instead:

1. E4 configures a provider's tier list through real CLI dispatch; E2 confirms default/custom/off persistence and round-trip. R1 reviews the equivalent API/editor call path and explicitly records that GUI/API handler runtime checks were excluded.
2. E3 starts saved fake Standard and Premium accounts, a local vendor and `Server.Front()` on ephemeral ports. With backup detection enabled, failing Premium quotas stay backup and matching quotas compete as primary. Execute both provider and pooled-group paths and assert upstream account plus routing trace.
3. E3 changes usage/reset/policy in controlled state, including different models on the same account; new conversations follow current eligibility while warm-cache/within-turn conversations retain existing affinity. Reset passing without a new future reset does not invent promotion.
4. E3/E5 prove caps/rests/ordinary eligibility still win, nonmatching accounts remain usable backups when needed, and local Codex/Claude spare selection agrees with the shared policy. Existing non-Smart and `NextInOrder` tests preserve compatibility.
5. E0 proves both directions of the intended behavior change fail on the original implementation; V2–V5 establish surrounding business compatibility. E6 records the user-facing and subsystem changes, actual results and excluded verification.

The user explicitly approved this whole plan via “Approve plan and hand off.” Send the required `PLAN_READY` handoff to the orchestrator; T1 remains the orchestrator's responsibility, with no implementation or commit in this design tab.
