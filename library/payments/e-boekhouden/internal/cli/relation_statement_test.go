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

func TestRelationStatement_ComputesRunningBalanceChronologically(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Invoice for 100 on Jan 1, partial payment mutation of 40 on Jan 5.
	// Running balance should go 100 -> 60.
	mustUpsert(t, db, db.UpsertInvoice, map[string]any{
		"id": "1", "invoiceNumber": "INV-001", "relationId": 42, "totalAmount": 100.0, "date": "2026-01-01",
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "10", "type": "3", "date": "2026-01-05", "relationId": 42, "description": "partial payment",
		"ledgerId": 1300, "amount": 40.0,
	})
	// A mutation for a different relation must not appear in this statement.
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "11", "type": "3", "date": "2026-01-06", "relationId": 99, "description": "unrelated",
		"ledgerId": 1300, "amount": 5.0,
	})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelRelationStatementCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"42"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var lines []statementLine
	if err := json.Unmarshal(out.Bytes(), &lines); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out.String())
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 statement lines for relation 42, got %d: %+v", len(lines), lines)
	}
	if lines[0].Kind != "invoice" || lines[0].Amount != 100 || lines[0].RunningBalance != 100 {
		t.Errorf("expected first line to be the 100 invoice with running balance 100, got %+v", lines[0])
	}
	if lines[1].Kind != "mutation" || lines[1].Amount != -40 || lines[1].RunningBalance != 60 {
		t.Errorf("expected second line to be a -40 payment with running balance 60, got %+v", lines[1])
	}
}

func TestRelationStatement_UnknownRelationReturnsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelRelationStatementCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"999"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var lines []statementLine
	if err := json.Unmarshal(out.Bytes(), &lines); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected no statement lines for an unknown relation, got %+v", lines)
	}
}
