//go:build integration

package application_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/router"
	"quizzivy/internal/modules/access/application"
	accessrepo "quizzivy/internal/modules/access/repositories"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identitycommand "quizzivy/internal/modules/identity/application/command"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

type revocation struct {
	pool     *pgxpool.Pool
	access   *application.Application
	identity *identityapp.Application
	tokens   *identitytoken.Issuer
	handler  http.Handler
	actor    string
	users    []string
	roles    []string
}

func newRevocation(t *testing.T) *revocation {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	dbx := db.NewContext(pool)
	r := &revocation{pool: pool, access: application.New(accessrepo.NewPostgres(dbx))}
	t.Cleanup(func() {
		for _, id := range r.users {
			if _, err := pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup user: %v", err)
			}
		}
		for _, id := range r.roles {
			if _, err := pool.Exec(context.Background(), `DELETE FROM app.roles WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup role: %v", err)
			}
		}
		pool.Close()
	})
	r.tokens, err = identitytoken.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r.identity = identityapp.New(identityrepo.NewUsers(dbx), r.tokens, time.Hour, identityrepo.NewStudents(dbx), attemptsrepo.NewStudentStats(dbx))
	r.identity.SetPrincipals(r.access)
	r.handler, err = router.New(router.Deps{
		Tokens:     r.tokens,
		Principals: adapters.Principals{Query: r.access.Queries.ResolvePrincipal},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	r.actor = r.user(t, `(SELECT id FROM app.roles WHERE builtin_key = 'teacher')`)
	return r
}

func (r *revocation) user(t *testing.T, role string) string {
	t.Helper()
	var id string
	if err := r.pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Revocation fixture', `+role+`)
		RETURNING id::text`, uuid.NewString()+"@example.com").Scan(&id); err != nil {
		t.Fatalf("user: %v", err)
	}
	r.users = append(r.users, id)
	return id
}

func (r *revocation) get(t *testing.T, userID, path string) int {
	t.Helper()
	token, err := r.tokens.Issue(userID, "student", 0)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.handler.ServeHTTP(rec, req)
	return rec.Code
}

func reached(code int) bool { return code != http.StatusUnauthorized && code != http.StatusForbidden }

func TestDisablingAStudentRefusesTheirLiveTokenOnTheNextRequest(t *testing.T) {
	r := newRevocation(t)
	student := r.user(t, `(SELECT id FROM app.roles WHERE builtin_key = 'student')`)
	if code := r.get(t, student, "/app/assignments"); !reached(code) {
		t.Fatalf("before the disable: %d, want the gate passed", code)
	}
	disabled := true
	if _, err := r.identity.Commands.UpdateStudent.Handle(context.Background(), identitycommand.UpdateStudent{
		Request: identitydomain.WriteRequest{ActorID: r.actor},
		Input:   identitydomain.StudentPatch{ID: student, Disabled: &disabled},
	}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if code := r.get(t, student, "/app/assignments"); code != http.StatusUnauthorized {
		t.Errorf("after the disable: %d, want 401", code)
	}
}

func TestResettingAPasswordRefusesTheLiveTokenOnTheNextRequest(t *testing.T) {
	r := newRevocation(t)
	student := r.user(t, `(SELECT id FROM app.roles WHERE builtin_key = 'student')`)
	if code := r.get(t, student, "/app/assignments"); !reached(code) {
		t.Fatalf("before the reset: %d, want the gate passed", code)
	}
	if _, err := r.identity.Commands.ResetStudentPassword.Handle(context.Background(), identitycommand.ResetStudentPassword{
		Request: identitydomain.WriteRequest{ActorID: r.actor},
		ID:      student,
	}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var epoch int
	if err := r.pool.QueryRow(context.Background(), `SELECT session_epoch FROM app.users WHERE id = $1`, student).Scan(&epoch); err != nil || epoch != 1 {
		t.Fatalf("session_epoch = %d (%v), want 1", epoch, err)
	}
	if code := r.get(t, student, "/app/assignments"); code != http.StatusUnauthorized {
		t.Errorf("after the reset: %d, want 401 for a token at epoch 0", code)
	}
}

func TestARemovedGrantIsRefusedAfterForgetAll(t *testing.T) {
	r := newRevocation(t)
	var role string
	if err := r.pool.QueryRow(context.Background(), `
		INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`,
		"Revocation fixture "+uuid.NewString()).Scan(&role); err != nil {
		t.Fatal(err)
	}
	r.roles = append(r.roles, role)
	if _, err := r.pool.Exec(context.Background(), `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'learning.take_tests')`, role); err != nil {
		t.Fatal(err)
	}
	user := r.user(t, "'"+role+"'::uuid")
	if code := r.get(t, user, "/app/assignments"); !reached(code) {
		t.Fatalf("with Take tests: %d, want the gate passed", code)
	}
	if _, err := r.pool.Exec(context.Background(), `DELETE FROM app.role_permissions WHERE role_id = $1`, role); err != nil {
		t.Fatal(err)
	}
	if code := r.get(t, user, "/app/assignments"); !reached(code) {
		t.Fatalf("a cached principal was re-read before ForgetAll: %d", code)
	}
	r.access.ForgetAll()
	if code := r.get(t, user, "/app/assignments"); code != http.StatusForbidden {
		t.Errorf("after ForgetAll: %d, want 403", code)
	}
}
