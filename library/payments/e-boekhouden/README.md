# e-Boekhouden CLI

**Balance, profit & loss, and journal entries from your terminal — with local financial reports and write-safety guards the API doesn't give you.**

e-Boekhouden's v1 REST API has no CLI today, only library wrappers in PHP, Rust, Node, and Python. This CLI absorbs every endpoint (all 24) and adds local financial reporting (trial balance, P&L, balance sheet, VAT summary, AR/AP aging) computed offline from your synced ledgers and mutations, plus reconciliation and safety-gated writes for a service where every mutation is a real accounting entry.

## Install

The recommended path installs both the `e-boekhouden-pp-cli` binary and the `pp-e-boekhouden` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install e-boekhouden
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install e-boekhouden --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install e-boekhouden --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install e-boekhouden --agent claude-code
npx -y @mvanhorn/printing-press-library install e-boekhouden --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/e-boekhouden-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-e-boekhouden --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-e-boekhouden --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install e-boekhouden --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/e-boekhouden-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EBOEKHOUDEN_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "e-boekhouden": {
      "command": "e-boekhouden-pp-mcp",
      "env": {
        "EBOEKHOUDEN_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth is a two-step handshake: create a long-lived API token once in your e-Boekhouden account settings, then the CLI exchanges it for a short-lived session token automatically on every run — you never handle the session token yourself, and it never touches disk as a long-lived credential.

## Quick Start

```bash
# Confirm the session handshake works and the API is reachable
e-boekhouden-pp-cli doctor

# Pull ledgers, mutations, invoices, and relations into the local store
e-boekhouden-pp-cli sync --full

# Check current balance across all ledgers
e-boekhouden-pp-cli report trial-balance --json

# See profit and loss computed from synced mutations
e-boekhouden-pp-cli report profit-loss --json

# Preview a new journal entry before writing it for real
e-boekhouden-pp-cli mutation create --date 2026-01-15 --ledger-id 1300 --type 7 --description "Office supplies" --rows '[{"ledgerId":4200,"vatCode":"HOOG_INK_21","amount":25.50}]' --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Safety-gated writes
- **`mutation create`** — Refuses to actually write a mutation or invoice without --confirm, and blocks an ambiguous write across multiple linked administrations unless --company names the exact target.

  _Use --dry-run to preview any mutation/invoice write, and add --confirm only once you're sure — add --company on accounts linked to more than one administration to avoid silently booking an entry into the wrong client's books._

  ```bash
  e-boekhouden-pp-cli mutation create --date 2026-01-15 --ledger-id 1300 --type 7 --description "Office supplies" --rows '[{"ledgerId":4200,"vatCode":"HOOG_INK_21","amount":25.50}]' --dry-run
  ```

### Local state that compounds
- **`invoice reconcile`** — Lists invoices with no matching payment mutation, and mutations that reference an invoice number the CLI doesn't recognize.

  _Reach for this when chasing overdue invoices — it tells you which ones genuinely have no recorded payment, not just which the UI marks outstanding._

  ```bash
  e-boekhouden-pp-cli invoice reconcile --json --select unmatchedInvoices.number,unmatchedInvoices.relationName
  ```
- **`mutation suggest`** — Suggests the ledger and VAT code most often used for past mutations with a similar description.

  _Use before booking a recurring but not-identical transaction to avoid re-deriving the right ledger/VAT code from memory every time._

  ```bash
  e-boekhouden-pp-cli mutation suggest "Office supplies - Staples" --json
  ```
- **`administration overview`** — Lists every administration linked to this API token, alongside ledger balances and outstanding invoices for the one administration this session is actually authenticated against.

  _Use this to see every administration linked to an API token plus that token's own balance/outstanding figures, instead of two separate calls._

  ```bash
  e-boekhouden-pp-cli administration overview --json
  ```
- **`relation statement`** — Full chronological history of invoices and mutations for one relation, with a computed running balance.

  _Use this to answer "has this customer actually paid what they owe" in one call instead of cross-referencing two separate lists._

  ```bash
  e-boekhouden-pp-cli relation statement 789012 --json
  ```
- **`ledger history`** — Itemized chronological mutation rows for one ledger account with a computed running balance.

  _Use this when you need to see how a specific ledger account arrived at its current balance, not just what the balance is today._

  ```bash
  e-boekhouden-pp-cli ledger history 1300 --json --select rows.date,rows.description,rows.runningBalance
  ```

## Recipes


### Monday portfolio round across all clients

```bash
e-boekhouden-pp-cli administration overview --json
```

One call replaces switching between every managed administration to check balance and outstanding invoices.

### Chase real unpaid invoices, not just UI-flagged ones

```bash
e-boekhouden-pp-cli invoice reconcile --agent --select unmatchedInvoices.number,unmatchedInvoices.relationName,unmatchedInvoices.amount
```

Narrows the reconciliation report to just the fields needed to act, since invoice/mutation payloads are large.

### Suggest a ledger/VAT code for a recurring expense

```bash
e-boekhouden-pp-cli mutation suggest "Office supplies - Staples"
```

Surfaces the ledger + VAT code you've used most often for similar descriptions before you book the entry.

### Full customer payment history in one call

```bash
e-boekhouden-pp-cli relation statement 789012
```

Joins invoices and mutations for one relation with a running balance instead of cross-referencing two lists.

### See how a ledger account reached its current balance

```bash
e-boekhouden-pp-cli ledger history 1300
```

Itemized mutation history with a running balance, not just the point-in-time total the API's balance endpoint returns.

## Usage

Run `e-boekhouden-pp-cli --help` for the full command reference and flag list.

## Commands

### administration

Manage administration

- **`e-boekhouden-pp-cli administration get`** - Get all administrations managed by the authorized accountant.
- **`e-boekhouden-pp-cli administration get-linked`** - Get all administrations that are linked to the authorized administration.<br/>Please note that this endpoint will always return administrations linked to the administration of the user that created the API credentials.

### costcenter

Manage costcenter

- **`e-boekhouden-pp-cli costcenter create-cost-center`** - Create a new cost center.
- **`e-boekhouden-pp-cli costcenter delete-cost-center`** - Delete a cost center.
- **`e-boekhouden-pp-cli costcenter get-cost-center`** - Get a cost center.
- **`e-boekhouden-pp-cli costcenter get-cost-centers`** - Get all cost centers.
- **`e-boekhouden-pp-cli costcenter update-cost-center`** - Update a cost center.

### emailtemplate

Manage emailtemplate

- **`e-boekhouden-pp-cli emailtemplate`** - Get all email templates.

### invoice

Manage invoice

- **`e-boekhouden-pp-cli invoice create`** - Create a new invoice.
- **`e-boekhouden-pp-cli invoice get`** - Search for invoices.
- **`e-boekhouden-pp-cli invoice get-id`** - Get an invoice.

### invoicetemplate

Manage invoicetemplate

- **`e-boekhouden-pp-cli invoicetemplate`** - Get all invoice templates.

### ledger

Manage ledger

- **`e-boekhouden-pp-cli ledger create`** - Create a new ledger.
- **`e-boekhouden-pp-cli ledger get`** - Get all ledgers.
- **`e-boekhouden-pp-cli ledger get-balances`** - Get the balances on all ledgers with optional filters.
- **`e-boekhouden-pp-cli ledger get-id`** - Get a ledger.
- **`e-boekhouden-pp-cli ledger update`** - Update a ledger.

### member

Manage member

- **`e-boekhouden-pp-cli member create`** - Create a new member (only available to clubs or associations).
- **`e-boekhouden-pp-cli member get`** - Get all members (only available to clubs or associations).
- **`e-boekhouden-pp-cli member get-id`** - Get a member (only available to clubs or associations).
- **`e-boekhouden-pp-cli member update`** - Update an existing member (only available to clubs or associations).

### mutation

Manage mutation

- **`e-boekhouden-pp-cli mutation create`** - Create a new mutation.
- **`e-boekhouden-pp-cli mutation get`** - Get all mutations.
- **`e-boekhouden-pp-cli mutation get-id`** - Get a mutation by id.
- **`e-boekhouden-pp-cli mutation get-outstanding-invoices`** - Get all outstanding invoices.

### product

Manage product

- **`e-boekhouden-pp-cli product create`** - Create a new product.
- **`e-boekhouden-pp-cli product delete`** - Delete a product.
- **`e-boekhouden-pp-cli product get`** - Get all products.
- **`e-boekhouden-pp-cli product get-group`** - Get all product groups.
- **`e-boekhouden-pp-cli product get-id`** - Get a product.
- **`e-boekhouden-pp-cli product update`** - Update a product.

### relation

Manage relation

- **`e-boekhouden-pp-cli relation create`** - Create a new relation.
- **`e-boekhouden-pp-cli relation get`** - Get all relations.
- **`e-boekhouden-pp-cli relation get-id`** - Get a relation.
- **`e-boekhouden-pp-cli relation update`** - Update an existing relation.

### session

Manage session

- **`e-boekhouden-pp-cli session end`** - Revokes the session token.
- **`e-boekhouden-pp-cli session start`** - Start a new session. The response session token can be used as the `Authorization` header.

### swagger

Manage swagger

- **`e-boekhouden-pp-cli swagger get`** - Get
- **`e-boekhouden-pp-cli swagger list`** - List

### unit

Manage unit

- **`e-boekhouden-pp-cli unit`** - Get all units.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
e-boekhouden-pp-cli administration get

# JSON for scripting and agents
e-boekhouden-pp-cli administration get --json

# Filter to specific fields
e-boekhouden-pp-cli administration get --json --select id,name,status

# Dry run — show the request without sending
e-boekhouden-pp-cli administration get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
e-boekhouden-pp-cli administration get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
e-boekhouden-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/e-boekhouden-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EBOEKHOUDEN_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `e-boekhouden-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `e-boekhouden-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EBOEKHOUDEN_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or session errors after the CLI has been idle for a while** — Session tokens are short-lived; re-run any command and the CLI re-authenticates automatically using your cached API token — no manual login needed
- **"no permission to access the requested resource" (SECURITY_001/002)** — Your API token's account doesn't have access to that administration or resource — check account settings on e-Boekhouden.nl
- **"refusing to create a mutation/invoice without --confirm"** — This is intentional: run with --dry-run to preview, then add --confirm once you're sure you want to write the real entry
- **"this API token is linked to multiple administrations"** — Pass --company "<exact name>" on mutation create / invoice create to confirm which administration this write targets
- **PAGE_001 / PAGE_002 errors on list commands** — limit must be between 1 and 2000 and offset must be >= 0 — adjust --limit/--offset

## Known Gaps

- **No file/attachment support.** No endpoint in the e-Boekhouden v1 API accepts a file upload or attachment — invoices and mutations cannot carry attached documents via this CLI or any other client of this API. This is a real API limitation, not a missing CLI feature.
- **One administration per session.** Each API token is scoped to exactly one administration, so `administration overview`, `mutation create --company`, and `invoice create --company` can only see or target the administrations linked to *that* token — there is no way to query or write to a different administration without switching `EBOEKHOUDEN_API_TOKEN`.
- **Local financial reports are convenience views, not certified figures.** `report trial-balance`, `report balance-sheet`, `report profit-loss`, and `report vat-summary` are computed from your last `sync` and, for VAT/ledger-history commands, from raw recorded amounts with no debit/credit sign inference. Cross-check totals against the e-Boekhouden web UI or your accountant before relying on them for filing.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**eboekhouden-rest-api**](https://github.com/Mantix/eboekhouden-rest-api) — PHP
- [**e-Boekhouden-MCP**](https://github.com/matisup10/e-Boekhouden-MCP) — TypeScript
- [**eboekhouden**](https://github.com/NixySoftware/eboekhouden) — Rust
- [**eboekhouden-php**](https://github.com/IntVent/eboekhouden-php) — PHP
- [**eBoekhouden-Node**](https://github.com/Vultwo/eBoekhouden-Node) — JavaScript
- [**eboekhouden-client-python**](https://github.com/Stichting-Verbonden-Stilte/eboekhouden-client-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
