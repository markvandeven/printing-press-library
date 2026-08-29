// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

// Local financial reports computed from synced ledger balances and
// mutations. These are not native e-Boekhouden endpoints — the API only
// returns per-ledger balances and raw mutations; the classification into
// trial balance / balance sheet / profit-and-loss / VAT summary / aging
// buckets is done here from the "Category" the API itself assigns each
// ledger (BAL, VW, FIN, DEB, CRED, plus the VAT-return categories AF6,
// AF19, AFOVERIG, VOOR, BTWRC, AF — see the Category enum in the e-Boekhouden
// OpenAPI spec). Run `sync` (or `ledger get-balances`) first to populate the
// local balance table these reports read from.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"

	"github.com/spf13/cobra"
)

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Local financial reports computed from synced ledgers and mutations",
		Hidden:      true,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReportTrialBalanceCmd(flags))
	cmd.AddCommand(newReportBalanceSheetCmd(flags))
	cmd.AddCommand(newReportProfitLossCmd(flags))
	cmd.AddCommand(newReportVatSummaryCmd(flags))
	cmd.AddCommand(newReportOutstandingAgingCmd(flags))
	return cmd
}

type reportLedgerLine struct {
	Code        string  `json:"code"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category"`
	Balance     float64 `json:"balance"`
}

// loadLedgerBalances reads the synced "balance" table (populated from
// GET /v1/ledger/balances) joined with ledger metadata for descriptions.
// JSON key casing is extracted defensively (PascalCase or camelCase) since
// the OpenAPI spec's list-DTO examples use PascalCase while detail-DTO
// examples use camelCase and live behavior wasn't verified against a real
// account during generation.
func loadLedgerBalances(db *store.Store) ([]reportLedgerLine, error) {
	rows, err := db.DB().Query(`
		SELECT
			COALESCE(json_extract(b.data, '$.Code'), json_extract(b.data, '$.code')) AS code,
			COALESCE(json_extract(b.data, '$.Type'), json_extract(b.data, '$.type')) AS category,
			COALESCE(json_extract(b.data, '$.Balance'), json_extract(b.data, '$.balance')) AS balance,
			l.description AS description
		FROM balance b
		LEFT JOIN ledger l ON l.code = COALESCE(json_extract(b.data, '$.Code'), json_extract(b.data, '$.code'))`)
	if err != nil {
		return nil, fmt.Errorf("query balances: %w", err)
	}
	defer rows.Close()

	lines := []reportLedgerLine{}
	for rows.Next() {
		var code, category, desc sql.NullString
		var balance sql.NullFloat64
		if err := rows.Scan(&code, &category, &balance, &desc); err != nil {
			continue
		}
		lines = append(lines, reportLedgerLine{
			Code: code.String, Category: category.String, Balance: balance.Float64, Description: desc.String,
		})
	}
	return lines, rows.Err()
}

// extractItems parses a raw API response into its item list, tolerating
// both a bare JSON array and the {"items"/"Items": [...]} envelope every
// list endpoint on this API actually returns (confirmed against a live
// account) — e.g. GET /v1/mutation/invoice/outstanding responds
// {"items": [...], "count": N}, not a bare array as the OpenAPI spec's
// stubbed response schema implied.
func extractItems(data json.RawMessage) []map[string]any {
	var items []map[string]any
	if json.Unmarshal(data, &items) == nil {
		return items
	}
	var envelope struct {
		Items  []map[string]any `json:"items"`
		Items2 []map[string]any `json:"Items"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		if len(envelope.Items) > 0 {
			return envelope.Items
		}
		return envelope.Items2
	}
	return nil
}

func openReportDB(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("e-boekhouden-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

func printReportLines(cmd *cobra.Command, flags *rootFlags, lines []reportLedgerLine, note string, emptyMsg string) error {
	if flags.asJSON || flags.agent {
		envelope := map[string]any{"lines": lines, "note": note}
		return flags.printJSON(cmd, envelope)
	}
	if len(lines) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), emptyMsg)
		return nil
	}
	rows := make([][]string, 0, len(lines))
	var total float64
	for _, l := range lines {
		rows = append(rows, []string{l.Code, l.Description, l.Category, fmt.Sprintf("%.2f", l.Balance)})
		total += l.Balance
	}
	if err := flags.printTable(cmd, []string{"CODE", "DESCRIPTION", "CATEGORY", "BALANCE"}, rows); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %.2f\n", total)
	if note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), note)
	}
	return nil
}

func newReportTrialBalanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "trial-balance",
		Short:       "List the balance of every ledger account (from the last sync).",
		Example:     "  e-boekhouden-pp-cli report trial-balance --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openReportDB(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := loadLedgerBalances(db)
			if err != nil {
				return err
			}
			return printReportLines(cmd, flags, lines, "Computed from the last `sync` of GET /v1/ledger/balances — run `sync --full` to refresh.", "No synced ledger balances. Run `sync --full` first.")
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newReportBalanceSheetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "balance-sheet",
		Short: "Balance-sheet-side ledger balances (assets, liabilities, debtors, creditors — every category except profit-and-loss).",
		Long: "Filters the trial balance to every ledger category except 'VW' (profit and\n" +
			"loss): BAL (balance), FIN (liquid assets), DEB (debtors), CRED (creditors),\n" +
			"and the VAT-return categories. See e-Boekhouden's own Category enum.",
		Example:     "  e-boekhouden-pp-cli report balance-sheet --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openReportDB(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadLedgerBalances(db)
			if err != nil {
				return err
			}
			lines := []reportLedgerLine{}
			for _, l := range all {
				if l.Category != "VW" {
					lines = append(lines, l)
				}
			}
			return printReportLines(cmd, flags, lines, "Balance-sheet categories per e-Boekhouden's Category enum (everything except VW). Computed from the last `sync` — run `sync --full` to refresh.", "No synced balance-sheet ledger balances. Run `sync --full` first.")
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newReportProfitLossCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "profit-loss",
		Short: "Profit-and-loss ledger balances (category VW) and their net total.",
		Long: "Filters the trial balance to category 'VW' (Winst en verlies / profit and\n" +
			"loss). The net total's sign reflects e-Boekhouden's own balance convention\n" +
			"as returned by the API — this was not independently verified against a live\n" +
			"account during generation, so cross-check the sign against the web UI before\n" +
			"relying on it.",
		Example:     "  e-boekhouden-pp-cli report profit-loss --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openReportDB(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadLedgerBalances(db)
			if err != nil {
				return err
			}
			lines := []reportLedgerLine{}
			for _, l := range all {
				if l.Category == "VW" {
					lines = append(lines, l)
				}
			}
			return printReportLines(cmd, flags, lines, "Category VW (profit and loss) per e-Boekhouden's own Category enum. Sign convention is as returned by the API — verify against the web UI. Computed from the last `sync` — run `sync --full` to refresh.", "No synced profit-and-loss ledger balances. Run `sync --full` first.")
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type vatSummaryLine struct {
	VatCode  string  `json:"vatCode"`
	Count    int     `json:"count"`
	Amount   float64 `json:"amount"`
	VatTotal float64 `json:"vatTotal"`
}

func newReportVatSummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "vat-summary",
		Short: "Aggregate synced mutation rows by VAT code: row count, total amount, total VAT.",
		Long: "Computed by summing mutation.rows[].amount and .vatAmount grouped by\n" +
			"rows[].vatCode across every synced mutation. Coverage note: the per-line VAT\n" +
			"breakdown (rows) is only present on a GET /v1/mutation/{id} detail response,\n" +
			"not the list response `sync` uses — confirmed against a live account, a plain\n" +
			"`sync --full` alone gives 0 rows here. `mutation get-id <id>` does\n" +
			"write-through cache its full detail response (including rows) into the local\n" +
			"store, so this report's coverage grows as you view individual mutations;\n" +
			"until then, treat any totals shown as a partial sample, not a full period.",
		Example:     "  e-boekhouden-pp-cli report vat-summary --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openReportDB(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT je.value ->> '$.vatCode' AS vat_code,
				       je.value ->> '$.amount' AS amount,
				       je.value ->> '$.vatAmount' AS vat_amount
				FROM mutation m, json_each(m.data, '$.rows') je
				WHERE je.value ->> '$.vatCode' IS NOT NULL`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			totals := map[string]*vatSummaryLine{}
			var order []string
			for rows.Next() {
				var code sql.NullString
				var amount, vatAmount sql.NullFloat64
				if err := rows.Scan(&code, &amount, &vatAmount); err != nil {
					continue
				}
				if !code.Valid {
					continue
				}
				if _, ok := totals[code.String]; !ok {
					totals[code.String] = &vatSummaryLine{VatCode: code.String}
					order = append(order, code.String)
				}
				totals[code.String].Count++
				totals[code.String].Amount += amount.Float64
				totals[code.String].VatTotal += vatAmount.Float64
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			summary := make([]vatSummaryLine, 0, len(order))
			for _, code := range order {
				summary = append(summary, *totals[code])
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{
					"lines": summary,
					"note":  "Coverage is partial: a plain `sync --full` doesn't populate per-line VAT detail (only `mutation get-id <id>` does, via its write-through cache), so totals only reflect mutations you've individually viewed. Not a substitute for your official VAT return.",
				})
			}
			if len(summary) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No VAT-coded mutation rows in the local store yet. Run `mutation get-id <id>` on individual mutations to populate coverage — see `report vat-summary --help`.")
				return nil
			}
			out := make([][]string, 0, len(summary))
			for _, s := range summary {
				out = append(out, []string{s.VatCode, fmt.Sprint(s.Count), fmt.Sprintf("%.2f", s.Amount), fmt.Sprintf("%.2f", s.VatTotal)})
			}
			return flags.printTable(cmd, []string{"VAT CODE", "ROWS", "AMOUNT", "VAT TOTAL"}, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// agingBuckets buckets a set of outstanding-invoice items by days since
// invoice date into the standard 0-30/31-60/61-90/90+ windows.
func agingBuckets(items []map[string]any, now time.Time) map[string][]map[string]any {
	buckets := map[string][]map[string]any{"0-30": {}, "31-60": {}, "61-90": {}, "90+": {}, "unknown": {}}
	for _, it := range items {
		dateStr := firstNonEmpty(it, "Date", "date", "InvoiceDate", "invoiceDate")
		age := daysSince(dateStr, now)
		bucket := "unknown"
		switch {
		case age < 0:
			bucket = "unknown"
		case age <= 30:
			bucket = "0-30"
		case age <= 60:
			bucket = "31-60"
		case age <= 90:
			bucket = "61-90"
		default:
			bucket = "90+"
		}
		buckets[bucket] = append(buckets[bucket], it)
	}
	return buckets
}

func newReportOutstandingAgingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outstanding-aging",
		Short: "Outstanding receivables (AR) and payables (AP) from the live API, bucketed by age.",
		Long: "Calls the native GET /v1/mutation/invoice/outstanding endpoint once for\n" +
			"debtors (credDeb=D, accounts receivable) and once for creditors\n" +
			"(credDeb=C, accounts payable) — both are required query values, confirmed\n" +
			"against the live API — and buckets each into 0-30 / 31-60 / 61-90 / 90+ day\n" +
			"windows based on invoice date.",
		Example:     "  e-boekhouden-pp-cli report outstanding-aging --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			receivablesData, err := c.Get(cmd.Context(), "/v1/mutation/invoice/outstanding", map[string]string{"credDeb": "D"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			receivables := extractItems(receivablesData)

			payablesData, err := c.Get(cmd.Context(), "/v1/mutation/invoice/outstanding", map[string]string{"credDeb": "C"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			payables := extractItems(payablesData)

			now := currentDateForAging()
			order := []string{"0-30", "31-60", "61-90", "90+", "unknown"}
			receivableBuckets := agingBuckets(receivables, now)
			payableBuckets := agingBuckets(payables, now)

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{
					"receivables": receivableBuckets,
					"payables":    payableBuckets,
					"note":        "Aging computed locally from the live GET /v1/mutation/invoice/outstanding response (once per credDeb value), bucketed by days since invoice date.",
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Outstanding receivables (money owed to you):")
			for _, b := range order {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s days: %d invoice(s)\n", b, len(receivableBuckets[b]))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Outstanding payables (money you owe):")
			for _, b := range order {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s days: %d invoice(s)\n", b, len(payableBuckets[b]))
			}
			return nil
		},
	}
	return cmd
}

func currentDateForAging() time.Time {
	return time.Now().UTC()
}

// daysSince returns whole days between dateStr (parsed as RFC3339 or a bare
// YYYY-MM-DD date) and now. Returns -1 when dateStr can't be parsed, which
// callers bucket as "unknown" rather than silently misclassifying it.
func daysSince(dateStr string, now time.Time) int {
	if dateStr == "" {
		return -1
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return int(now.Sub(t).Hours() / 24)
		}
	}
	return -1
}
