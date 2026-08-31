// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"

	"github.com/spf13/cobra"
)

type statementLine struct {
	Date           string  `json:"date"`
	Kind           string  `json:"kind"` // "invoice" or "mutation"
	Reference      string  `json:"reference"`
	Description    string  `json:"description,omitempty"`
	Amount         float64 `json:"amount"`
	RunningBalance float64 `json:"runningBalance"`
}

func newNovelRelationStatementCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "statement <relation-id>",
		Short: "Full chronological history of invoices and mutations for one relation, with a computed running balance.",
		Long: "Joins synced invoices and mutations for one relation, ordered by date, with a\n" +
			"cumulative running balance (invoices add to the balance owed, payment\n" +
			"mutations of type 3/4 reduce it). Local computation from synced data only.",
		Example:     "  e-boekhouden-pp-cli relation statement 789012 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			relationID := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("e-boekhouden-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			lines := []statementLine{}

			invRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data, '$.date'), json_extract(data, '$.Date')) AS invoice_date,
					COALESCE(json_extract(data, '$.invoiceNumber'), json_extract(data, '$.InvoiceNumber')) AS number,
					COALESCE(json_extract(data, '$.totalAmount'), json_extract(data, '$.TotalAmount')) AS amount
				FROM resources
				WHERE resource_type = 'invoice'
				  AND (CAST(json_extract(data, '$.relationId') AS TEXT) = ? OR CAST(json_extract(data, '$.RelationId') AS TEXT) = ?)`,
				relationID, relationID)
			if err != nil {
				return fmt.Errorf("query invoices: %w", err)
			}
			for invRows.Next() {
				var date, number sql.NullString
				var amount sql.NullFloat64
				if err := invRows.Scan(&date, &number, &amount); err != nil {
					continue
				}
				lines = append(lines, statementLine{
					Date: date.String, Kind: "invoice", Reference: number.String, Amount: amount.Float64,
				})
			}
			invRows.Close()

			// mutation.data's "amount" is a top-level field on the synced
			// list shape (GET /v1/mutation) — confirmed against a live
			// account. The full "rows" breakdown (per-line ledger/VAT split)
			// is only present on GET /v1/mutation/{id} detail fetches, not
			// the list response sync uses, so it is not read here.
			mutRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT m.date, m.type, m.invoice_number, m.description,
				       COALESCE(json_extract(m.data, '$.amount'), 0)
				FROM mutation m WHERE m.relation_id = ?`, relationID)
			if err != nil {
				return fmt.Errorf("query mutations: %w", err)
			}
			for mutRows.Next() {
				var date, mType, invNum, desc sql.NullString
				var amount sql.NullFloat64
				if err := mutRows.Scan(&date, &mType, &invNum, &desc, &amount); err != nil {
					continue
				}
				signed := amount.Float64
				// Payment received (3) / payment sent (4) reduce the amount owed;
				// everything else (invoice-shaped mutation types 1/2) adds to it.
				if mType.String == "3" || mType.String == "4" {
					signed = -signed
				}
				ref := invNum.String
				if ref == "" {
					ref = "mutation"
				}
				lines = append(lines, statementLine{
					Date: date.String, Kind: "mutation", Reference: ref, Description: desc.String, Amount: signed,
				})
			}
			mutRows.Close()

			sort.SliceStable(lines, func(i, j int) bool { return lines[i].Date < lines[j].Date })
			var running float64
			for i := range lines {
				running += lines[i].Amount
				lines[i].RunningBalance = running
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, lines)
			}
			if len(lines) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No synced invoices or mutations for relation %s. Run `sync --full` first.\n", relationID)
				return nil
			}
			rows := make([][]string, 0, len(lines))
			for _, l := range lines {
				rows = append(rows, []string{l.Date, l.Kind, l.Reference, fmt.Sprintf("%.2f", l.Amount), fmt.Sprintf("%.2f", l.RunningBalance)})
			}
			return flags.printTable(cmd, []string{"DATE", "KIND", "REFERENCE", "AMOUNT", "RUNNING BALANCE"}, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
