// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

// e-Boekhouden's balance DTOs (GET /v1/ledger/balances and
// GET /v1/ledger/{id}/balance) have no id/Id/ID/uuid/slug/name field —
// only Code/Type/Balance, keyed by ledger code — and no ledgerId field
// either, despite the "ledger_id" column being NOT NULL. UpsertBalance
// falls back to "code"/"Code" for its own row identity, and resolves
// ledger_id by looking up the already-synced ledger with a matching code,
// falling back to the resolved id (the code) when no match exists yet —
// confirmed against a live account that `sync` runs "ledger" and
// "ledger-balances" concurrently with no ordering guarantee, so the lookup
// can legitimately miss even on a full sync. These tests lock that behavior
// in since a regression here silently breaks `sync` for the ledger-balances
// resource with no error surfaced to the user (see UpsertBalance/
// upsertBalanceTx's comments for the underlying generator gap).

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestUpsertBalance_ResolvesIdAndLedgerIDFromSyncedLedger(t *testing.T) {
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ledgerRaw, _ := json.Marshal(map[string]any{"id": "42", "code": "1001", "description": "Debtors", "category": "DEB"})
	if err := db.UpsertLedger(ledgerRaw); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	raw, err := json.Marshal(map[string]any{"Code": "1001", "Type": "DEB", "Balance": 500.0})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := db.UpsertBalance(raw); err != nil {
		t.Fatalf("UpsertBalance with Code-only (no id, no ledgerId) object failed: %v", err)
	}

	var total int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM balance`).Scan(&total); err != nil {
		t.Fatalf("query balance count: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected exactly 1 balance row to be stored, got %d", total)
	}

	var ledgerID string
	if err := db.DB().QueryRow(`SELECT ledger_id FROM balance LIMIT 1`).Scan(&ledgerID); err != nil {
		t.Fatalf("query ledger_id: %v", err)
	}
	if ledgerID != "42" {
		t.Errorf("expected ledger_id to resolve to the synced ledger's id 42, got %q", ledgerID)
	}
}

func TestUpsertBalance_MissingIdentityFieldsFails(t *testing.T) {
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	raw, _ := json.Marshal(map[string]any{"Type": "DEB", "Balance": 500.0})
	if err := db.UpsertBalance(raw); err == nil {
		t.Fatalf("expected an error when neither id nor code is present, got nil")
	}
}

func TestUpsertBalance_CodeWithNoSyncedLedgerFallsBackToOwnID(t *testing.T) {
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// A balance can be identified by Code even if no ledger with that code
	// has been synced yet (e.g. a concurrent sync race, or ledger-balances
	// synced on its own). ledger_id falls back to the resolved id (the code)
	// rather than failing the whole upsert — see upsertBalanceTx's comment.
	raw, _ := json.Marshal(map[string]any{"Code": "9999", "Type": "DEB", "Balance": 500.0})
	if err := db.UpsertBalance(raw); err != nil {
		t.Fatalf("UpsertBalance with an unsynced ledger code failed: %v", err)
	}

	var ledgerID string
	if err := db.DB().QueryRow(`SELECT ledger_id FROM balance WHERE id = '9999'`).Scan(&ledgerID); err != nil {
		t.Fatalf("query ledger_id: %v", err)
	}
	if ledgerID != "9999" {
		t.Errorf("expected ledger_id to fall back to the resolved id 9999, got %q", ledgerID)
	}
}
