package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Persist a saved query and return only NEW notices since the cursor's last advance",
		Long: `Saved-query primitive backed by a SQLite cursor.

  watch add <name> <filter-json>     create or replace a saved query
  watch list                         list saved queries with their cursors
  watch rm <name>                    remove a saved query
  watch run <name>                   fetch only notices published since the cursor; advance the cursor

filter-json keys mirror the TenderNed API:
  search, cpvCodes, publicatieType, typeOpdracht, procedure,
  nationaalOfEuropees, sluitingsDatumVanaf, sluitingsDatumTot,
  aanbestedendeDienstId

Example:
  watch add civils '{"cpvCodes":"45000000-7","nationaalOfEuropees":"NA"}'
  watch run civils`,
	}
	cmd.AddCommand(newWatchAddCmd(flags), newWatchListCmd(flags), newWatchRmCmd(flags), newWatchRunCmd(flags))
	return cmd
}

func ensureWatchTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tn_watches (
		name TEXT PRIMARY KEY,
		filter_json TEXT NOT NULL,
		cursor TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	return err
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "add [name] [filter-json]",
		Short:   "Create or replace a saved query",
		Example: `  tenderned-pp-cli watch add civils '{"cpvCodes":"45000000-7","nationaalOfEuropees":"NA"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureWatchTable(cmd.Context(), s.DB()); err != nil {
				return err
			}
			var sanity map[string]any
			if err := json.Unmarshal([]byte(args[1]), &sanity); err != nil {
				return fmt.Errorf("filter-json: %w", err)
			}
			now := time.Now().UTC().Format(time.RFC3339)
			_, err = s.DB().ExecContext(cmd.Context(), `INSERT INTO tn_watches(name, filter_json, cursor, created_at, updated_at)
				VALUES(?, ?, '', ?, ?) ON CONFLICT(name) DO UPDATE SET filter_json=excluded.filter_json, updated_at=excluded.updated_at`,
				args[0], args[1], now, now)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved watch %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved queries with their cursors",
		Example:     `  tenderned-pp-cli watch list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureWatchTable(cmd.Context(), s.DB()); err != nil {
				return err
			}
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT name, filter_json, cursor, updated_at FROM tn_watches ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var name, filt, cur, upd sql.NullString
				if err := rows.Scan(&name, &filt, &cur, &upd); err != nil {
					continue
				}
				out = append(out, map[string]any{
					"name":    name.String,
					"filter":  json.RawMessage(filt.String),
					"cursor":  cur.String,
					"updated": upd.String,
				})
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			for _, w := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s  cursor=%s\n", w["name"], w["cursor"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "rm [name]",
		Short: "Remove a saved query",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureWatchTable(cmd.Context(), s.DB()); err != nil {
				return err
			}
			res, err := s.DB().ExecContext(cmd.Context(), `DELETE FROM tn_watches WHERE name=?`, args[0])
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d watch(es)\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:     "run [name]",
		Short:   "Run a saved query against the live API; return only notices newer than the cursor; advance the cursor",
		Example: `  tenderned-pp-cli watch run civils --limit 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureWatchTable(cmd.Context(), s.DB()); err != nil {
				return err
			}
			var filt, cur sql.NullString
			err = s.DB().QueryRowContext(cmd.Context(), `SELECT filter_json, cursor FROM tn_watches WHERE name=?`, args[0]).Scan(&filt, &cur)
			if err == sql.ErrNoRows {
				return fmt.Errorf("no watch named %q (use 'watch add' first)", args[0])
			}
			if err != nil {
				return err
			}
			var filter map[string]any
			if err := json.Unmarshal([]byte(filt.String), &filter); err != nil {
				return err
			}
			params := url.Values{}
			for k, v := range filter {
				params.Set(k, fmt.Sprint(v))
			}
			// Force publicatieDatumVanaf to the cursor when present.
			if cur.String != "" {
				params.Set("publicatieDatumVanaf", cur.String[:10])
			}
			if limit > 0 {
				params.Set("size", fmt.Sprintf("%d", limit))
			} else {
				params.Set("size", "50")
			}
			params.Set("page", "0")
			fullURL := tnBaseURL + "/publicaties?" + params.Encode()
			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, fullURL, nil)
			if err != nil {
				return fmt.Errorf("building request: %w", err)
			}
			req.Header.Set("Accept", "application/json")
			resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode/100 != 2 {
				return fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			var body struct {
				Content       []json.RawMessage `json:"content"`
				TotalElements int               `json:"totalElements"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				return err
			}
			fresh := []json.RawMessage{}
			latest := cur.String
			for _, raw := range body.Content {
				var v struct {
					Date string `json:"publicatieDatum"`
				}
				_ = json.Unmarshal(raw, &v)
				if cur.String == "" || v.Date > cur.String {
					fresh = append(fresh, raw)
					if v.Date > latest {
						latest = v.Date
					}
				}
			}
			if latest != cur.String {
				// Surface cursor-update failure to stderr — silently
				// swallowing it means the next watch run would replay
				// notices we already returned this time, breaking the
				// "only NEW notices" contract.
				if _, uerr := s.DB().ExecContext(cmd.Context(), `UPDATE tn_watches SET cursor=?, updated_at=? WHERE name=?`,
					latest, time.Now().UTC().Format(time.RFC3339), args[0]); uerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: cursor update for watch %q failed: %v (next run may re-report these notices)\n", args[0], uerr)
				}
			}
			result := map[string]any{
				"watch":  args[0],
				"new":    fresh,
				"count":  len(fresh),
				"cursor": latest,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d new notice(s) for watch %q (cursor → %s)\n", len(fresh), args[0], latest)
			for _, raw := range fresh {
				var v struct {
					ID    string `json:"publicatieId"`
					Title string `json:"aanbestedingNaam"`
					Buyer string `json:"opdrachtgeverNaam"`
					Date  string `json:"publicatieDatum"`
				}
				_ = json.Unmarshal(raw, &v)
				date := v.Date
				if idx := strings.Index(date, "T"); idx > 0 {
					date = date[:idx]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %s | %s | %s\n", v.ID, date, v.Buyer, v.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max notices to fetch this run")
	return cmd
}
