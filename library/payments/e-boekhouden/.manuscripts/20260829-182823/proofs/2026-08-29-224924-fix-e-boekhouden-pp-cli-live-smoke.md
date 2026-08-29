# e-Boekhouden CLI Live Smoke Test (post-publish-question, user-provided token)

After the initial shipcheck/polish/promote cycle (mock-mode only, no API key),
the user provided a real read-only API token (plain text file on their own
machine) and asked for read-only live testing before deciding on publish.
This surfaced 8 real bugs that mock-mode verification could not catch —
all fixed and re-verified live. No write/mutating commands were run at any
point (mutation create / invoice create / costcenter create / etc. were never
exercised against the live API), per the user's explicit "READ ONLY" instruction.

## Bugs found live and fixed

1. **Session `source` identifier rejected by the real API.** `"pp-cli"` (with
   a hyphen) violates the API's own documented pattern `^[\w_ ]{1,10}$` — `\w`
   excludes hyphens. Every session exchange failed with `API_SESSION_002`.
   Fixed: `"pp_cli"`.
2. **`administration overview` aborted entirely for non-accountant tokens.**
   `GET /v1/administration` returns `EP_001` ("only available to
   accountants") for the majority of real accounts (single-administration,
   non-accountant). The command treated this as fatal instead of degrading
   gracefully. Fixed: administrations list is now best-effort/omitted, while
   linked administrations + balances + outstanding figures (which work for
   everyone) still render.
3. **Missing required `credDeb` query parameter.** `GET
   /v1/mutation/invoice/outstanding` requires `credDeb=D` (debtors/AR) or
   `credDeb=C` (creditors/AP) — undocumented as a hard requirement until hit
   live (`MUTA_006`). Fixed in both `administration overview` and `report
   outstanding-aging`, and upgraded both to report AR and AP separately
   (previously assumed the API had no AP-side data at all — it does).
4. **`balance` typed table never populated by `sync`.** `store.UpsertBatch`'s
   per-item typed dispatch switches on the literal table name `"balance"`,
   but `sync` calls it with the sync-resource name `"ledger-balances"` — no
   match, so all 58 real balance rows landed only in the generic `resources`
   table under `resource_type="ledger-balances"`, silently skipping every
   local financial report. Fixed with a resource-name-to-typed-table-name
   remap in `upsertResourceBatch` (sync.go).
5. **`balance` ledger_id foreign-key lookup raced against concurrent
   `ledger` sync.** Even after fix #4, the lookup depended on `ledger` having
   already committed the matching row — but `sync` runs resources
   concurrently (confirmed via interleaved progress events), so the lookup
   legitimately missed on a real full sync. Fixed: falls back to the
   balance's own resolved id (the code) when no match is found yet, since
   nothing in this codebase depends on `ledger_id` being the true numeric id.
6. **Mutation list responses have no `rows` field.** The OpenAPI spec's
   `MutationsResponseDto` (list) schema was an undefined `$ref` (generator
   stubbed it as a permissive object); the actual list shape only exposes a
   top-level `amount` and `ledgerId`, not the per-line `rows` breakdown shown
   in the (correct) detail schema for `GET /v1/mutation/{id}`. This made
   `mutation suggest`, `ledger history`'s row-level matching, and `relation
   statement`'s amount computation silently return 0/empty against real
   synced data. Fixed: switched to the reliable top-level `amount`/`ledgerId`
   fields; `report vat-summary` (which genuinely needs the row-level VAT
   breakdown) now documents accurately that coverage depends on `mutation
   get-id` write-through-caching individual mutations, rather than claiming
   full coverage from a plain sync.
7. **`{"items": [...], "count": N}` envelope shape, not a bare array.**
   Every list-shaped GET response (outstanding invoices, ledger balances,
   linked administrations) wraps its array in an `items`/`Items` envelope,
   contradicting the spec's stubbed bare-array response schema. Broke
   `report outstanding-aging`'s parsing entirely (hard error) and silently
   zeroed every count in `administration overview`'s human-readable output
   and its ledger-balance table rendering. Fixed with a shared
   `extractItems()` helper tolerant of both shapes.

## What was NOT tested (by design)
Every mutating command (`mutation create`, `invoice create`, `costcenter
create/update/delete`, `product create/update/delete`, `relation
create/update`, `ledger create/update`, `member create/update`) was left
completely untouched — `--dry-run`/mock-mode verification is the only
evidence for write-path correctness. The write-safety guard
(`--confirm`/`--company`) was verified to correctly refuse an un-confirmed
write, but no real write was ever sent.

## Post-fix verification
Full `shipcheck` umbrella re-run: 6/6 legs PASS, Grade A (92/100). All Go
unit tests updated to match corrected field assumptions and still pass.
Library copy at `~/printing-press/library/e-boekhouden` re-synced with every
fix (edited directly, not regenerated).
