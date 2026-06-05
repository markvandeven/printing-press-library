// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelConclusieCmd(flags *rootFlags) *cobra.Command {
	var flagFull bool

	cmd := &cobra.Command{
		Use:   "conclusie <ecli>",
		Short: "Pair a Hoge Raad decision with its A-G conclusie (bidirectional)",
		Long: `Given a Hoge Raad uitspraak ECLI, walk the dcterms:relation edges to find
the matching A-G (Advocate-General) conclusie ECLI. Given a conclusie ECLI,
walks the reverse direction to find the resulting uitspraak.

Pass --full to also fetch the paired decision's content (metadata +
summary + body).`,
		Example: `  rechtspraak-pp-cli conclusie ECLI:NL:HR:2024:1
  rechtspraak-pp-cli conclusie ECLI:NL:PHR:2023:1057 --full --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ecli := args[0]
			parsed, err := rechtspraak.ParseECLI(ecli)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			http := mustHTTP()
			d, err := http.Get(ctx, ecli, false)
			if err != nil {
				return err
			}
			pair := pickConclusiePair(d, parsed.Court)
			if pair == "" {
				return fmt.Errorf("no conclusie/uitspraak pair found in relations for %s", ecli)
			}
			result := map[string]any{
				"source":      ecli,
				"source_type": d.Type,
				"paired":      pair,
				"direction":   pairDirection(parsed.Court, d.Type),
			}
			if flagFull {
				paired, err := http.Get(ctx, pair, false)
				if err == nil {
					result["paired_decision"] = paired
				} else {
					result["paired_error"] = err.Error()
				}
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Source:    %s (%s)\n", ecli, d.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "Paired:    %s\n", pair)
			fmt.Fprintf(cmd.OutOrStdout(), "Direction: %s\n", pairDirection(parsed.Court, d.Type))
			if flagFull {
				if paired, ok := result["paired_decision"].(*rechtspraak.Decision); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s, %s)\n", paired.ECLI, paired.Court, paired.DecisionDate)
					if paired.Summary != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", paired.Summary)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagFull, "full", false, "Also fetch the paired decision's content")
	return cmd
}

// pickConclusiePair returns the related ECLI that matches the conclusie/uitspraak
// pairing pattern. The logic:
//   - HR uitspraak → find Conclusie relation (links to PHR ECLI)
//   - PHR conclusie → find Cassatie relation (links to HR ECLI)
func pickConclusiePair(d *rechtspraak.Decision, sourceCourt string) string {
	if d == nil {
		return ""
	}
	wantConclusie := sourceCourt == "HR" || strings.EqualFold(d.Type, "Uitspraak")
	for _, rel := range d.Relations {
		t := strings.ToLower(rel.TypeRelatie + rel.Text)
		if wantConclusie {
			if strings.Contains(t, "conclusie") {
				return rel.Target
			}
		} else {
			if strings.Contains(t, "cassatie") {
				return rel.Target
			}
		}
	}
	// Fall back to the first relation if no specific match.
	if len(d.Relations) > 0 {
		return d.Relations[0].Target
	}
	return ""
}

func pairDirection(sourceCourt, sourceType string) string {
	if sourceCourt == "HR" || strings.EqualFold(sourceType, "Uitspraak") {
		return "uitspraak → conclusie"
	}
	return "conclusie → uitspraak"
}
