// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type administrationOverview struct {
	// Administrations and LinkedAdministrations are metadata only — the
	// e-Boekhouden API scopes every session to exactly one administration
	// (whichever one the configured API token belongs to). There is no
	// endpoint or header to select or query a *different* administration's
	// data in the same session, so balance/outstanding figures below always
	// describe the currently-authenticated administration, not every row
	// in these two lists.
	//
	// Administrations (GET /v1/administration) is accountant-only — the API
	// returns EP_001 "This endpoint is only available to accountants." for
	// every non-accountant token (confirmed against the live API). It is
	// omitted (nil) rather than failing the whole command when unavailable.
	Administrations       json.RawMessage `json:"administrations,omitempty"`
	LinkedAdministrations json.RawMessage `json:"linkedAdministrations"`
	CurrentSession        struct {
		LedgerBalances         json.RawMessage `json:"ledgerBalances"`
		OutstandingReceivables json.RawMessage `json:"outstandingReceivables"`
		OutstandingPayables    json.RawMessage `json:"outstandingPayables"`
	} `json:"currentSession"`
	Note string `json:"note"`
}

func newNovelAdministrationOverviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Shows administrations linked to this account, plus balance and outstanding-invoice counts for the currently authenticated administration.",
		Long: "e-Boekhouden scopes each API token to exactly one administration — there is no\n" +
			"way to query a different administration's balances in the same session. This\n" +
			"command lists every administration you (or your accountant token) can see and\n" +
			"the linked administrations, alongside the ledger balances and outstanding\n" +
			"invoices for the ONE administration the current session is actually\n" +
			"authenticated against. Run it once per API token to build a full portfolio view.",
		Example:     "  e-boekhouden-pp-cli administration overview --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			overview := administrationOverview{
				Note: "administrations/linkedAdministrations are account-level metadata; currentSession reflects only the administration this API token is bound to — the API has no way to query other administrations' data from one session. \"administrations\" is omitted for non-accountant tokens (the API restricts GET /v1/administration to accountants).",
			}

			// GET /v1/administration is accountant-only (EP_001 for everyone
			// else) — best-effort, never fails the whole command.
			var admins json.RawMessage
			if a, aerr := c.Get(cmd.Context(), "/v1/administration", nil); aerr == nil {
				admins = a
				overview.Administrations = admins
			}

			linked, err := c.Get(cmd.Context(), "/v1/administration/linked", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			overview.LinkedAdministrations = linked

			balances, err := c.Get(cmd.Context(), "/v1/ledger/balances", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			overview.CurrentSession.LedgerBalances = balances

			// credDeb is a required query param: D = debtors (accounts
			// receivable — money owed TO this administration), C = creditors
			// (accounts payable — money this administration owes). Confirmed
			// required against the live API (MUTA_006 without it).
			receivables, err := c.Get(cmd.Context(), "/v1/mutation/invoice/outstanding", map[string]string{"credDeb": "D"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			overview.CurrentSession.OutstandingReceivables = receivables

			payables, err := c.Get(cmd.Context(), "/v1/mutation/invoice/outstanding", map[string]string{"credDeb": "C"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			overview.CurrentSession.OutstandingPayables = payables

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, overview)
			}

			linkedItems := extractItems(linked)
			receivableItems := extractItems(receivables)
			payableItems := extractItems(payables)

			if admins != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Administrations you can see: %d\n", len(extractItems(admins)))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Administrations you can see: n/a (this token is not an accountant token)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Linked administrations: %d\n", len(linkedItems))
			fmt.Fprintf(cmd.OutOrStdout(), "\nCurrent session (this API token's administration):\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Outstanding receivables (money owed to you): %d\n", len(receivableItems))
			fmt.Fprintf(cmd.OutOrStdout(), "  Outstanding payables (money you owe): %d\n", len(payableItems))
			fmt.Fprintln(cmd.OutOrStdout(), "  Ledger balances:")
			return flags.printTable(cmd, []string{"CODE", "TYPE", "BALANCE"}, balanceTableRows(extractItems(balances)))
		},
	}
	return cmd
}

func balanceTableRows(items []map[string]any) [][]string {
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		code := firstNonEmpty(it, "Code", "code")
		typ := firstNonEmpty(it, "Type", "type")
		bal := firstNonEmpty(it, "Balance", "balance")
		rows = append(rows, []string{code, typ, bal})
	}
	return rows
}

func firstNonEmpty(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}
