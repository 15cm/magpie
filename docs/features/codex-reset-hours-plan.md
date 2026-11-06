# Codex weekly auto-reset: only when the renewal is at least X hours away

Status: **Approved by the user (2026-11-06)**, with decisions D1 to D4
confirmed.

Base: `main` at e169656a3 (worktree `codex-reset-hours`).

## Goal and acceptance criteria

### Goal

Today, when a Codex account has "when its week runs out" on, magpie spends one
of its rate-limit resets once its weekly window is used up. It does that
however soon the vendor would renew the week anyway, so a card can be spent
an hour before the free renewal. After this change, the weekly trigger spends
a reset only when the used-up week's renewal (`ResetsAt`) is **at least X
hours** away. X comes from a new environment variable. The separate
"before one expires" trigger (`SpendExpiringCodexResets`) does not change.

### Decisions (confirmed by the user)

| # | Decision | Chosen |
| --- | --- | --- |
| D1 | Variable name | `MAGPIE_CODEX_WEEK_RESET_MIN_HOURS` |
| D2 | Default when unset or empty | `0`: no threshold, so today's behaviour and today's label. Users who already turned the weekly switch on keep what they chose (code standards: magpie's default never overrides a value the user set). |
| D3 | Invalid value | **Fail closed**: the weekly trigger spends nothing, makes no vendor read, logs why, and the GUI and CLI say it is paused. The expiry trigger keeps running. A failed read means "unknown", and a spent reset can't be taken back. |
| D4 | Sandbox E2E guidance | The "Sandbox E2E" text in the E2E section, approved as proposed. The user adds it to the feature worktree's AGENTS.md, or approves the orchestrator adding it, before E2E delivery. |

### Value rules

- Accepted: a whole number of hours from `0` to `168` (one week), with
  whitespace around it allowed (`" 24 "`). `0` means no threshold.
- Invalid: anything else. That includes `-1`, `169`, `1.5`, `24h`, `abc`
  and `1e2`. Under D3, the weekly trigger is paused.
- Unset and `""` are the same, and both mean the default (D2).
- The variable is read from the process that serves the gateway. It is read
  on each look, so a change takes effect after a restart of that process.
  Nothing is cached across a restart, so a restart is enough.

### Exact boundary

With `until` the used-up weekly window's `ResetsAt` and `now` the time of the
look (`autoResets.use`'s `now`), the weekly trigger may spend only when:

```
until.Sub(now) >= time.Duration(X) * time.Hour
```

The comparison is inclusive: exactly X hours away spends, and one second less
doesn't. X = 0 gives `until.Sub(now) >= 0`. `weekUsedUp` already requires
`until.After(now)`, so X = 0 is today's rule.

### Acceptance criteria

1. With X set and the week used up, renewal `X h - 1 s` away: no reset is
   spent, on both gateway paths (out-of-allowance `autoReset` and
   `resetBeforeCredits`). The request falls back as it does today: another
   account, credits, or the 429 with why.
2. Renewal exactly `X h` away: a reset is spent (unit test with a fixed
   `now`).
3. Renewal `X h + 1 s` away, or more: a reset is spent, still one a week at
   most.
4. Unset, `""` or `0`: behaviour and words are the same as on main, and the
   existing tests pass unchanged.
5. Invalid value (D3): no weekly spend and no read of the
   vendor's windows. The cause is logged at most once per wait
   (`autoResetWait[true]`, 10 min). The GUI and CLI say the weekly trigger
   is paused and name the variable.
6. The expiry trigger is unchanged. An account with "before one expires" on
   whose week is used up and renews in under X hours still spends the
   expiring reset at its lead, or at once when held up past it.
7. Generic uses of the week's state are unchanged: `weekUsedUp`,
   `Allowance.WeekUsedUp`, routing, caps and Smart/Weekly pace.
8. With X > 0, the GUI's weekly label reads "when its week runs out and
   refresh is at least {X} h ahead" in en, zh-CN, zh-TW, ja and de. So do
   its titles and status lines, the CLI's `quota auto-reset` words and the
   TUI's `AutoResetWords`.
9. `magpie daemon install` carries the variable into the service.
10. Production (`https://magpie.sachi.m.mado.moe`) is not configured,
    restarted or deployed to. Its settings stay as they are: weekly off for
    all nine accounts, expiry on.

## Design

### Module boundaries and data flow

```
env MAGPIE_CODEX_WEEK_RESET_MIN_HOURS
   │  read by provider.CodexWeekResetMinHours()   (the only reader)
   ▼
provider.AutoUseCodexReset ── look(now) ──► weekUsedUp(windows, now)  [unchanged]
   │                                └─► weekResetFarEnough(until, now, hours) [new]
   │  declines → sentinel error → autoResets.use waits 10 min, returns err
   ▼
gateway.autoResetOf (logs "codex reset for X not used: …")  [unchanged code]
   ├── Server.autoReset              (every candidate out of allowance)
   ├── Server.autoResetSignedIn
   └── Server.resetBeforeCredits(On) (prefilter Allowance.WeekUsedUp, unchanged)

provider.WeekAheadWords() ──► AutoResetWords (TUI, CLI resetsCell) and cli.autoResetSay
gui.settingsState() ──► settingsJSON.codexWeekResetMinHours / codexWeekResetMinHoursBad ──► app.js weekSay()
daemon.carried ──► the service environment
```

Only the spend decision changes. Both gateway paths already end in
`AutoUseCodexReset`, which reads the week again before spending. That makes
it the one place the threshold goes, and no gateway code changes. The
gateway's prefilter `Allowance.WeekUsedUp` stays as it is. When it passes and
the threshold declines, the request goes to credits or the next account, as
it does when no reset is held.

### Interfaces (internal/provider/codex_autoreset.go)

```go
// CodexWeekResetMinHoursMost bounds MAGPIE_CODEX_WEEK_RESET_MIN_HOURS: a week.
const CodexWeekResetMinHoursMost = 168

// CodexWeekResetMinHours is how many hours ahead a used-up week's renewal
// must be for the weekly trigger to spend a reset (MAGPIE_CODEX_WEEK_RESET_MIN_HOURS).
// Unset or "" is the default (D2). ok is false, and raw the value as set,
// when it isn't a whole number from 0 to CodexWeekResetMinHoursMost.
func CodexWeekResetMinHours() (hours int, raw string, ok bool)

// weekResetFarEnough says whether a used-up week ending at until is at least
// hours ahead of now: inclusive, exactly hours away is far enough.
func weekResetFarEnough(until, now time.Time, hours int) bool

// errWeekRenewsSoon and errMinHoursBad are what AutoUseCodexReset's look
// declines with; autoResets.use treats them as any failed look (a 10-minute
// wait) and the gateway logs them as "not used".
var errWeekRenewsSoon, errMinHoursBad = errors.New(...), errors.New(...)

// WeekAheadWords is what the weekly trigger's words add: "" for 0,
// " and refresh is at least 24h ahead", or " (paused: MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=abc
// is not whole hours from 0 to 168)".
func WeekAheadWords() string
```

`AutoUseCodexReset`'s look becomes:

1. `hours, raw, ok := CodexWeekResetMinHours()`. When `!ok`, return
   `fmt.Errorf("%w: %s=%q …", errMinHoursBad, …)` before any network read.
2. Read the windows as today (`codexUserWindows`).
3. `week := weekUsedUp(windows, now)`. When `week != nil &&
   !weekResetFarEnough(*week, now, hours)`, return
   `fmt.Errorf("%w: its week renews %s, under the %dh asked", errWeekRenewsSoon, …)`.
4. Otherwise return `week, held, nil` as today.

`autoResets.use` keeps its signature. An error from the look already sets
`next = now + autoResetWait[true]` (10 min). That also bounds the log lines
and the vendor reads while the trigger declines. The renewal only gets
closer, so nothing changes until the window renews, a manual reset starts a
new week, or the process restarts with another X. A manual reset is picked
up within 10 minutes.

`weekUsedUp` and `Allowance.WeekUsedUp` don't change. Comments that say
"weekUsedUp's rule" (routing.go, account_caps.go) are left alone, because
the rule they name is still the one they mean.

### Read convention

- Use a literal `os.Getenv("MAGPIE_CODEX_WEEK_RESET_MIN_HOURS")` with
  `strings.TrimSpace` and `strconv.Atoi`, read on every call, with no
  package-level cache. That lets tests use `t.Setenv`.
  `TestFolderVarsAreListed` scans provider for literal Getenv names. The
  variable is no folder, so it goes in `notAnAgent`
  (internal/agent/agentenv_test.go) with a reason, not in `agentenv.Vars`.
- Sandbox: tests that need a value set it with `t.Setenv`. Every test that
  depends on the default calls `t.Setenv(name, "")`, which is how
  `MAGPIE_TRUST_PROXY` tests guard against a developer's shell. The existing
  auto-reset tests in provider and gateway get that line, so a value in the
  developer's shell can't turn them red.
- daemon: add the name to `carried` in internal/daemon/daemon.go.

### GUI

- `settingsJSON` (internal/gui/api.go) gains read-only fields. They are
  never saved, because they sit outside `settings.Settings`:
  - `CodexWeekResetMinHours int` (`json:"codexWeekResetMinHours"`), the
    value in effect;
  - `CodexWeekResetMinHoursBad string` (`json:"codexWeekResetMinHoursBad,omitempty"`),
    the raw value when it is invalid.
  `settingsState()` fills them from `provider.CodexWeekResetMinHours()`.
  `POST /api/settings` decodes into `settings.Settings`, so the fields can't
  be written back.
- app.js: add `autoResetSay(c, s = state?.settings)` and
  `autoResetCaseTitle(c, on, s)`. Every place that now does `t(c.say)` or
  `t(c.on/c.off, resetLeadVars())` uses them: `autoResetRow`, the folded
  panel brief, `autoResetBrief`, `autoResetButton`'s aria-label and title,
  and autoresetown.js's Settings rows. Their output depends on the value:
  - 0: today's strings, unchanged.
  - X > 0: "when its week runs out and refresh is at least {hours} h ahead".
    The week's "On:" titles get "A week that renews in under {hours} h is
    waited out instead." appended. `autoResetStatus` and the
    `resets-kept` tag use the same X-aware variants.
  - Invalid: "when its week runs out" + " · " + "paused". The titles get
    "{env} isn't a whole number of hours from 0 to 168, so a used-up week
    uses no reset by itself." appended.
- New i18n keys go in zh-CN, zh-TW, ja and de with the same placeholders.
  `i18n-keys.test.cjs` and gui-zh-tw/ja/de enforce that. The strings:
  1. `when its week runs out and refresh is at least {hours} h ahead`
  2. `auto-used when the week runs out and refresh is at least {hours} h ahead`
  3. `{who} uses a reset by itself only when its week runs out and refresh is at least {hours} h ahead`
  4. `{who} uses a reset by itself when its week runs out and refresh is at least {hours} h ahead, or before one expires`
  5. `A week that renews in under {hours} h is waited out instead.`
  6. `paused`
  7. `{env} isn't a whole number of hours from 0 to 168, so a used-up week uses no reset by itself.`
- app.js size budget (`TestCoreScriptBudget`): the change adds about 1 KB.
  If the budget is hit, move the new helpers into autoresetown.js or trim.
  Don't raise the budget (LESSONS.md).

### CLI and TUI

- `provider.AutoResetWords`: "auto when the week runs out" + WeekAheadWords
  (also for "auto"). The TUI picks this up through `internal/tui/pages.go`.
- `cli.autoResetSay` and `quotaAutoResetCmd`'s ✓ lines append
  `WeekAheadWords()` after "when its week runs out" / "once its week is
  used up".
- `quotaUsage`: one sentence after the auto-reset paragraph:
  "MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=24 spends the week's only while its
  renewal is at least 24 hours away (0 to 168; a bad value pauses it)."

### Error handling summary

| Case | Weekly trigger | Log | GUI/CLI |
| --- | --- | --- | --- |
| unset, "", 0 | as main | as main | as main |
| valid X, renewal ≥ X | spends (once a week) | "reset used … its week used up" | X shown |
| valid X, renewal < X | no spend, 10 min wait | "codex reset for U not used: its week renews …, under the Xh asked" | X shown |
| invalid | no spend, no vendor read, 10 min wait | "… not used: MAGPIE_…=\"abc\" is not whole hours from 0 to 168" | "paused" |
| expiry trigger, any X | unchanged | unchanged | unchanged |

### Docs

- docs/subsystems/gateway-routing.md: the `autoReset`/`resetBeforeCredits`
  sentence gets the threshold, with a link to `AutoUseCodexReset`.
- docs/reference.md: the credits/auto-reset passage (~l.2466), and the
  service-environment paragraph (~l.2778), where the variable is carried.
- site/public/docs/docker.html with zh/ and ja/: one row in the Environment
  table.
- README.md and README.zh-CN.md: nothing. They don't describe auto-reset.

## Tasks

### Wave 1

**T1, core rule, gateway proof, CLI/TUI words, env guard lists**
- Goal: the threshold decides the weekly spend everywhere it is spent, and
  the text-mode surfaces say so.
- Owned files: `internal/provider/codex_autoreset.go`,
  `internal/provider/codex_autoreset_test.go`,
  `internal/provider/codex_expiring_test.go` (an expiry-preservation case),
  `internal/gateway/codex_autoreset_test.go` (the fake backend's week end
  made settable, plus new cases), `internal/cli/quota_cli.go` (and its test,
  if one pins the words), `internal/agent/agentenv_test.go`,
  `internal/daemon/daemon.go` and `internal/daemon/*_test.go`.
- Produces: `CodexWeekResetMinHours`, `CodexWeekResetMinHoursMost`,
  `WeekAheadWords`, and the decline errors described above.
- Consumes: none.
- Proving command:
  `go test -tags nogui -count=1 -run 'AutoReset|WeekUsedUp|WeekResetFar|MinHours|Expiring|ResetSpent' ./internal/provider ./internal/gateway ./internal/cli ./internal/daemon && go test -tags nogui -count=1 -run TestFolderVarsAreListed ./internal/agent`
- Tests to add:
  - `TestCodexWeekResetMinHours`: a table of unset, "", "0", "24",
    " 24 ", "168", "169", "-1", "1.5", "24h", "abc".
  - `TestWeekResetFarEnough`: a fixed now; `X h - 1 s` false, `X h` true,
    `X h + 1 s` true, X = 0 with the week 1 s ahead true.
  - `TestAutoUseCodexResetHoursAhead` (provider, httptest backend): the
    week renews in 72 h. X = 73 spends none and its error is
    `errWeekRenewsSoon`. X = 71 spends one. Invalid spends none, and the
    backend sees no usage read.
  - gateway: `TestCodexAutoResetHoursAhead`, with the fake week end 72 h
    away. X = 80 gives the 429 and spent "". X = 60 gives 200 and spent
    "acct-1". The credits path with X = 80 spends credits, not a reset. The
    margins are wide on purpose, because Unix-second truncation and time
    passing during the test must not decide the outcome (code standards:
    room on both sides).
  - Expiry preservation: an existing `SpendExpiringCodexResets` case run
    with X = 168 still spends.
  - Break by hand: drop the threshold check, and the new tests go red with
    a real FAIL.

### Wave 2 (parallel, disjoint files)

**T2, GUI**
- Goal: every GUI surface of the weekly trigger shows X, or shows "paused".
- Owned files: `internal/gui/api.go`, `internal/gui/assets/app.js`,
  `internal/gui/assets/autoresetown.js`, `internal/gui/assets/i18n.js`,
  `internal/gui/tests/codex-auto-reset.test.cjs`,
  `internal/gui/tests/codex-auto-reset-settings.test.cjs`,
  `internal/gui/tests/panel-options-fold.test.cjs`, and a Go test for
  `settingsState` in `internal/gui` (`*_test.go`, new file).
- Consumes: `provider.CodexWeekResetMinHours`, from T1.
- Produces: `settingsJSON.codexWeekResetMinHours` and
  `codexWeekResetMinHoursBad`.
- Proving command:
  `go test -tags nogui -count=1 -run 'SettingsState|CoreScriptBudget' ./internal/gui && node --test internal/gui/tests/codex-auto-reset.test.cjs internal/gui/tests/codex-auto-reset-settings.test.cjs internal/gui/tests/panel-options-fold.test.cjs internal/gui/tests/i18n-keys.test.cjs && BROWSER=webkit node --test internal/gui/tests/codex-auto-reset.test.cjs internal/gui/tests/codex-auto-reset-settings.test.cjs internal/gui/tests/panel-options-fold.test.cjs`
- The GUI tests get fixtures with `codexWeekResetMinHours: 0` (the old
  words), `24` (the new words in every language), and
  `codexWeekResetMinHoursBad: "abc"` ("paused").

**T3, docs**
- Goal: the owning references and the deployment env docs describe the
  rule.
- Owned files: `docs/subsystems/gateway-routing.md`, `docs/reference.md`,
  `site/public/docs/docker.html`, `site/public/docs/zh/docker.html`,
  `site/public/docs/ja/docker.html`.
- Consumes: the name and rules fixed in this plan.
- Proving command: `git diff --check` and a grep for the variable name
  across the five files. Links resolve
  (`grep -o '](../../internal/[^)]*)' docs/subsystems/gateway-routing.md`
  each exists).

If the orchestrator would rather not split, T1 to T3 also make one
cohesive task. The split follows only the Go-core, GUI and docs boundaries.

## Test and verification

Run everything in a sandbox HOME with caches pinned (the
docs/subsystems/provider-plugins.md snippet):

```sh
set -o pipefail
GOPATH=$(go env GOPATH) GOMODCACHE=$(go env GOMODCACHE) GOCACHE=$(go env GOCACHE)
export GOPATH GOMODCACHE GOCACHE
h=$(mktemp -d); export HOME=$h XDG_CONFIG_HOME=$h/.config XDG_CACHE_HOME=$h/.cache XDG_DATA_HOME=$h/.local/share XDG_STATE_HOME=$h/.local/state
unset MAGPIE_CODEX_WEEK_RESET_MIN_HOURS MAGPIE_ADDR
# … checks …
chmod -R u+w "$h"; rm -rf "$h"
```

Full-feature checks, recorded with output at the pushed head:

1. `gofmt -l $(git diff --name-only main -- '*.go')` prints nothing.
2. `go vet ./...`, `GOOS=linux go vet -tags nogui ./...`,
   `GOOS=windows go vet ./...`.
3. `go build ./...`, `GOOS=linux go build -tags nogui .`,
   `GOOS=windows go build .`.
4. `go test -tags nogui -timeout 45m ./... 2>&1 | tee $SCRATCH/suite.log`
   (one suite per machine, kept whole until its reds are named).
5. `go test -race -count=5 -run 'AutoReset|HoursAhead' ./internal/provider ./internal/gateway`.
6. GUI: the whole suite once (`make test-ui`) in Chromium, then
   `BROWSER=webkit`. This change adds an API field the GUI reads, which
   counts as "a new API the GUI calls" in code standards §4. That includes
   gui-zh-tw, gui-ja, gui-de and i18n-keys.
7. Break-by-hand record: remove `weekResetFarEnough`'s check and name the
   tests that FAIL.
8. Each test sets the variable it depends on with `t.Setenv`. Run the
   provider and gateway packages once with
   `MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=999` exported in the shell. They must
   still pass, which proves the defaults are pinned.

Real use (criterion 5), not possible, and the record says so: spending a
real ChatGPT reset is irreversible, and the only real accounts are on the
production service, which this feature must not touch. The vendor's
`reset_at` shape is already covered by the existing parser and its
fixtures, and this change does not alter it.

## E2E journey (sandbox)

The project's AGENTS.md has no sandbox E2E guidance yet. **The user must add
it to the feature worktree's AGENTS.md before E2E delivery.** The user approved this text
(D4):

> ## Sandbox E2E
>
> E2E runs never touch a live magpie: not `https://magpie.sachi.m.mado.moe`,
> not `~/.config/magpie`, not a running gateway on :3425, not a real agent's
> config. Build the binary into the scratchpad. Run it with a fresh
> `HOME`/XDG set from `mktemp -d`, `MAGPIE_ADDR=127.0.0.1:<free port>` and
> the Go caches pinned. Remove the HOME afterwards. Never spend a real
> ChatGPT/Codex reset or point at a real account: the vendor is an
> `httptest` server, through the gateway's real server stack (`gateway.New`)
> in Go journey tests, and the GUI runs against its Playwright fake API.
> Record each step's command and output in the commit body.

A built binary can't be pointed at a fake ChatGPT
(`provider.CodexBase` has no env override, and adding one is out of scope).
So the spend journeys run through the real server stack in Go, and the
text and GUI journeys run on the built binary and in Playwright.

| # | Step | Expected |
| --- | --- | --- |
| J1 | Gateway journey, X unset: account `me@example.com` with weekly on, week 100% used, renewing in 72 h, one reset held; POST a Codex turn | 200 "pong", one reset spent, log "codex reset used … week used up" (main's behaviour) |
| J2 | Same with `MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=80` (below: renewal 72 h < 80 h) | 429 with why, no reset spent, log "… not used: its week renews …, under the 80h asked"; a second request inside 10 min makes no usage read |
| J3 | Same with `=60` (above) | 200, one reset spent; a second used-up turn in the same week spends none |
| J4 | Exactly at the boundary: `TestWeekResetFarEnough` with fixed now, renewal = X h | far enough (spends); X h − 1 s not |
| J5 | Credits path, `=80`, account holds credits | the turn goes through on credits; no reset spent |
| J6 | Invalid `=abc` | no reset spent, no usage read, log names the variable and value |
| J7 | Expiry preserved, `=168`, expiry on, a reset running out within the lead, week used up and renewing in 1 h | `SpendExpiringCodexResets` spends it (`why: expiring`); weekly spends none |
| J8 | Built binary in the sandbox HOME: `magpie quota auto-reset me@example.com week on` then `magpie quota auto-reset me@example.com`, with the variable unset, `24`, `abc` | words: "…when its week runs out" / "…when its week runs out and refresh is at least 24h ahead" / "…(paused: MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=\"abc\" …)"; settings.json in the sandbox HOME only |
| J9 | `TestEnviron` (internal/daemon) with `MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=24` set (there is no dry-run install; no real service is installed) | `environ()` carries `MAGPIE_CODEX_WEEK_RESET_MIN_HOURS=24` |
| J10 | GUI (Playwright, both engines, five languages): fixture `codexWeekResetMinHours` 0 / 24 / bad | the card, panel (folded and open), brief and Settings rows show the old words / "…and refresh is at least 24 h ahead" / "… · paused"; the expiry row unchanged in all three |
