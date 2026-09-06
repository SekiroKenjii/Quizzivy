//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/config"
)

// world is one whole application, assembled the way cmd/api assembles it,
// served in-process over HTTP against the database TEST_DATABASE_URL names.
type world struct {
	t      *testing.T
	server *httptest.Server
	pool   *pgxpool.Pool
}

func boot(t *testing.T) *world {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; the end-to-end suite needs a database")
	}
	ctx := context.Background()
	cfg := config.Config{
		Port:                        "0",
		Env:                         "test",
		DatabaseURL:                 dsn,
		AllowedOrigins:              []string{"http://localhost:5173"},
		MaxConcurrentPasswordHashes: 4,
		JWTSigningKey:               []byte(strings.Repeat("e2e-signing-key-", 2)),
		AccessTokenTTL:              15 * time.Minute,
		RefreshTokenTTL:             30 * 24 * time.Hour,
	}
	app, err := core.New(ctx, cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("assemble the application: %v", err)
	}
	t.Cleanup(app.Close)
	handler, err := app.Handler()
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return &world{t: t, server: server, pool: pool}
}

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// teacher provisions the practice's admin account the way the seed does, and
// returns the credentials a browser would type.
func (w *world) teacher() (email, password string) {
	w.t.Helper()
	password = "giao-vien-" + nonce(w.t)
	hash, err := identitydomain.Passwords.Hash(context.Background(), password)
	if err != nil {
		w.t.Fatal(err)
	}
	email = "teacher-" + nonce(w.t) + "@example.com"
	if _, err := w.pool.Exec(context.Background(),
		`INSERT INTO app.users (email, full_name, role, password_hash) VALUES ($1, 'Cô Thương', 'admin', $2)`,
		email, hash); err != nil {
		w.t.Fatal(err)
	}
	return email, password
}

// client is one browser: its own cookie jar, so refresh cookies stay with it.
type client struct {
	w     *world
	http  *http.Client
	token string
}

func (w *world) browser() *client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		w.t.Fatal(err)
	}
	return &client{w: w, http: &http.Client{Jar: jar}}
}

func (c *client) login(email, password string) map[string]any {
	c.w.t.Helper()
	status, body := c.call(http.MethodPost, "/auth/login", map[string]any{"email": email, "password": password})
	if status != http.StatusOK {
		c.w.t.Fatalf("login %s: status %d: %v", email, status, body)
	}
	c.token = body["accessToken"].(string)
	return body
}

// call sends JSON and decodes JSON; a body of nil sends nothing.
func (c *client) call(method, path string, payload any) (int, map[string]any) {
	c.w.t.Helper()
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			c.w.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.w.server.URL+path, reader)
	if err != nil {
		c.w.t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.w.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	body := map[string]any{}
	if len(raw) > 0 {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			c.w.t.Fatalf("%s %s answered %d with a body that is not JSON: %s", method, path, resp.StatusCode, raw)
		}
		if m, ok := decoded.(map[string]any); ok {
			body = m
		} else {
			body["items"] = decoded
		}
	}
	return resp.StatusCode, body
}

func (c *client) must(status int, method, path string, payload any) map[string]any {
	c.w.t.Helper()
	got, body := c.call(method, path, payload)
	if got != status {
		c.w.t.Fatalf("%s %s: status %d, want %d: %v", method, path, got, status, body)
	}
	return body
}

func id(body map[string]any) string {
	return body["id"].(string)
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }
