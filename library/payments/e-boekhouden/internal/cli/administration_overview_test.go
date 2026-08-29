// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	m := map[string]any{"Code": "1001", "type": "DEB"}
	if got := firstNonEmpty(m, "Code", "code"); got != "1001" {
		t.Errorf("expected Code=1001, got %q", got)
	}
	if got := firstNonEmpty(m, "Type", "type"); got != "DEB" {
		t.Errorf("expected fallback to lowercase 'type'=DEB, got %q", got)
	}
	if got := firstNonEmpty(m, "Missing", "AlsoMissing"); got != "" {
		t.Errorf("expected empty string for missing keys, got %q", got)
	}
}

func TestBalanceTableRows(t *testing.T) {
	items := []map[string]any{
		{"Code": "1001", "Type": "DEB", "Balance": 500.0},
		{"code": "1300", "type": "FIN", "balance": 1200.5},
	}
	rows := balanceTableRows(items)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "1001" || rows[0][1] != "DEB" || rows[0][2] != "500" {
		t.Errorf("unexpected PascalCase row: %v", rows[0])
	}
	if rows[1][0] != "1300" || rows[1][1] != "FIN" || rows[1][2] != "1200.5" {
		t.Errorf("unexpected camelCase row: %v", rows[1])
	}
}
