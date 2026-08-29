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

func TestSignificantWords(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"basic", "Office supplies - Staples", []string{"office", "supplies", "staples"}},
		{"short words dropped", "a to it Staples", []string{"staples"}},
		{"duplicates deduped", "Rent rent RENT", []string{"rent"}},
		{"empty", "", nil},
		{"numbers kept", "Invoice 12345 payment", []string{"invoice", "12345", "payment"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := significantWords(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("significantWords(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("significantWords(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestMutationSuggest_RanksByFrequencyOfPastLedger(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Two past "office supplies" mutations booked to ledger 4200, one booked
	// to a different ledger. The suggestion should rank 4200 first. Matches
	// on the mutation's own top-level ledgerId (the list-sync shape) — not
	// a per-line rows[].ledgerId breakdown, which only exists on a detail
	// fetch (GET /v1/mutation/{id}), confirmed against a live account.
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "1", "type": "1", "date": "2026-01-01", "description": "Office supplies - Staples",
		"ledgerId": 4200, "amount": 25.5,
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "2", "type": "1", "date": "2026-02-01", "description": "Office supplies - Bruna",
		"ledgerId": 4200, "amount": 40.0,
	})
	mustUpsert(t, db, db.UpsertMutation, map[string]any{
		"id": "3", "type": "1", "date": "2026-03-01", "description": "Office chair",
		"ledgerId": 4500, "amount": 300.0,
	})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelMutationSuggestCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"Office supplies - Amazon"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var suggestions []mutationSuggestion
	if err := json.Unmarshal(out.Bytes(), &suggestions); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, out.String())
	}
	if len(suggestions) == 0 {
		t.Fatalf("expected at least one suggestion, got none")
	}
	top := suggestions[0]
	if top.LedgerID != 4200 {
		t.Fatalf("expected top suggestion ledger 4200, got ledger %d", top.LedgerID)
	}
	if top.Occurrences != 2 {
		t.Errorf("expected 2 occurrences for the top suggestion, got %d", top.Occurrences)
	}
}

func TestMutationSuggest_NoMatchesReturnsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newNovelMutationSuggestCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"Completely unrelated description"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var suggestions []mutationSuggestion
	if err := json.Unmarshal(out.Bytes(), &suggestions); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(suggestions) != 0 {
		t.Fatalf("expected no suggestions against an empty store, got %+v", suggestions)
	}
}
