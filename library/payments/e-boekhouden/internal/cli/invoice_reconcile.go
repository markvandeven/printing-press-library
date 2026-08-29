// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"

	"github.com/spf13/cobra"
)

type unmatchedInvoice struct {
	Number       string  `json:"number"`
	RelationID   int64   `json:"relationId,omitempty"`
	RelationName string  `json:"relationName,omitempty"`
	Amount       float64 `json:"amount"`
	Date         string  `json:"date"`
}

type unknownInvoiceMutation struct {
	MutationID    string `json:"mutationId"`
	InvoiceNumber string `json:"invoiceNumber"`
	Date          string `json:"date"`
	Description   string `json:"description,omitempty"`
}

type invoiceReconcileReport struct {
	UnmatchedInvoices       []unmatchedInvoice       `json:"unmatchedInvoices"`
	UnknownInvoiceMutations []unknownInvoiceMutation `json:"unknownInvoiceMutations"`
	Note                    string                   `json:"note"`
}

func newNovelInvoiceReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Lists invoices with no matching payment mutation, and mutations referencing an unknown invoice number.",
		Long: "Cross-references synced invoices against synced mutations of type 'Invoice\n" +
			"payment received' / 'Invoice payment sent' by invoice number. This is a local\n" +
			"integrity check, distinct from the API's own outstanding-invoices endpoint —\n" +
			"it flags invoices with no recorded payment at all, and payment mutations that\n" +
			"reference an invoice number this CLI has never synced.",
		Example:     "  e-boekhouden-pp-cli invoice reconcile --json --select unmatchedInvoices.number,unmatchedInvoices.relationName",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("e-boekhouden-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			report := invoiceReconcileReport{
				UnmatchedInvoices:       []unmatchedInvoice{},
				UnknownInvoiceMutations: []unknownInvoiceMutation{},
				Note:                    "Local computation from synced data only. Run `sync --full` first to refresh. Payment matching is by exact invoice number, not amount — a payment recorded under a different reference will show as unmatched here even if it was actually paid.",
			}

			invRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data, '$.invoiceNumber'), json_extract(data, '$.InvoiceNumber')) AS number,
					COALESCE(json_extract(data, '$.relationId'), json_extract(data, '$.RelationId')) AS relation_id,
					COALESCE(json_extract(data, '$.totalAmount'), json_extract(data, '$.TotalAmount')) AS amount,
					COALESCE(json_extract(data, '$.date'), json_extract(data, '$.Date')) AS invoice_date
				FROM resources WHERE resource_type = 'invoice'`)
			if err != nil {
				return fmt.Errorf("query invoices: %w", err)
			}
			defer invRows.Close()

			for invRows.Next() {
				var number sql.NullString
				var relationID sql.NullInt64
				var amount sql.NullFloat64
				var invDate sql.NullString
				if err := invRows.Scan(&number, &relationID, &amount, &invDate); err != nil {
					continue
				}
				if !number.Valid || number.String == "" {
					continue
				}
				var paidCount int
				_ = db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM mutation WHERE invoice_number = ? AND type IN ('3', '4')`,
					number.String).Scan(&paidCount)
				if paidCount > 0 {
					continue
				}
				u := unmatchedInvoice{Number: number.String, Amount: amount.Float64, Date: invDate.String}
				if relationID.Valid {
					u.RelationID = relationID.Int64
					var name sql.NullString
					_ = db.DB().QueryRowContext(cmd.Context(),
						`SELECT name FROM relation WHERE id = ?`, fmt.Sprint(relationID.Int64)).Scan(&name)
					u.RelationName = name.String
				}
				report.UnmatchedInvoices = append(report.UnmatchedInvoices, u)
			}
			if err := invRows.Err(); err != nil {
				return fmt.Errorf("reading invoices: %w", err)
			}

			mutRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, invoice_number, date, description FROM mutation
				WHERE type IN ('3', '4') AND invoice_number IS NOT NULL AND invoice_number != ''`)
			if err != nil {
				return fmt.Errorf("query mutations: %w", err)
			}
			defer mutRows.Close()

			for mutRows.Next() {
				var id, invNum string
				var date, desc sql.NullString
				if err := mutRows.Scan(&id, &invNum, &date, &desc); err != nil {
					continue
				}
				var invoiceExists int
				_ = db.DB().QueryRowContext(cmd.Context(), `
					SELECT COUNT(*) FROM resources
					WHERE resource_type = 'invoice'
					  AND (json_extract(data, '$.invoiceNumber') = ? OR json_extract(data, '$.InvoiceNumber') = ?)`,
					invNum, invNum).Scan(&invoiceExists)
				if invoiceExists > 0 {
					continue
				}
				report.UnknownInvoiceMutations = append(report.UnknownInvoiceMutations, unknownInvoiceMutation{
					MutationID: id, InvoiceNumber: invNum, Date: date.String, Description: desc.String,
				})
			}
			if err := mutRows.Err(); err != nil {
				return fmt.Errorf("reading mutations: %w", err)
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Unmatched invoices (no recorded payment): %d\n", len(report.UnmatchedInvoices))
			for _, u := range report.UnmatchedInvoices {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-10.2f  %s  %s\n", u.Number, u.Amount, u.Date, u.RelationName)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nPayment mutations referencing an unknown invoice number: %d\n", len(report.UnknownInvoiceMutations))
			for _, m := range report.UnknownInvoiceMutations {
				fmt.Fprintf(cmd.OutOrStdout(), "  mutation %s -> invoice %s (%s)\n", m.MutationID, m.InvoiceNumber, m.Date)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
