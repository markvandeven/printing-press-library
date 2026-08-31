// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"

	"github.com/spf13/cobra"
)

type mutationSuggestion struct {
	LedgerID    int64   `json:"ledgerId"`
	LedgerCode  string  `json:"ledgerCode,omitempty"`
	Occurrences int     `json:"occurrences"`
	LastAmount  float64 `json:"lastAmount"`
}

func newNovelMutationSuggestCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "suggest <description>",
		Short: "Suggests the ledger most often used for past mutations with a similar description.",
		Long: "Ranks the ledger used most often on your own synced mutation history whose\n" +
			"description shares words with the given description. Purely a local frequency\n" +
			"match over your own data — no external service, no API call.\n" +
			"\n" +
			"Ranks by the mutation's own top-level ledger only, not a per-line VAT/ledger\n" +
			"breakdown — that breakdown (rows) is only present on a GET /v1/mutation/{id}\n" +
			"detail fetch, not the list response `sync` uses.",
		Example:     "  e-boekhouden-pp-cli mutation suggest \"Office supplies - Staples\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			description := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("e-boekhouden-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			words := significantWords(description)
			if len(words) == 0 {
				return fmt.Errorf("description must contain at least one word of 3+ characters")
			}

			var likeClauses []string
			var likeArgs []any
			for _, w := range words {
				likeClauses = append(likeClauses, "m.description LIKE ?")
				likeArgs = append(likeArgs, "%"+w+"%")
			}
			query := fmt.Sprintf(`
				SELECT m.ledger_id, COALESCE(json_extract(m.data, '$.amount'), 0), m.date
				FROM mutation m
				WHERE (%s) AND m.ledger_id IS NOT NULL
				ORDER BY m.date DESC`, strings.Join(likeClauses, " OR "))

			rows, err := db.DB().QueryContext(cmd.Context(), query, likeArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			counts := map[int64]*mutationSuggestion{}
			var order []int64
			for rows.Next() {
				var ledgerID sql.NullInt64
				var amount sql.NullFloat64
				var mutDate sql.NullString
				if err := rows.Scan(&ledgerID, &amount, &mutDate); err != nil {
					continue
				}
				if !ledgerID.Valid {
					continue
				}
				k := ledgerID.Int64
				if _, ok := counts[k]; !ok {
					counts[k] = &mutationSuggestion{LedgerID: k}
					order = append(order, k)
				}
				counts[k].Occurrences++
				if amount.Valid && counts[k].LastAmount == 0 {
					counts[k].LastAmount = amount.Float64
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			// Resolve ledger codes for display; best-effort, missing codes are fine.
			for _, k := range order {
				var code sql.NullString
				_ = db.DB().QueryRowContext(cmd.Context(),
					`SELECT code FROM ledger WHERE id = ?`, fmt.Sprint(k)).Scan(&code)
				if code.Valid {
					counts[k].LedgerCode = code.String
				}
			}

			suggestions := make([]mutationSuggestion, 0, len(order))
			for _, k := range order {
				suggestions = append(suggestions, *counts[k])
			}
			sort.SliceStable(suggestions, func(i, j int) bool {
				return suggestions[i].Occurrences > suggestions[j].Occurrences
			})
			if limit > 0 && len(suggestions) > limit {
				suggestions = suggestions[:limit]
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, suggestions)
			}
			if len(suggestions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No similar past mutations found. Run `sync` first if the local store is empty.")
				return nil
			}
			return flags.printTable(cmd, []string{"LEDGER ID", "LEDGER CODE", "TIMES USED", "LAST AMOUNT"}, suggestionRows(suggestions))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of suggestions to return")
	return cmd
}

func suggestionRows(s []mutationSuggestion) [][]string {
	rows := make([][]string, 0, len(s))
	for _, x := range s {
		rows = append(rows, []string{
			fmt.Sprint(x.LedgerID),
			x.LedgerCode,
			fmt.Sprint(x.Occurrences),
			fmt.Sprintf("%.2f", x.LastAmount),
		})
	}
	return rows
}

// significantWords splits a free-text description into lowercase words of
// 3+ characters, deduplicated, for use in a LIKE-based frequency match.
func significantWords(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	var out []string
	for _, f := range fields {
		if len(f) < 3 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
