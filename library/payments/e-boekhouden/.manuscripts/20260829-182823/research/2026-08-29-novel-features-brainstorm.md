## Customer model

**Persona 1: A solo bookkeeper who closes her own books every Friday afternoon before the weekend.**

*Today (without this CLI):* She logs into the e-Boekhouden web UI, opens her bank export in a spreadsheet tab, and manually keys each unmatched transaction into a new mutation, guessing which ledger account and VAT code to use from memory or by scrolling back through last month's entries in the UI. She has no way to ask "what did I book the last five office-supply purchases against?" without visually scanning a list.

*Weekly ritual:* Friday afternoon bank reconciliation — turning a stack of bank transactions into correctly-coded mutations (ledger + VAT split) before the weekend, then a quick balance/P&L glance to see how the week landed.

*Frustration:* Re-deriving the right ledger/VAT code for a recurring but not-quite-identical transaction description every single time, with no memory of her own past choices to lean on.

**Persona 2: An accountant who manages fifteen client administrations and does a Monday-morning round before calls start.**

*Today (without this CLI):* She logs into e-Boekhouden, switches administrations one at a time from a dropdown, and for each client checks the balance and outstanding invoices before her first client call of the day. Fifteen context switches, fifteen page loads, no single view of "which of my clients need attention today."

*Weekly ritual:* The Monday portfolio round — cycling through every managed administration to check cash position and unpaid invoices before the week's client calls.

*Frustration:* No aggregated view across administrations; and because the UI's active administration silently carries over, she has occasionally caught herself about to book an entry into the wrong client's books after a rushed switch.

**Persona 3: A small business owner who chases her own overdue invoices twice a month between everything else she does.**

*Today (without this CLI):* She opens the invoices list, sorts by date, and cross-references it against her memory of who's paid — because "outstanding" in the UI doesn't tell her whether a bank mutation covering that invoice has actually landed yet. She keeps a mental (sometimes literal sticky-note) list of "did I actually get paid for this one."

*Weekly ritual:* Reviewing which customers still owe money and whether a promised payment actually shows up as a booked mutation.

*Frustration:* No way to see, in one place, "this invoice has no matching payment" versus "this looks paid" without manually eyeballing two separate lists.

## Candidates (pre-cut)

| # | Name | Command (draft) | Description | Persona | Source | Inline kill/keep verdict |
|---|------|------------------|--------------|---------|--------|--------------------------|
| 1 | Mutation ledger/VAT suggest | `mutation suggest "<description>"` | Suggests the ledger + VAT code most often used for similar past mutation descriptions | 1 | (a) | Keep — reframed as mechanical FTS frequency match, not semantic NLP |
| 2 | Administration portfolio overview | `administration overview` | Loops all linked administrations, showing balance + outstanding-invoice count for each in one table | 2 | (a)/(b) | Keep — flag verifiability (multi-admin session behavior needs confirming against live account) |
| 3 | Administration default context | `administration use <id>` | Caches a default administration so flags don't need repeating | 2 | (a)/(b) | Kill — thin config convenience, not transcendent, not a differentiator |
| 4 | VAT-code compatibility validator | `mutation create --vat-check` | Warns if a VAT code is incompatible with a chosen ledger | domain nuance | (b) | Kill — reimplementation risk: no VAT/ledger compatibility endpoint or reference exists in the spec to source the rule from |
| 5 | Mutation split templates | `mutation template run <name>` | Saves and replays recurring multi-row mutation shapes (e.g. monthly rent split) | 1/3 | (b)/(e) | Keep to Pass 3 |
| 6 | Mutation-to-invoice payment matching | `mutation match-invoice` | Suggests which outstanding invoice a new payment mutation likely settles, by amount/date proximity | 3 | (c)/(a) | Keep to Pass 3 — flag verifiability (fuzzy match) |
| 7 | Relation statement | `relation statement <id>` | Full chronological history of invoices + mutations for one relation with running balance | 3 | (c) | Keep to Pass 3 |
| 8 | Ledger drill-down (general ledger inquiry) | `ledger history <code>` | Itemized chronological mutation rows for one ledger with a computed running balance (T-account view) | 1/2 | (b)/(c) | Keep to Pass 3 |
| 9 | Duplicate mutation detection | `mutation create` (pre-write guard) | Warns before creating a mutation that closely matches an existing one (same amount/date window/relation) | 1 | (b)/(e) | Keep to Pass 3 |
| 10 | Administration safety banner | `mutation create` / `invoice create` (guard) | Refuses a write if the active session's bound administration doesn't match the `--administration` the user targeted | 2 | (e)/(b) | Keep to Pass 3 |
| 11 | Balance snapshot diff | `report balance diff` | Period-over-period variance highlighting between two synced balance snapshots | 1/2 | (e) | Keep to Pass 3 — flag: close to absorbed row 14's snapshot claim |
| 12 | Prioritized collections list | `relation collections` | Outstanding invoices merged with relation contact info, sorted by overdue risk | 3 | (c)/(e) | Keep to Pass 3 — flag: close to absorbed row 14 (AR aging) |
| 13 | Invoice-mutation reconciliation health | `invoice reconcile` | Local audit: invoices with no matching payment mutation, and mutations referencing unknown invoice numbers | 3 | (c) | Keep to Pass 3 |
| 14 | Mutation CSV bulk import | `mutation import <file.csv>` | Bulk-creates mutations from a pre-mapped CSV (date/description/amount/ledger/VAT), `--dry-run` preview | 1 | (a)/(e) | Keep to Pass 3 — flag: risk of being a thin loop over `mutation create` |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Mutation ledger/VAT suggest | `mutation suggest "<description>"` | 8/10 | hand-code | Matches the input description against synced mutations' description field via SQLite FTS, ranks candidate ledger+VAT-code pairs by frequency, with no external dependencies. | Brief Data Layer notes mutations carry "per-row ledger/VAT/amount splits" as the core write complexity; Table Stakes flags VAT-code handling as an extensive nuance |
| 2 | Administration portfolio overview | `administration overview` | 8/10 | hand-code | Calls administration linked-list plus per-administration ledger balances-list and mutation outstanding-invoices, aggregating results into one table with no external dependencies. | Brief Top Workflows #5: "Multi-administration accountants switching between managed administrations"; Users section names accountants managing several clients |
| 3 | Relation statement | `relation statement <id>` | 8/10 | hand-code | Joins synced invoices and mutations by relation_id in local SQLite, ordered by date with a computed running balance, with no external dependencies. | Brief Top Workflows #3: "List/search outstanding invoices and relations to chase payments"; Data Layer names relations as a highest-value searchable entity |
| 4 | Ledger drill-down (T-account) | `ledger history <code>` | 8/10 | hand-code | Filters synced mutation rows by ledger code in local SQLite and computes a cumulative running balance ordered by date, with no external dependencies. | Brief API Identity: "mutations are journal entries (the core ledger data), ledgers are the chart of accounts"; Data Layer calls mutations "the highest-gravity table" |
| 5 | Administration safety banner | `mutation create` / `invoice create` (write guard) | 10/10 | hand-code | Reads the active session's bound administration id from local state and compares it against the `--administration` flag on every mutation/invoice create, refusing the write on mismatch, with no external dependencies. | Brief User Vision explicit ask: "prominently document safety warnings since the API allows direct manipulation of financial/accounting entries"; Top Workflows #5 multi-administration switching |
| 6 | Invoice-mutation reconciliation health | `invoice reconcile` | 9/10 | hand-code | Joins synced invoices against synced mutations in local SQLite to list invoices with no matching payment mutation and mutations referencing unknown invoice numbers, with no external dependencies. | Brief Top Workflows #3 (chasing outstanding invoices); Data Layer sync-cursor notes on mutations/invoices date-range tracking as the reconciliation substrate |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Administration default context (`administration use`) | Thin config convenience — caches a flag default, no aggregation or transcendent power | Administration portfolio overview |
| VAT-code compatibility validator | Reimplementation risk — no endpoint or reference data in the spec sources VAT/ledger compatibility rules; would require inventing business logic | Mutation ledger/VAT suggest |
| Mutation split templates | Weekly-use soft kill — recurring journal entries (rent, subscriptions) fire monthly, not weekly | Mutation ledger/VAT suggest |
| Mutation-to-invoice payment matching | Overlaps with invoice-mutation reconciliation's cross-reference mechanism, but relies on fuzzy amount/date proximity matching that's harder to verify deterministically | Invoice-mutation reconciliation health |
| Duplicate mutation detection | Redundant refinement of the same absorbed write-safety feature (row 15) as the administration safety banner, with weaker research backing (no direct brief evidence for double-booking risk) | Administration safety banner |
| Balance snapshot diff | Weekly-use soft kill (period review is monthly-cadence) and duplicates the historical-snapshot capability already claimed as absorbed row 14's added value | Ledger drill-down (T-account) |
| Prioritized collections list | Thin presentation/sort refinement of the already-absorbed AR/AP aging report (row 14) rather than an independently transcendent feature | Relation statement |
| Mutation CSV bulk import | Fails wrapper-vs-leverage test — a looped multiplier over the already-absorbed `mutation create` endpoint, equivalent to a shell loop a user could write themselves | Mutation ledger/VAT suggest |
