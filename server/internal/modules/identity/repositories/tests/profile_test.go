//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"reflect"
	"testing"
	"time"
)

type profileWorld struct {
	pool  *pgxpool.Pool
	tx    pgx.Tx
	users *repositories.Users
	id    string
}

func profileFixture(t *testing.T) profileWorld {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("fixture rollback: %v", err)
		}
	})
	var id string
	err = tx.QueryRow(context.Background(), `INSERT INTO app.users(email,full_name,role_id) VALUES ('r407-'||uuidv7()::text||'@example.com','Original',(SELECT id FROM app.roles WHERE builtin_key='student')) RETURNING id::text`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return profileWorld{pool, tx, repositories.NewUsers(db.NewContext(tx)), id}
}
func ptr[T any](v T) *T { return &v }
func profileWrite(w profileWorld, p domain.ProfilePatch) domain.ProfileRecord {
	return domain.ProfileRecord{UserID: w.id, Patch: p, Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}
func auditFields(t *testing.T, w profileWorld) map[string]json.RawMessage {
	t.Helper()
	rows, err := w.tx.Query(context.Background(), `SELECT action,diff FROM app.audit_log WHERE entity='user' AND entity_id=$1::uuid ORDER BY id`, w.id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var action string
		var diff json.RawMessage
		if err := rows.Scan(&action, &diff); err != nil {
			t.Fatal(err)
		}
		out[action] = diff
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func auditCount(t *testing.T, w profileWorld) int {
	t.Helper()
	var count int
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE entity_id=$1::uuid`, w.id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestProfileSQLPreservesOmissionClearsNullAndReturnsPersistedValues(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	first, err := w.users.UpdateProfile(ctx, profileWrite(w, domain.ProfilePatch{DisplayNameSet: true, DisplayName: ptr("Public"), PhoneSet: true, Phone: ptr("+84 123456"), Locale: ptr("en"), TimeZone: ptr("UTC")}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.users.UpdateProfile(ctx, profileWrite(w, domain.ProfilePatch{PhoneSet: true}))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := w.users.FindUserByID(ctx, w.id)
	if err != nil {
		t.Fatal(err)
	}
	if first.Phone == nil || second.Phone != nil || stored.Phone != nil || *second.DisplayName != "Public" || *second.Locale != "en" || *second.TimeZone != "UTC" || second.FullName != "Original" {
		t.Fatalf("first=%+v second=%+v stored=%+v", first, second, stored)
	}
	if !reflect.DeepEqual(second, stored) {
		t.Fatal("response is not persisted projection")
	}
	if _, err := w.users.UpdateProfile(ctx, profileWrite(w, domain.ProfilePatch{DisplayNameSet: true})); err != nil {
		t.Fatal(err)
	}
	public, err := repositories.NewStudents(db.NewContext(w.tx)).Account(ctx, w.id)
	if err != nil {
		t.Fatal(err)
	}
	if public.DisplayName != nil {
		t.Fatalf("public nullable clear=%+v", public)
	}
}

func TestProfileSQLAuditsOnlyActualSortedPrivateNamesAndCompatibleRename(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	p := domain.ProfilePatch{FullName: ptr("Changed"), PhoneSet: true, Phone: ptr("+84 123456"), DisplayNameSet: true, DisplayName: ptr("Public"), Locale: ptr("en"), TimeZone: ptr("UTC")}
	if _, err := w.users.UpdateProfile(ctx, profileWrite(w, p)); err != nil {
		t.Fatal(err)
	}
	events := auditFields(t, w)
	if string(events["user.profile_updated"]) != `{"fields": ["displayName", "locale", "phone", "timeZone"]}` {
		t.Fatalf("private audit=%s", events["user.profile_updated"])
	}
	var rename map[string]string
	if err := json.Unmarshal(events["user.renamed"], &rename); err != nil {
		t.Fatal(err)
	}
	if rename["from"] != "Original" || rename["to"] != "Changed" || len(events) != 2 {
		t.Fatalf("audit=%v", events)
	}
	before := auditCount(t, w)
	if _, err := w.users.UpdateProfile(ctx, profileWrite(w, p)); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, w) != before {
		t.Fatal("unchanged supplied fields claimed audit change")
	}
}

func TestProfileSQLRollsBackMutationWhenAuditMetadataIsInvalid(t *testing.T) {
	w := profileFixture(t)
	in := profileWrite(w, domain.ProfilePatch{FullName: ptr("Never saved"), PhoneSet: true, Phone: ptr("123456")})
	in.IP = ptr("invalid-ip")
	if _, err := w.users.UpdateProfile(context.Background(), in); err == nil {
		t.Fatal("invalid audit metadata accepted")
	}
	stored, err := w.users.FindUserByID(context.Background(), w.id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.FullName != "Original" || stored.Phone != nil || auditCount(t, w) != 0 {
		t.Fatal("failed audit left a partial mutation")
	}
}

func TestProfileSQLRefusesDisabledAtTheWriteAndKeepsZoneErrorsExplicit(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	if zone, err := w.users.EffectiveZone(ctx, w.id); err != nil || zone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("default=%q %v", zone, err)
	}
	for _, zone := range []string{"UTC", "Asia/Tokyo", "America/New_York"} {
		if _, err := w.users.UpdateProfile(ctx, profileWrite(w, domain.ProfilePatch{TimeZone: ptr(zone)})); err != nil {
			t.Fatal(err)
		}
		if got, err := w.users.EffectiveZone(ctx, w.id); err != nil || got != zone {
			t.Fatalf("zone=%q %v", got, err)
		}
	}
	if _, err := w.tx.Exec(ctx, `UPDATE app.users SET time_zone='Local' WHERE id=$1::uuid`, w.id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.users.EffectiveZone(ctx, w.id); !errors.Is(err, domain.ErrTimeZoneInvalid) {
		t.Fatal(err)
	}
	if _, err := w.tx.Exec(ctx, `UPDATE app.users SET disabled_at=now() WHERE id=$1::uuid`, w.id); err != nil {
		t.Fatal(err)
	}
	before := auditCount(t, w)
	if _, err := w.users.UpdateProfile(ctx, profileWrite(w, domain.ProfilePatch{FullName: ptr("No")})); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatal(err)
	}
	if _, err := w.users.EffectiveZone(ctx, w.id); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatal(err)
	}
	if auditCount(t, w) != before {
		t.Fatal("disabled write audited")
	}
	if _, err := w.users.EffectiveZone(ctx, "01935000-0000-7000-8000-0000000000ff"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal(err)
	}
}

func TestProfileMigrationKeepsTheR3InsertListAndAppAppendOnlyPrivileges(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	var prefs string
	var fields int
	if err := w.tx.QueryRow(ctx, `SELECT preferences::text, num_nonnulls(display_name,phone,avatar_key,locale,time_zone) FROM app.users WHERE id=$1::uuid`, w.id).Scan(&prefs, &fields); err != nil {
		t.Fatal(err)
	}
	if prefs != "{}" || fields != 0 {
		t.Fatalf("legacy defaults=%s fields%d", prefs, fields)
	}
	var allowed bool
	if err := w.tx.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app','app.audit_log','UPDATE') OR has_table_privilege('quizzivy_app','app.audit_log','DELETE') OR has_table_privilege('quizzivy_app','app.attempt_events','UPDATE') OR has_table_privilege('quizzivy_app','app.attempt_events','DELETE')`).Scan(&allowed); err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("app append-only privileges changed")
	}
}

func TestProfileAndPreferencesPersistWithTheActualAppRoleAndCheckedRollback(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("app rollback: %v", err)
		}
	})
	var database, role string
	var version int
	if err := tx.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int`).Scan(&database, &role, &version); err != nil {
		t.Fatal(err)
	}
	if role != "quizzivy_app" || version < 180000 {
		t.Fatalf("app proof database=%s role=%s version=%d", database, role, version)
	}
	t.Logf("actual app connection database=%s role=%s version=%d", database, role, version)
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO app.users(email,full_name,role_id) VALUES ('r407-app-'||uuidv7()::text||'@example.com','App original',(SELECT id FROM app.roles WHERE builtin_key='student')) RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	users := repositories.NewUsers(db.NewContext(tx))
	out, err := users.UpdateProfile(ctx, domain.ProfileRecord{UserID: id, Patch: domain.ProfilePatch{DisplayNameSet: true, DisplayName: ptr("App public"), TimeZone: ptr("UTC")}, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := users.UpdatePreferences(ctx, domain.PreferencesRecord{UserID: id, Patch: json.RawMessage(`{"compactTables":false}`), Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1::uuid`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if out.DisplayName == nil || *out.DisplayName != "App public" || prefs.CompactTables == nil || *prefs.CompactTables || count != 2 {
		t.Fatalf("app projection=%+v prefs=%+v count=%d", out, prefs, count)
	}
}

func TestProfileConcurrentDisjointPatchesReadTheLockedCurrentValues(t *testing.T) {
	race := newProfileRace(t)
	race.insert("Owned profile concurrency")
	w := profileWorld{pool: race.pool}
	ctx, id := race.ctx, race.id
	holder := race.begin()
	record := domain.ProfileRecord{UserID: id, Patch: domain.ProfilePatch{DisplayNameSet: true, DisplayName: ptr("Concurrent public")}, Now: time.Now()}
	if _, err := repositories.NewUsers(db.NewContext(holder)).UpdateProfile(ctx, record); err != nil {
		t.Fatal(err)
	}
	race.startWriter(func(ctx context.Context, users *repositories.Users) error {
		_, err := users.UpdateProfile(ctx, domain.ProfileRecord{UserID: id, Patch: domain.ProfilePatch{PhoneSet: true, Phone: ptr("+84 123456")}, Now: record.Now})
		return err
	})
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := race.result(); err != nil {
		t.Fatal(err)
	}
	stored, err := repositories.NewUsers(db.NewContext(w.pool)).FindUserByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1::uuid`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if stored.DisplayName == nil || *stored.DisplayName != "Concurrent public" || stored.Phone == nil || *stored.Phone != "+84 123456" || stored.FullName != "Owned profile concurrency" || count != 2 {
		t.Fatalf("lost profile fields=%+v audit=%d", stored, count)
	}
}
