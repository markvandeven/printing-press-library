# e-Boekhouden CLI Brief

## API Identity
- Domain: Dutch/Belgian online bookkeeping SaaS (e-Boekhouden.nl). The v1 REST API (OpenAPI 3.1.1, official, at `https://api.e-boekhouden.nl/openapi/v1.json`) covers administrations, cost centers, invoices, invoice/email templates, ledgers (general accounts) + balances, members (clubs/associations), mutations (the core bookkeeping journal entries), products, relations (customers/suppliers), and session auth. 24 real endpoints (excluding the swagger UI routes).
- Users: Dutch/Belgian small businesses, bookkeepers, and accountants (multi-administration support for accountants managing several clients) who use e-Boekhouden as their ledger and want programmatic/agent access to balances, P&L, and journal entries.
- Data profile: Financial/accounting records — ledgers (chart of accounts), mutations (journal entries with per-row ledger/VAT/amount splits), invoices, relations (contacts), products, cost centers. Read-heavy for reporting; mutation and invoice creation are the primary write paths. No endpoint anywhere in the spec supports file/attachment upload — confirmed API limitation, not a CLI gap.

## Reachability Risk
- None. Official OpenAPI spec fetched directly from the vendor at `api.e-boekhouden.nl/openapi/v1.json` (found by following the Scalar reference page's `sources` config after `/scalar/openapi/v1.json` 404'd). No bot-protection, no 403s found in research on the v1 REST API. Community repos target it directly with no blocked/rate-limit complaints found.

## Auth (session_handshake — critical, non-standard header)
1. User creates a long-lived **API token** in e-Boekhouden account settings (shown once).
2. CLI calls `POST /v1/session` with `{"accessToken": "<API_TOKEN>", "source": "<=10 char id>"}` to obtain a short-lived session `token` + `expiresIn` (seconds).
3. Every subsequent request sends `Authorization: <SESSION_TOKEN>` — **no `Bearer ` prefix**, despite the security scheme being named "bearer". This is a real footgun: naive spec-driven auth emission would prepend `Bearer `, breaking every call.
4. `DELETE /v1/session` revokes the session token (logout).
- Canonical env var: `EBOEKHOUDEN_API_TOKEN` (long-lived token the user creates). Session token is ephemeral/internal — not user-supplied, must be cached and refreshed by the CLI (`expiresIn` seconds), not persisted as a long-lived credential.
- `source` field is a required 1-10 char identifier; use a fixed value like `pp-cli`.

## Top Workflows
1. Check current balance and profit/loss for the active administration (`ledger balances`, aggregated mutation reads) — the user's stated #1 purpose.
2. Record a mutation (journal entry) — invoice payment received, expense booked, bank transaction — with per-row ledger/VAT splits (`mutation create`).
3. List/search outstanding invoices and relations to chase payments (`mutation invoice outstanding`, `relation list`).
4. Create and send an invoice to a relation (`invoice create`).
5. Multi-administration accountants switching between managed administrations (`administration list` / `administration linked`).

## Table Stakes (from absorbed competitor tools)
- CRUD for cost centers, products, relations, ledgers, members (PATCH-based partial update, not full PUT).
- Mutation + invoice creation with line-item/row detail and VAT-code handling (extensive official VAT code tables per NL/BE).
- Filtering DSL support: every list endpoint's filterable params accept `[eq]/[not_eq]/[like]/[not_like]/[gt]/[gte]/[lt]/[lte]/[range]` suffix syntax — must be exposed as CLI flags, not just passthrough.
- Pagination via `limit` (1-2000, default 100) and `offset` (default 0) on all search endpoints.
- Session lifecycle management (login/logout), auto re-auth on session expiry.
- **Financial reporting from local data**: the one existing MCP server for this API (matisup10/e-Boekhouden-MCP) already ships trial balance, P&L, balance sheet, VAT summary, and AR/AP aging as computed reports — not native API endpoints. This is our Priority 0/2 bar to match, and SQLite makes it easy to beat (historical trend, offline).
- **Write-safety gating**: the same MCP server requires an explicit env-var opt-in (`EBOEKHOUDEN_MCP_ENABLE_WRITE_TOOLS=true`) plus a per-call `confirm: true` flag before any mutation. Our `--dry-run` default + explicit confirmation pattern already matches this; document it prominently per the user's explicit request for safety warnings.

## Data Layer
- Primary entities: `administrations`, `costcenters`, `ledgers` (+ `ledger_balances` computed), `mutations` (journal entries, the highest-gravity table), `invoices`, `relations`, `products`, `members`, `units`, `emailtemplates`, `invoicetemplates`.
- Sync cursor: `mutations` and `invoices` support date-range filters (`date[gte]`/`date[lte]`) — use incremental sync keyed on mutation/invoice date or id.
- FTS/search: relations (name/company), mutations (description), invoices (invoice number) are the highest-value searchable fields.
- Local reporting layer: derive P&L, balance sheet, trial balance, and VAT summary from synced `mutations` + `ledgers` joined by ledger code — this is the transcendence foundation the user explicitly asked for (balance, profit/loss).

## Codebase Intelligence
- Source: research via GitHub topic search + README fetches (no DeepWiki-eligible large repo found; clients are small).
- Community clients found: `Mantix/eboekhouden-rest-api` (PHP, targets the same v1 REST API — full CRUD across all resources, session auth, filter DSL, PDF download URLs, direct invoice email delivery), `NixySoftware/eboekhouden` (Rust, OpenAPI-generated client for the same spec), `IntVent/eboekhouden-php`, `Vultwo/eBoekhouden-Node`, `Stichting-Verbonden-Stilte/eboekhouden-client-python` — all wrap the new v1 REST API. Several older repos (`onetoweb/eboekhouden`, `innovato/eBoekhoudenApi`, `raylanconsulting/eboekhouden`, `HollandOpenSource/...`) target the legacy **SOAP** API and are not relevant to v1.
- `matisup10/e-Boekhouden-MCP`: existing MCP server, 34 read tools + 15 opt-in write tools, computed financial reports, dual-lock write safety (env flag + per-call confirm). This is the tool to beat.
- Auth pattern confirmed independently by this MCP's docs: session token cached, HTTPS-only, no Bearer prefix language contradicted — MCP describes "short-lived session" matching the spec's own Quick Start.
- No GitHub issues found describing 403/blocking/deprecation for the v1 REST API.

## User Vision
- Purpose: provide balance, profit/loss, and mutation (journal entry) information, including creating mutations.
- Expected limitation: file/attachment upload is likely not possible via the API — **confirmed true**: no attachment endpoint exists anywhere in the v1 spec.
- Explicit ask: implement all API methods (all 24 endpoints), and prominently document safety warnings since the API allows direct manipulation of financial/accounting entries.

## Product Thesis
- Name: e-boekhouden (slug: `e-boekhouden`)
- Why it should exist: No CLI exists for the new v1 REST API at all (only PHP/Rust/Node/Python library wrappers and one MCP server). A CLI gives bookkeepers and agents direct terminal/agent access to balances, P&L, and journal entries with offline SQLite-backed reporting (trial balance, P&L, balance sheet, VAT summary, AR/AP aging) that the API itself doesn't natively expose, plus the filter-DSL and safety-gated mutation writes every competitor has had to hand-roll.

## Build Priorities
1. Data layer + sync for all 11 resource types, with correct session_handshake auth (no Bearer prefix) and session token caching/refresh.
2. Absorb all 24 endpoints: full CRUD (GET/POST/PATCH/DELETE where the API supports it) with the filter-DSL flags and pagination on every list command.
3. Transcendence: local financial reports (trial balance, P&L, balance sheet, VAT summary, AR/AP aging) computed from synced ledgers+mutations; safety-gated mutation/invoice creation with explicit confirmation and prominent documentation warnings.
