# e-Boekhouden CLI Build Log

## What was built
- Full absorb: all 24 v1 REST API endpoints across 11 resources (administration,
  costcenter, invoice, invoicetemplate, emailtemplate, ledger, member, mutation,
  product, relation, unit, session), spec-emitted CRUD/list commands.
- Session-handshake auth hand-wired (internal/client/session.go): the generator's
  built-in `session_handshake` auth type only supports a GET-based bootstrap+crumb
  fetch; e-Boekhouden needs a POST with a JSON credential body. Declared the spec
  auth as `api_key` with `x-auth-format: "{token}"` (no Bearer prefix) and hand-wrote
  the real POST /v1/session exchange, session token caching (disk-persisted, 0600),
  auto re-exchange on 401, and a one-time-retry-on-401 hook in client.go's request loop.
- 6 novel/transcendence features, all hand-coded local SQLite joins/aggregations:
  mutation suggest, invoice reconcile, administration overview, relation statement,
  ledger history, and a write-safety guard (requireWriteConfirmation + confirmAdministrationTarget
  in write_safety.go) wired into mutation create / invoice create.
- 5 local financial reports (new `report` command group, not generator-scaffolded):
  trial-balance, balance-sheet, profit-loss, vat-summary, outstanding-aging — computed
  from synced ledger balances and mutation rows, addressing the user's explicit #1 ask
  (balance, profit/loss visibility).
- Known Gaps documented in README.md and SKILL.md anti-triggers: no file/attachment
  support (confirmed real API limitation, not a CLI gap), one-administration-per-session
  scoping, and local reports framed as convenience views not certified figures.

## Deliberate scope changes from the approved Phase 1.5 manifest (with reasoning)
- **"Administration Safety Banner"** was approved as comparing the session's bound
  administration against a `--administration` flag on writes. Discovered mid-build
  that the API exposes no administration-selector param anywhere (writes are
  implicitly scoped to whichever administration the token belongs to) — the original
  design was not buildable. Reframed to: refuse an ambiguous write when the token is
  linked to more than one administration, unless `--company "<exact name>"` names the
  target. Same safety goal, grounded in real API data (GET /v1/administration's
  Company field). Surfaced to the user via AskUserQuestion; no response was received
  before the build needed to continue, so proceeded with the recommended default.
- **"Administration Portfolio Overview"** had the same underlying constraint (no
  per-administration data fan-out is possible with one session). Reframed to: list
  every linked administration (metadata) alongside the current session's own balance
  and outstanding-invoice figures, explicitly labeled as scoped to the authenticated
  administration only.

## Generator-level bug found and fixed in the printed CLI
`internal/store.UpsertBalance`/`upsertBalanceTx` could not store e-Boekhouden's
Code-keyed balance records (GET /v1/ledger/balances returns {Code, Type, Balance}
with no id/Id/ID/uuid/slug/name field, and the "ledger_id" column is NOT NULL with
no ledgerId field in the DTO at all). Fixed by falling back to "code"/"Code" for row
identity and resolving ledger_id via a lookup against the already-synced ledger
table by code. Regression tests added (internal/store/balance_id_test.go). This is a
systemic generator gap (extractObjectID's fallback list lacks "code" that the
sync-level extractID() already has) — retro candidate, not e-Boekhouden-specific.

## Other bugs found and fixed during hand-build
- TEXT-vs-INTEGER SQLite comparison bugs in relation_statement.go and
  ledger_history.go: comparing a bound TEXT parameter against a bare
  `json_extract(...)` result (no column affinity) silently never matches when the
  JSON value is a number. Fixed with explicit `CAST(... AS TEXT)`.
- Confusing "unable to open database file: out of memory (14)" error on first run:
  hand-written novel commands used `store.OpenReadOnly`, which doesn't create the DB
  file, unlike every other generated read command's `store.OpenWithContext`. Switched
  to match convention; empty-store results now render as clean `[]`/empty messages.
- A frontmatter/agent-context description truncation bug affecting 4 separate
  generated surfaces (SKILL.md frontmatter, .goreleaser.yaml brew description,
  agent_context.go, mcp/tools.go handleContext) — all pulled from the same
  `naming.CompactDescription`/`firstCompleteClauseUnder` path, which stops at the
  LAST comma-bounded clause under the length limit even when that clause is far
  short of the budget and no further comma/semicolon/colon exists later in the
  string. Patched all 4 files directly in the printed CLI with the full/appropriate
  text; root-caused but not fixed at the generator level (retro candidate, full
  repro: naming.CompactDescription() on a headline with exactly 2 commas near the
  start and no further clause punctuation before the end).

## Skipped / deferred
None — all approved manifest items shipped (with the two documented reframes above).

## Verification
go build, go vet, gofmt, go test ./... all clean. govulncheck clean after bumping
go.mod floor from 1.26.4 to 1.26.6 (closed 5 stdlib CVEs fixed in that range).
Full shipcheck umbrella: 6/6 legs PASS. Scorecard: 91/100, Grade A.
