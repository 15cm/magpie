# Codex reset hours: verification and remaining blockers

The user chose **keep feature scope; document blockers**, retain the branch and leave it unmerged. No production deployment or settings changes were made. The later user instruction exempts this project from sandbox E2E testing, including e2e-feature; AGENTS.md records that decision. Ordinary regression checks still apply.

## Implemented behavior

`MAGPIE_CODEX_WEEK_RESET_MIN_HOURS` accepts whole hours 0–168 with surrounding whitespace. Unset/empty defaults to 0. Weekly auto-use requires renewal at least that many hours away (inclusive). Invalid values pause the weekly trigger before its vendor read. Expiry spending and generic allowance/routing classification remain independent. CLI/TUI and all five GUI languages report threshold/pause. Installed services carry the variable. GUI fields are derived and never persisted, including fresh and cached state responses.

## Accepted tasks and commits

| Work | Review rounds | Commit |
|---|---:|---|
| Approved plan | user approval | 147709ed7 |
| T1 core, gateway, CLI/TUI, environment | 1 | e21bc1474 |
| Clock and LAN-authentication fixtures | 1 | 06c8e5bb5 |
| T3 references, deployment docs, instructions | 1 | 0e178028b |
| T2 GUI and state serialization | 2 | 558da031b |
| Settings and portable shell fixtures | 1 | 582cb2478 |
| Mobile quota wrapping and panel fixtures | 1 | e4756c589 |

The final bounded GUI repair batch was reviewed with no findings: scoped Desktop/install label layout, bare context-tile padding, reader-driven test scrolling, settled resize measurements, equivalent alignment measurements, and a Japanese placeholder correction. It is committed as `5951d32d1`. The user-requested sandbox E2E exemption is committed separately as `c20ba850d`.

Independent reviewers were restricted to source and artifact review by their configured tools. They did not execute commands; parent and workers supplied execution evidence. Quota failures before verdict did not count as review rounds.

## Passing evidence and limits

- Parent focused provider/gateway/CLI, environment guard and daemon tests passed in temporary HOME/XDG with pinned Go caches and ambient threshold 999.
- Removing the threshold guard caused behavioral FAIL in provider HoursAhead/73 and gateway below/credits_below; restored checks passed. Equality and ±1-second boundaries use a fixed clock.
- Parent T2 checks passed: Go state/cache/profile/budget checks, Chromium 75/75 and WebKit 71/71. Invalid negative, over-limit and overflowing numeric API values are covered.
- Native, Linux nogui and Windows cross-vet/build checks passed. Windows execution was not performed.
- Full Go run at 582cb2478 recorded 85 passing packages and 17 packages without tests. The outer launch log retained the complete results; editing a running scratch verification script damaged its dedicated suite log and interrupted subsequent commands. Separate parent follow-up completed race count=5 and ambient-999 provider/gateway package checks with exit 0. These checks predate the final browser repair batch and are not claimed as a final-head full-suite pass.
- Bun 1.3.13 failed local Git fixture installs. Exact temporary Bun 1.4.2 passed those tests unchanged. Host Pi package-root variables could select host Pi 1.4.0 over fixture 1.2.0; clearing inherited Pi/module overrides made TestPiPackage pass. No dependency/profile changes were committed.
- Built CLI in fresh HOME passed unset/24/abc set and readback journeys. Gateway, expiry, boundary and daemon sandbox journeys passed before the later sandbox E2E exemption. No real reset was spent.
- Mobile quota/panel repairs passed parent 34/34 in both engines; worker 72 related checks passed. The final bounded repair batch passed the parent's combined run: 69 tests in each engine invocation, with exit 0 (some language tests run both engines internally). It also passed independent source review; this does not clear the full browser suite.

## Full browser gate remains open

The completed broad run used cached Docker Playwright 1.64.0, network disabled, feature tree mounted read-only, concurrency 2. Some files run both engines despite BROWSER=chromium. It recorded **8,716 tests: 8,537 pass, 179 fail across 50 files**, with no skips. Source edits occurred during that long run, so it is diagnostic evidence, not a final-head acceptance run. The full WebKit suite and a final repaired-tree full browser rerun were not performed, per the user's scope decision.

Six representative files were also run against original base e169656a31752907d34538ef53b85c194b2e0122 under the same container: gui-ja, model-chip-steady, provider-fetch-again, recovered-file, remote-magpie and segs-press-redraw. That probe recorded 32 pass/44 fail, confirming those failures precede the feature in this environment. This is not proof that every remaining failure is pre-existing.

Remaining groups include picked-only model list expectations, key-editor fixtures, disabled search controls, retained routing-row identity, dynamic presentation classes, private Playwright import compatibility, Linux animation assumptions, navigation timeout setup, hidden-page polling fixtures, synthetic scrolling, clipping/width errors, scroll containment, control targeting, movement after updates, virtualized wire endpoints and hidden header controls. Some causes are source-backed; others require runtime reproduction. They are not blanket-classified as environment failures or waived. Existing issue histories were not checked and no external issues were filed: the user chose documenting blockers rather than broader maintenance or filing.

## Evidence locations

These are local scratch artifacts; they are not committed or guaranteed durable after host cleanup:

- `/tmp/codex-reset-hours-final-evidence/gui-chromium.log`: complete broad browser output.
- `/tmp/codex-reset-hours-gui-failure-inventory.json`: failures grouped by 50 files.
- `/tmp/codex-reset-hours-base-probe.log`: original-base six-file comparison.
- `/tmp/codex-reset-hours-final-verification-r2-launch.log`: retained full Go package output and shell failure.
- `/tmp/codex-reset-hours-r2-followup.log`: passing race and ambient-999 follow-up.
- `/tmp/codex-reset-hours-eight-gui/RESULTS.md`: final bounded repairs, baseline, mutations and related checks.
- `/tmp/codex-reset-hours-final-batch.log`: parent final bounded both-engine run.
- `/tmp/codex-reset-hours-browser-repair-evidence/RESULTS.md`: earlier three-file browser repair evidence.

## Delivery status

Feature implementation is complete and reviewed. Full regression acceptance is **not complete**, and this branch is **not merge-ready**. Keep the Paseo workspace and branch for later work. Do not merge, push, deploy or archive the workspace on the basis of this report. Production weekly auto-use remains off on all nine accounts and expiry auto-use remains on; this workflow never changed those settings.
