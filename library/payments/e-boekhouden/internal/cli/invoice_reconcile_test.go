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

func mustUpsert(t *testing.T, db *store.Store, upsert func(json.RawMessage) error, obj map[string]any) {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := upsert(raw); err != nil {
		t.Fatalf("upsert fixture: %v", err)
	}
}

func TestInvoiceReconcile_FindsUnmatchedInvoiceAndUnknownMutation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Invoice 1 has a matching payment mutation (type 3) — should NOT appear
	// as unmatched. Invoice 2 has no payment mutation — SHOULD appear.
	mustUpsert(t, db, db.UpsertInvoice, map[string]any{
		"id": "1", "invoiceNumber": "INV-001", "relationId": 42, "totalAmount": 100.0, "date": "2026-01-01",
	})
	mustUpsert(t, db, db.UpsertInvoice, map[string]any{
		"id": "2", "invoiceNumber": "INV-002", "relationId": 42, "totalAmount": 200.0, "date": "2026-01-02",
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "10", "type": "3", "date": "2026-01-05", "invoiceNumber": "INV-001", "description": "payment for INV-001",
		"ledgerId": 1300, "rows": []any{map[string]any{"ledgerId": 1300, "vatCode": "GEEN", "amount": 100.0}},
	})
	// Payment mutation referencing an invoice number that was never synced.
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "11", "type": "4", "date": "2026-01-06", "invoiceNumber": "INV-999", "description": "payment for unknown invoice",
		"ledgerId": 1300, "rows": []any{map[string]any{"ledgerId": 1300, "vatCode": "GEEN", "amount": 50.0}},
	})
	mustUpsert(t, db, db.UpsertRelation, map[string]any{"id": "42", "name": "Acme BV"})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelInvoiceReconcileCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var report invoiceReconcileReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out.String())
	}

	if len(report.UnmatchedInvoices) != 1 {
		t.Fatalf("expected 1 unmatched invoice, got %d: %+v", len(report.UnmatchedInvoices), report.UnmatchedInvoices)
	}
	if report.UnmatchedInvoices[0].Number != "INV-002" {
		t.Errorf("expected unmatched invoice INV-002, got %q", report.UnmatchedInvoices[0].Number)
	}
	if report.UnmatchedInvoices[0].RelationName != "Acme BV" {
		t.Errorf("expected relation name resolved to Acme BV, got %q", report.UnmatchedInvoices[0].RelationName)
	}

	if len(report.UnknownInvoiceMutations) != 1 {
		t.Fatalf("expected 1 unknown-invoice mutation, got %d: %+v", len(report.UnknownInvoiceMutations), report.UnknownInvoiceMutations)
	}
	if report.UnknownInvoiceMutations[0].InvoiceNumber != "INV-999" {
		t.Errorf("expected unknown invoice number INV-999, got %q", report.UnknownInvoiceMutations[0].InvoiceNumber)
	}
}

func TestInvoiceReconcile_EmptyStoreReturnsEmptyLists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelInvoiceReconcileCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var report invoiceReconcileReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(report.UnmatchedInvoices) != 0 || len(report.UnknownInvoiceMutations) != 0 {
		t.Fatalf("expected empty lists on an empty store, got %+v", report)
	}
}
