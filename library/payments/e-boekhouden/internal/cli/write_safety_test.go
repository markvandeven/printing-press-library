// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRequireWriteConfirmation(t *testing.T) {
	cases := []struct {
		name      string
		dryRun    bool
		confirmed bool
		wantErr   bool
	}{
		{"dry run without confirm is allowed", true, false, false},
		{"dry run with confirm is allowed", true, true, false},
		{"real write without confirm is refused", false, false, true},
		{"real write with confirm is allowed", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{dryRun: tc.dryRun}
			err := requireWriteConfirmation(flags, tc.confirmed, "mutation")
			if tc.wantErr && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
