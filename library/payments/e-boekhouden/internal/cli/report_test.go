// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/store"
)

func TestDaysSince(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		date string
		want int
	}{
		{"bare date 10 days ago", "2026-02-19", 10},
		{"rfc3339 same day", "2026-03-01T00:00:00Z", 0},
		{"empty string", "", -1},
		{"unparseable", "not-a-date", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := daysSince(tc.date, now); got != tc.want {
				t.Errorf("daysSince(%q) = %d, want %d", tc.date, got, tc.want)
			}
		})
	}
}

func seedBalances(t *testing.T, db *store.Store, rows []map[string]any) {
	t.Helper()
	for _, r := range rows {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal balance fixture: %v", err)
		}
		if err := db.UpsertBalance(raw); err != nil {
			t.Fatalf("upsert balance fixture: %v", err)
		}
	}
}

func TestReportTrialBalanceAndCategorySplit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	mustUpsert(t, db, db.UpsertLedger, map[string]any{"id": "1300", "code": "1300", "description": "Bank", "category": "FIN"})
	mustUpsert(t, db, db.UpsertLedger, map[string]any{"id": "8000", "code": "8000", "description": "Omzet", "category": "VW"})
	seedBalances(t, db, []map[string]any{
		{"Code": "1300", "Type": "FIN", "Balance": 5000.0},
		{"Code": "8000", "Type": "VW", "Balance": -1200.0},
	})
	db.Close()

	// Trial balance sees both lines.
	flags := &rootFlags{asJSON: true}
	trial := newReportTrialBalanceCmd(flags)
	var trialOut bytes.Buffer
	trial.SetOut(&trialOut)
	trial.Flags().Set("db", dbPath)
	trial.SetContext(context.Background())
	if err := trial.RunE(trial, nil); err != nil {
		t.Fatalf("trial-balance RunE: %v", err)
	}
	var trialEnvelope struct {
		Lines []reportLedgerLine `json:"lines"`
	}
	if err := json.Unmarshal(trialOut.Bytes(), &trialEnvelope); err != nil {
		t.Fatalf("unmarshal trial-balance output: %v\noutput: %s", err, trialOut.String())
	}
	if len(trialEnvelope.Lines) != 2 {
		t.Fatalf("expected 2 trial-balance lines, got %d: %+v", len(trialEnvelope.Lines), trialEnvelope.Lines)
	}

	// Balance sheet excludes the VW (profit-and-loss) ledger.
	bs := newReportBalanceSheetCmd(flags)
	var bsOut bytes.Buffer
	bs.SetOut(&bsOut)
	bs.Flags().Set("db", dbPath)
	bs.SetContext(context.Background())
	if err := bs.RunE(bs, nil); err != nil {
		t.Fatalf("balance-sheet RunE: %v", err)
	}
	var bsEnvelope struct {
		Lines []reportLedgerLine `json:"lines"`
	}
	if err := json.Unmarshal(bsOut.Bytes(), &bsEnvelope); err != nil {
		t.Fatalf("unmarshal balance-sheet output: %v", err)
	}
	if len(bsEnvelope.Lines) != 1 || bsEnvelope.Lines[0].Code != "1300" {
		t.Fatalf("expected balance-sheet to contain only the FIN ledger 1300, got %+v", bsEnvelope.Lines)
	}

	// Profit-loss includes only the VW ledger.
	pl := newReportProfitLossCmd(flags)
	var plOut bytes.Buffer
	pl.SetOut(&plOut)
	pl.Flags().Set("db", dbPath)
	pl.SetContext(context.Background())
	if err := pl.RunE(pl, nil); err != nil {
		t.Fatalf("profit-loss RunE: %v", err)
	}
	var plEnvelope struct {
		Lines []reportLedgerLine `json:"lines"`
	}
	if err := json.Unmarshal(plOut.Bytes(), &plEnvelope); err != nil {
		t.Fatalf("unmarshal profit-loss output: %v", err)
	}
	if len(plEnvelope.Lines) != 1 || plEnvelope.Lines[0].Code != "8000" || plEnvelope.Lines[0].Balance != -1200.0 {
		t.Fatalf("expected profit-loss to contain only the VW ledger 8000 with balance -1200, got %+v", plEnvelope.Lines)
	}
}

func TestReportVatSummary_AggregatesByVatCode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "1", "type": "1", "date": "2026-01-01", "description": "Purchase A",
		"ledgerId": 1300, "rows": []any{map[string]any{"ledgerId": 4200, "vatCode": "HOOG_INK_21", "amount": 100.0, "vatAmount": 21.0}},
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "2", "type": "1", "date": "2026-01-02", "description": "Purchase B",
		"ledgerId": 1300, "rows": []any{map[string]any{"ledgerId": 4200, "vatCode": "HOOG_INK_21", "amount": 50.0, "vatAmount": 10.5}},
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "3", "type": "2", "date": "2026-01-03", "description": "Sale A",
		"ledgerId": 1300, "rows": []any{map[string]any{"ledgerId": 8000, "vatCode": "GEEN", "amount": 200.0, "vatAmount": 0.0}},
	})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newReportVatSummaryCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var envelope struct {
		Lines []vatSummaryLine `json:"lines"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out.String())
	}
	byCode := map[string]vatSummaryLine{}
	for _, l := range envelope.Lines {
		byCode[l.VatCode] = l
	}
	hoog, ok := byCode["HOOG_INK_21"]
	if !ok {
		t.Fatalf("expected a HOOG_INK_21 summary line, got %+v", envelope.Lines)
	}
	if hoog.Count != 2 || hoog.Amount != 150 || hoog.VatTotal != 31.5 {
		t.Errorf("expected HOOG_INK_21 count=2 amount=150 vatTotal=31.5, got %+v", hoog)
	}
	geen, ok := byCode["GEEN"]
	if !ok || geen.Count != 1 || geen.Amount != 200 {
		t.Errorf("expected GEEN count=1 amount=200, got %+v (found=%v)", geen, ok)
	}
}
