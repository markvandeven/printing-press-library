// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"

	"github.com/spf13/cobra"
)

type ledgerHistoryRow struct {
	Date           string  `json:"date"`
	MutationID     string  `json:"mutationId"`
	Description    string  `json:"description,omitempty"`
	Amount         float64 `json:"amount"`
	RunningBalance float64 `json:"runningBalance"`
}

func newNovelLedgerHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "history <ledger-code>",
		Short: "Itemized chronological mutation rows for one ledger account with a computed running balance.",
		Long: "Lists every synced mutation whose top-level ledger matches this ledger (by\n" +
			"code), in date order, with a running total of the raw recorded amounts. Only\n" +
			"the mutation's own ledger is matched — the per-line ledger/VAT breakdown\n" +
			"(rows) is only available on a GET /v1/mutation/{id} detail fetch, not the\n" +
			"list response `sync` uses, so counter-ledger rows aren't included. This is a\n" +
			"local computation for drill-down purposes, not a certified accounting\n" +
			"balance — amounts are summed as recorded by the API with no debit/credit\n" +
			"sign inference applied. Cross-check the total against `ledger balance` for\n" +
			"the authoritative figure.",
		Example:     "  e-boekhouden-pp-cli ledger history 1300 --json --select rows.date,rows.description,rows.runningBalance",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ledgerCode := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("e-boekhouden-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var ledgerID sql.NullString
			if err := db.DB().QueryRowContext(cmd.Context(),
				`SELECT id FROM ledger WHERE code = ?`, ledgerCode).Scan(&ledgerID); err != nil || !ledgerID.Valid {
				return fmt.Errorf("no synced ledger found with code %q — run `sync --full` first, or check `ledger list`", ledgerCode)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT m.id, m.date, m.description, COALESCE(json_extract(m.data, '$.amount'), 0)
				FROM mutation m WHERE m.ledger_id = ?`,
				ledgerID.String)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			history := []ledgerHistoryRow{}
			for rows.Next() {
				var id, date, desc sql.NullString
				var amount sql.NullFloat64
				if err := rows.Scan(&id, &date, &desc, &amount); err != nil {
					continue
				}
				history = append(history, ledgerHistoryRow{
					Date: date.String, MutationID: id.String, Description: desc.String, Amount: amount.Float64,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			sort.SliceStable(history, func(i, j int) bool { return history[i].Date < history[j].Date })
			var running float64
			for i := range history {
				running += history[i].Amount
				history[i].RunningBalance = running
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, history)
			}
			if len(history) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No synced mutations reference ledger %s. Run `sync --full` first.\n", ledgerCode)
				return nil
			}
			out := make([][]string, 0, len(history))
			for _, h := range history {
				out = append(out, []string{h.Date, h.MutationID, h.Description, fmt.Sprintf("%.2f", h.Amount), fmt.Sprintf("%.2f", h.RunningBalance)})
			}
			return flags.printTable(cmd, []string{"DATE", "MUTATION ID", "DESCRIPTION", "AMOUNT", "RUNNING BALANCE"}, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
