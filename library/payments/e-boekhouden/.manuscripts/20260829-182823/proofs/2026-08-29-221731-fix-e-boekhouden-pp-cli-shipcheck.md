# e-Boekhouden CLI Shipcheck Report

## Shipcheck umbrella result: PASS (6/6 legs)

| Leg | Result |
|---|---|
| verify | PASS (100%, 21/21, 0 critical) |
| validate-narrative | PASS (10/10 commands resolved + full examples pass) |
| dogfood | PASS (path validity 3/3, 0 dead flags/functions, novel features 6/6 survived) |
| workflow-verify | PASS (no workflow manifest; not applicable) |
| verify-skill | PASS (all checks: flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command) |
| scorecard | PASS — 91/100, Grade A |

## Fix loops (2, within the skill's default cap)

**Loop 1:**
- validate-narrative FAILed on an `export EBOEKHOUDEN_API_TOKEN=...` quickstart
  entry (not a CLI invocation). Fixed: removed from research.json's quickstart array.
- Scorecard's live sample-output probe: 5/6 novel commands failed with a confusing
  SQLite "out of memory (14)" error. Root cause: hand-written commands used
  `store.OpenReadOnly` instead of the codebase's standard `store.OpenWithContext`,
  so a nonexistent DB file (fresh install, no `sync` yet) produced a driver error
  instead of a clean empty result. Fixed across all 5 affected files, plus fixed a
  few nil-slice-marshals-to-null spots so empty JSON results render as `[]`.

**Loop 2:** none needed — full umbrella passed after loop 1's fixes.

## Before/after
- verify: 100% pass rate both before and after (loop 1 fixed narrative/scorecard,
  not verify itself).
- scorecard: 90/100 → 91/100 (insight dimension 4/10 → 7/10 after the empty-DB fix
  let more sample commands execute meaningfully).
- Sample Output Probe: 1/6 → 3/6 pass rate. Remaining 3 failures are expected in
  this environment: no API key was provided (Administration Portfolio Overview
  needs a live call), and the local store has no synced data (Mutation Ledger/VAT
  Suggest has nothing to match against; Ledger Drill-Down has no synced ledger to
  resolve a code against). None are functional bugs — see phase-4.85-findings.md
  for the unit-test coverage that verifies the real logic against seeded fixtures.

## Top blockers found and fixed
See the build log (`*-build-log.md`) for full detail: a broken command-wiring
collision, a non-buildable "administration safety" design requiring reframe, two
SQLite TEXT/INTEGER comparison bugs, a generator-level balance-upsert bug (fixed +
tested), a description-truncation bug affecting 4 generated surfaces (fixed), and
the OpenReadOnly-vs-OpenWithContext UX bug (fixed).

## Final ship recommendation: ship
