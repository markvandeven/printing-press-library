// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-written session-handshake auth for e-Boekhouden. The generator's
// built-in session_handshake auth type only supports a GET-based
// bootstrap+crumb fetch; e-Boekhouden requires POSTing the long-lived API
// token as a JSON body to /v1/session and using the returned short-lived
// session token verbatim (no "Bearer " prefix) on every subsequent request.
// See docs/SPEC-EXTENSIONS.md session_handshake auth type for the pattern
// this deviates from and why.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/e-boekhouden/internal/cliutil"
)

// sessionSource identifies this integration to e-Boekhouden support. Must be
// 1-10 chars matching ^[\w_ ]{1,10}$ per the /v1/session request contract —
// \w does not include a hyphen, so "pp-cli" is rejected with API_SESSION_002
// (confirmed against the live API); "pp_cli" is accepted.
const sessionSource = "pp_cli"

// sessionExpiryMargin is subtracted from the server-reported expiresIn so a
// request in flight when the token is about to expire doesn't race the
// server's own clock.
const sessionExpiryMargin = 30 * time.Second

// SessionManager exchanges the long-lived EBOEKHOUDEN_API_TOKEN for a
// short-lived session token and caches it across invocations. Safe for
// concurrent use.
type SessionManager struct {
	mu         sync.Mutex
	httpClient *http.Client
	baseURL    string
	token      string
	expiresAt  time.Time
}

func newSessionManager(httpClient *http.Client, baseURL string) *SessionManager {
	m := &SessionManager{httpClient: httpClient, baseURL: baseURL}
	m.loadFromDisk()
	return m
}

type sessionStartRequest struct {
	AccessToken string `json:"accessToken"`
	Source      string `json:"source"`
}

type sessionStartResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"`
}

// EnsureToken returns a valid session token, exchanging the API token for a
// fresh one if the cached token is missing or expired. Concurrent callers
// wait for a single in-flight exchange.
func (m *SessionManager) EnsureToken(ctx context.Context, apiToken string) (string, error) {
	m.mu.Lock()
	if m.token != "" && time.Now().Before(m.expiresAt) {
		t := m.token
		m.mu.Unlock()
		return t, nil
	}
	m.mu.Unlock()

	if cliutil.IsVerifyEnv() {
		// Mirrors the generator's client_credentials mock-token pattern: never
		// dial out during a verify pass, even though StartSession itself
		// requires no auth and so isn't gated by the transport layer's
		// mutating-verb short-circuit.
		return "mock-session-token-for-testing", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Re-check under the write lock in case another goroutine won the race
	// while we waited.
	if m.token != "" && time.Now().Before(m.expiresAt) {
		return m.token, nil
	}

	reqBody, err := json.Marshal(sessionStartRequest{AccessToken: apiToken, Source: sessionSource})
	if err != nil {
		return "", fmt.Errorf("building session request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/v1/session", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("building session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("starting e-boekhouden session: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading session response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("starting e-boekhouden session: HTTP %d: %s (check EBOEKHOUDEN_API_TOKEN)", resp.StatusCode, string(body))
	}

	var sess sessionStartResponse
	if err := json.Unmarshal(body, &sess); err != nil {
		return "", fmt.Errorf("parsing session response: %w", err)
	}
	if sess.Token == "" {
		return "", fmt.Errorf("e-boekhouden session response had an empty token")
	}

	m.token = sess.Token
	ttl := time.Duration(sess.ExpiresIn) * time.Second
	if ttl > sessionExpiryMargin {
		ttl -= sessionExpiryMargin
	}
	m.expiresAt = time.Now().Add(ttl)
	m.saveToDisk()
	return m.token, nil
}

// Invalidate clears the cached session token so the next EnsureToken call
// re-exchanges it. Call this after a 401/403 in case the session expired
// early or was revoked server-side.
func (m *SessionManager) Invalidate() {
	m.mu.Lock()
	m.token = ""
	m.expiresAt = time.Time{}
	m.mu.Unlock()
}

type sessionRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (m *SessionManager) sessionFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "e-boekhouden-pp-cli", "session.json")
}

func (m *SessionManager) loadFromDisk() {
	path := m.sessionFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var rec sessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return
	}
	if rec.Token == "" || time.Now().After(rec.ExpiresAt) {
		return
	}
	m.token = rec.Token
	m.expiresAt = rec.ExpiresAt
}

func (m *SessionManager) saveToDisk() {
	path := m.sessionFilePath()
	if path == "" {
		return
	}
	rec := sessionRecord{Token: m.token, ExpiresAt: m.expiresAt}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}
