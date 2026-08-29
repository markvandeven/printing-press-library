# Phase 4.95: Native Code Review

Performed as a direct manual security/correctness pass by the build agent
(running as a forked continuation; the harness's /review slash command and
the code-review skill were not invoked to avoid further sub-agent spawning
from within a fork).

## Scope
internal/cli/{mutation_suggest,invoice_reconcile,relation_statement,ledger_history,
administration_overview,report,write_safety,mutation_create,invoice_create}.go,
internal/client/{client,session}.go, internal/store/store.go (balance ID/ledger_id fix).

## Findings and outcomes

1. **SQL injection risk** — checked every hand-written query for string
   concatenation of user input. All user-controlled values go through
   parameterized `?` placeholders; the one `fmt.Sprintf`-built query
   (mutation_suggest.go's LIKE-clause loop) only interpolates static SQL
   fragments, never user input directly. No findings.
2. **Credential handling** — the e-Boekhouden session token is cached to
   `~/.config/e-boekhouden-pp-cli/session.json` with 0600 permissions inside a
   0700 directory; the long-lived EBOEKHOUDEN_API_TOKEN is never written to
   disk by this CLI's own code (only read from the env var / config, per the
   generator's standard config.go). No findings.
3. **No exec/unsafe usage** in any hand-written file. No findings.
4. **Write-safety guard (write_safety.go)** — `confirmAdministrationTarget`
   fails open (returns nil, allowing the write) if the GET /v1/administration
   metadata lookup itself errors, rather than blocking the write. This is a
   deliberate choice (documented in the code comment) so a transient network
   blip on the metadata check doesn't block a legitimate write that would
   otherwise surface its own auth error from the real POST. Reviewed and kept
   as-is — the requireWriteConfirmation --confirm gate is the primary control
   and is fail-closed.
5. **Generator-level bug found and fixed in-place** (see build log): `UpsertBalance`/
   `upsertBalanceTx` in internal/store/store.go could not store e-Boekhouden's
   Code-keyed balance records at all (missing-id error, then a NOT NULL
   violation on ledger_id). Fixed with tests; flagged as a retro candidate
   since the same generator gap (extractObjectID's fallback list omits "code"/
   "gid"/"sid"/"uid"/"guid"/"key" that the sync-level extractID() fallback
   list already has) likely affects any printed CLI whose typed resource is
   keyed by a non-"id" field.

Status: complete, 0 blocking findings, 1 generator-level retro candidate filed
(see build log for full repro).
