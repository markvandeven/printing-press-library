// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

// Write-safety guards shared by mutation and invoice creation. e-Boekhouden
// writes are real accounting entries with no soft-delete or undo, so the
// absorb manifest's write-safety row requires two independent confirmations
// beyond the generic --dry-run default: an explicit --confirm to execute at
// all, and (when the API token is linked to more than one administration) an
// explicit --company match so a write can't silently land in the wrong
// client's books.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/client"

	"github.com/spf13/cobra"
)

// requireWriteConfirmation refuses to proceed with a real (non-dry-run)
// mutation/invoice write unless the caller passed --confirm.
func requireWriteConfirmation(flags *rootFlags, confirmed bool, resource string) error {
	if flags.dryRun {
		return nil
	}
	if confirmed {
		return nil
	}
	return fmt.Errorf("refusing to create a %s without --confirm: this writes a real, hard-to-reverse accounting entry to e-Boekhouden. Re-run with --dry-run to preview the request, or add --confirm to actually send it", resource)
}

// confirmAdministrationTarget guards against writing to the wrong client's
// books. e-Boekhouden has no API-level way to select or verify which
// administration a write targets — the session is bound to whatever
// administration the configured API token belongs to. When that token is
// linked to more than one administration (accountant-managed portfolios),
// this refuses the write unless --company names the exact target, since a
// silent mismatch would be a real, hard-to-reverse mistake.
func confirmAdministrationTarget(cmd *cobra.Command, c *client.Client, flags *rootFlags, company string) error {
	if flags.dryRun {
		return nil
	}
	data, err := c.Get(cmd.Context(), "/v1/administration", nil)
	if err != nil {
		// Best-effort: don't block a write because the metadata lookup itself
		// failed (e.g. transient network issue) — the real POST below will
		// surface any genuine auth problem.
		return nil
	}
	var items []map[string]any
	if json.Unmarshal(data, &items) != nil || len(items) <= 1 {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		if name := firstNonEmpty(it, "Company", "company"); name != "" {
			names = append(names, name)
		}
	}
	if company == "" {
		return fmt.Errorf("this API token is linked to multiple administrations (%s) — pass --company \"<exact name>\" to confirm which one this write targets", strings.Join(names, ", "))
	}
	for _, name := range names {
		if strings.EqualFold(name, company) {
			return nil
		}
	}
	return fmt.Errorf("--company %q does not match any administration linked to this token (known: %s)", company, strings.Join(names, ", "))
}
