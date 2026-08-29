// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"
)

func TestLedgerHistory_MatchesOnlyTopLevelLedger(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	mustUpsert(t, db, db.UpsertLedger, map[string]any{
		"id": "4200", "code": "4200", "description": "Kantoorkosten", "category": "VW",
	})
	// Two mutations whose own top-level ledger is 4200 (the list-sync
	// shape carries a top-level ledgerId + amount; the per-line rows
	// breakdown only exists on a GET /v1/mutation/{id} detail fetch,
	// confirmed against a live account, so it is not matched here).
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "1", "type": "1", "date": "2026-01-01", "description": "Office supplies",
		"ledgerId": 4200, "amount": 25.0,
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "2", "type": "7", "date": "2026-01-10", "description": "Journal correction",
		"ledgerId": 4200, "amount": 10.0,
	})
	// A mutation touching an unrelated ledger must not appear.
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "3", "type": "1", "date": "2026-01-15", "description": "Unrelated",
		"ledgerId": 5000, "amount": 500.0,
	})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelLedgerHistoryCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"4200"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var history []ledgerHistoryRow
	if err := json.Unmarshal(out.Bytes(), &history); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out.String())
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history rows for ledger 4200, got %d: %+v", len(history), history)
	}
	if history[0].MutationID != "1" || history[0].Amount != 25 || history[0].RunningBalance != 25 {
		t.Errorf("expected first row from mutation 1 (amount 25, running 25), got %+v", history[0])
	}
	if history[1].MutationID != "2" || history[1].Amount != 10 || history[1].RunningBalance != 35 {
		t.Errorf("expected second row from mutation 2 (amount 10, running 35), got %+v", history[1])
	}
}

func TestLedgerHistory_UnknownLedgerCodeErrors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelLedgerHistoryCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	err = cmd.RunE(cmd, []string{"9999"})
	if err == nil {
		t.Fatalf("expected an error for an unknown ledger code, got nil")
	}
}
