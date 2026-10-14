//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/db"
)

func restoreSettings(t *testing.T, pool *pgxpool.Pool, want rawRow) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Errorf("restoring the settings row: begin: %v", err)
		return
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("restoring the settings row: rollback: %v", err)
		}
	}()
	for _, sql := range []string{
		`SET LOCAL lock_timeout = '30s'`,
		`ALTER TABLE app.organization_settings DISABLE TRIGGER organization_settings_set_updated_at`,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Errorf("restoring the settings row: %s: %v", sql, err)
			return
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.organization_settings
		   SET name = $1, short_name = $2, website = $3, contact_email = $4, front_desk_phone = $5,
		       front_desk_hours = $6, address = $7,
		       logo_sha256 = $8, logo_type = $9, logo_dark_sha256 = $10, logo_dark_type = $11,
		       google_sign_in = $12, password_sign_in = $13, force_password_change = $14, join_codes_enabled = $15,
		       allowed_google_domains = $16,
		       keep_answers_months = $17, keep_focus_events_months = $18, anonymise_disabled_after_months = $19,
		       revision = $20, updated_by = (SELECT id FROM app.users WHERE id = $21::uuid), updated_at = $22`,
		want.name, want.shortName, want.website, want.contactEmail, want.phone, want.hours, want.address,
		want.logo, want.logoType, want.logoDark, want.logoDarkType,
		want.google, want.password, want.force, want.joinCodes, want.domains,
		want.keepAnswers, want.keepFocus, want.anonymise,
		want.revision, want.updatedBy, want.updatedAt); err != nil {
		t.Errorf("restoring the settings row: update: %v", err)
		return
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE app.organization_settings ENABLE TRIGGER organization_settings_set_updated_at`); err != nil {
		t.Errorf("restoring the settings row: enable the trigger: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		t.Errorf("restoring the settings row: commit: %v", err)
		return
	}
	if got := readRaw(t, pool); !got.same(want) {
		t.Errorf("settings row after the restore = %+v, want the original %+v", got, want)
	}
}

func removeAuditRows(t *testing.T, pool *pgxpool.Pool, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE entity = 'organization' AND occurred_at = $1`, at); err != nil {
		t.Errorf("removing the audit rows: %v", err)
	}
}

func removeAccount(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	for attempt := 0; ; attempt++ {
		_, err := pool.Exec(context.Background(), `DELETE FROM app.users WHERE lower(email) = lower($1)`, email)
		if err == nil {
			return
		}
		if !db.IsCheckViolation(err, "users_last_admin") {
			t.Errorf("removing the account %s: %v", email, err)
			return
		}
		if attempt == 20 {
			t.Logf("kept the account %s: it is the only active Admin, which the users_last_admin guard never lets go", email)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func insertAdmin(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, password_hash)
		VALUES ($1, 'Quản trị thử đua', (SELECT id FROM app.roles WHERE builtin_key = 'admin'), '$argon2id$not-a-real-hash')
		RETURNING id::text`, email).Scan(&id); err != nil {
		t.Fatalf("admin: %v", err)
	}
	return id
}

func holdSettingsRow(t *testing.T, pool *pgxpool.Pool) func() {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("releasing the settings row: %v", err)
		}
	}
	t.Cleanup(release)
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '30s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM app.organization_settings FOR UPDATE`); err != nil {
		t.Fatalf("holding the settings row: %v", err)
	}
	return release
}

func waitForQueuedWriters(t *testing.T, pool *pgxpool.Pool, writers int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var queued int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_locks WHERE NOT granted AND locktype IN ('tuple', 'transactionid')`).Scan(&queued); err != nil {
			t.Fatal(err)
		}
		if queued >= writers {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Log("the contenders were not all seen waiting on the row; racing them anyway")
}

type contender struct {
	name string
	err  error
}

func race(t *testing.T, migrate *pgxpool.Pool, repo *repositories.Postgres, base domain.Settings, actor string, at time.Time, writers int, label string) []contender {
	t.Helper()
	release := holdSettingsRow(t, migrate)
	results := make([]contender, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc := base
			doc.SignIn.Password = true
			doc.Profile.Name = fmt.Sprintf("%s %d", label, i)
			_, err := repo.Save(context.Background(), domain.Save{
				Settings: doc, ExpectedRevision: base.Revision, ActorID: actor,
				IP: "198.51.100.9", UserAgent: "organization-race", Now: at, GoogleConfigured: true,
			})
			results[i] = contender{name: doc.Profile.Name, err: err}
		}()
	}
	waitForQueuedWriters(t, migrate, writers)
	release()
	wg.Wait()
	return results
}

func TestConcurrentSavesOfOneRevisionStoreExactlyOneAndRefuseTheRest(t *testing.T) {
	migrate := connect(t, migrateEnv)
	app := connectMax(t, appEnv, 16)
	original := readRaw(t, migrate)
	t.Cleanup(func() { restoreSettings(t, migrate, original) })
	at := uniqueInstant(t)
	t.Cleanup(func() { removeAuditRows(t, migrate, at) })
	email := "org-race-" + nonce(t) + "@example.com"
	t.Cleanup(func() { removeAccount(t, migrate, email) })
	actor := insertAdmin(t, migrate, email)
	repo := repositories.NewPostgres(db.NewContext(app))
	ctx := context.Background()

	audited := 0
	for round, writers := range []int{2, 2, 2, 2, 3, 8} {
		var results []contender
		var base domain.Settings
		for attempt := 0; attempt < 3; attempt++ {
			var err error
			if base, err = repo.Read(ctx); err != nil {
				t.Fatalf("round %d: read: %v", round, err)
			}
			results = race(t, migrate, repo, base, actor, at, writers, fmt.Sprintf("Đua %d lần %d", round, attempt))
			if !allStale(results) {
				break
			}
		}

		var winner string
		wins := 0
		for _, r := range results {
			switch {
			case r.err == nil:
				wins++
				winner = r.name
			case errors.Is(r.err, domain.ErrStale):
			default:
				t.Fatalf("round %d: %s failed with %v, want it stored or stale", round, r.name, r.err)
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: %d of %d saves of revision %d were stored, want exactly one: %+v", round, wins, writers, base.Revision, results)
		}
		audited++

		stored, err := repo.Read(ctx)
		if err != nil {
			t.Fatalf("round %d: read after: %v", round, err)
		}
		if stored.Profile.Name != winner || stored.Revision != base.Revision+1 || stored.UpdatedBy != actor {
			t.Errorf("round %d: stored %q at revision %d by %q, want the winner %q at revision %d by %q",
				round, stored.Profile.Name, stored.Revision, stored.UpdatedBy, winner, base.Revision+1, actor)
		}
		var rows int
		if err := migrate.QueryRow(ctx, `
			SELECT count(*) FROM app.audit_log WHERE entity = 'organization' AND action = 'organization.updated' AND occurred_at = $1`, at).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != audited {
			t.Errorf("round %d: audit rows = %d, want one per stored save (%d)", round, rows, audited)
		}
	}
}

func allStale(results []contender) bool {
	for _, r := range results {
		if !errors.Is(r.err, domain.ErrStale) {
			return false
		}
	}
	return true
}
