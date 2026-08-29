# e-Boekhouden CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/get/create/update/delete cost centers | Mantix PHP client, spec CRUD | Typed CRUD commands + SQLite cache | Offline list, --json, --dry-run |
| 2 | List/get/create invoices, search/filter | Mantix PHP client, spec | Typed commands with full filter-DSL flags | FTS search across synced invoices |
| 3 | List invoice templates, email templates | spec | Typed list commands | Cached locally |
| 4 | List/get/create/update ledgers (chart of accounts) | Mantix PHP client, spec | Typed CRUD + local cache | Offline chart-of-accounts browsing |
| 5 | Get ledger balances (all ledgers, or per-ledger) with filters | spec | Typed commands | Snapshot history in SQLite for trend queries |
| 6 | List/get/create/update members (clubs/associations) | spec | Typed CRUD | N/A beyond parity |
| 7 | List/get/create mutations (journal entries), get outstanding invoices | Mantix PHP client, matisup10 MCP, spec | Typed commands, mutation create with per-row ledger/VAT splits and --dry-run | Local FTS search on description; safety-gated create |
| 8 | List/get/create/update/delete products, list product groups | Mantix PHP client, spec | Typed CRUD | Offline cache |
| 9 | List/get/create/update relations (customers/suppliers) | Mantix PHP client, spec | Typed CRUD + FTS on name/company | Offline search across relations |
| 10 | List units | spec | Typed list command | N/A beyond parity |
| 11 | List administrations, list linked administrations (multi-admin/accountant support) | spec | Typed list commands | N/A beyond parity |
| 12 | Session login/logout with API-token exchange | spec, all community clients | Automatic session handshake, session token caching + auto-refresh on expiry, correct no-Bearer-prefix header | Transparent to the user — no manual session management like competitors require |
| 13 | Filter-suffix DSL ([eq]/[gt]/[range]/[like]/etc.) on every list command | spec | Native CLI flags per filter operator, not raw passthrough | Discoverable via --help, not buried in docs |
| 14 | Trial balance / P&L / balance sheet / VAT summary / AR-AP aging reports | matisup10 e-Boekhouden-MCP (computed, not native API) | Computed locally from synced ledgers+mutations, same report set | Offline, historical snapshots, --json/--csv for spreadsheets |
| 15 | Write-safety gating on mutations (dual-lock: explicit enable + confirm) | matisup10 e-Boekhouden-MCP | --dry-run default + explicit --confirm flag + prominent README/SKILL safety warnings | Same protection model, built into CLI defaults instead of opt-in env var |
| 16 | PDF/document download URLs for invoices | Mantix PHP client | Not applicable — no attachment/file endpoints exist in the v1 API at all | N/A — documented as a known API limitation, not a CLI gap. Status: **(not built — no API surface exists)** |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|--------------------------|
| 1 | Administration safety banner | `mutation create` / `invoice create` (write guard) | 10/10 | hand-code | Requires reading the locally-cached bound-administration state from the session and comparing it against the target `--administration` before any write is sent — no API endpoint exposes "which administration is this session scoped to" as a pre-write check |
| 2 | Invoice-mutation reconciliation health | `invoice reconcile` | 9/10 | hand-code | Requires a local join between synced invoices and synced mutations to find invoices with no matching payment and mutations referencing unknown invoice numbers — no single API call produces this cross-reference |
| 3 | Mutation ledger/VAT suggest | `mutation suggest "<description>"` | 8/10 | hand-code | Requires local SQLite FTS frequency-ranking of past mutation descriptions against ledger+VAT-code pairs the user has actually used before — no reference/compatibility endpoint exists in the API to source this from |
| 4 | Administration portfolio overview | `administration overview` | 8/10 | hand-code | Requires fanning out across every linked administration's balances and outstanding invoices and merging into one table — the API only exposes one administration's data per session |
| 5 | Relation statement | `relation statement <id>` | 8/10 | hand-code | Requires a local join of synced invoices and mutations by relation_id with a computed running balance ordered by date — no endpoint returns a unified relation ledger history |
| 6 | Ledger drill-down (T-account view) | `ledger history <code>` | 8/10 | hand-code | Requires filtering synced mutation rows by ledger code and computing a cumulative running balance locally — the API's balance endpoints return only point-in-time totals, not itemized history with a running total |

## Stub / Known-Gap Items
None. Every planned feature above (absorbed + transcendence) is fully buildable against the resolved spec. Row 16 (file/attachment handling) is not a stub — it documents a genuine API limitation (no attachment endpoints exist anywhere in the v1 spec), matching the user's own expectation stated in the briefing.
