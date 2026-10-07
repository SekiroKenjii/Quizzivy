//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"strings"
	"testing"
	"time"
)

func prefWrite(w profileWorld, patch string) domain.PreferencesRecord {
	return domain.PreferencesRecord{UserID: w.id, Patch: json.RawMessage(patch), Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

func TestPreferencesSQLMergesTopLevelFalseReplacesNestedAndReturnsStored(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	if _, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{"theme":"dark","compactTables":true,"assignmentDefaults":{"durationMinutes":45,"showScore":true}}`)); err != nil {
		t.Fatal(err)
	}
	got, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{"compactTables":false,"largerTestText":false,"assignmentDefaults":{"showScore":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme == nil || *got.Theme != "dark" || got.CompactTables == nil || *got.CompactTables || got.LargerTestText == nil || *got.LargerTestText || got.AssignmentDefaults == nil || got.AssignmentDefaults.DurationMinutes != nil || got.AssignmentDefaults.ShowScore == nil || *got.AssignmentDefaults.ShowScore {
		t.Fatalf("merged=%+v defaults=%+v", got, got.AssignmentDefaults)
	}
	var stored string
	if err := w.tx.QueryRow(ctx, `SELECT preferences::text FROM app.users WHERE id=$1::uuid`, w.id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stored), &value); err != nil {
		t.Fatal(err)
	}
	if string(value["assignmentDefaults"]) != `{"showScore": false}` {
		t.Fatal(stored)
	}
	before := auditCount(t, w)
	if _, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{"theme":"dark"}`)); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, w) != before {
		t.Fatal("no-op preferences emitted an audit")
	}
	audits := auditFields(t, w)
	if string(audits["user.preferences_updated"]) != `{"fields": ["assignmentDefaults", "compactTables", "largerTestText"]}` {
		t.Fatalf("audit=%s", audits["user.preferences_updated"])
	}
}

func TestPreferencesStoredCapUsesNormalizedUTF8BytesAndRollsBackOverflow(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(fmt.Sprint(multi), func(t *testing.T) {
			w := profileFixture(t)
			ctx := context.Background()
			filler := strings.Repeat("a", 8183)
			if multi {
				filler = strings.Repeat("ữ", 2727) + "aa"
			}
			exact := `{"x":"` + filler + `"}`
			if _, err := w.tx.Exec(ctx, `UPDATE app.users SET preferences=$2::jsonb WHERE id=$1::uuid`, w.id, exact); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := w.tx.QueryRow(ctx, `SELECT octet_length(preferences::text) FROM app.users WHERE id=$1::uuid`, w.id).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 8192 {
				t.Fatalf("normalized bytes=%d", n)
			}
			if _, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{"compactTables":false}`)); !errors.Is(err, domain.ErrPreferencesTooLarge) {
				t.Fatalf("merged overflow=%v", err)
			}
			if auditCount(t, w) != 0 {
				t.Fatal("overflow audited")
			}
			var saved int
			if err := w.tx.QueryRow(ctx, `SELECT octet_length(preferences::text) FROM app.users WHERE id=$1::uuid`, w.id).Scan(&saved); err != nil || saved != 8192 {
				t.Fatalf("saved=%d %v", saved, err)
			}
			nested, err := w.tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = nested.Exec(ctx, `INSERT INTO app.users(email,full_name,role_id,preferences) VALUES ('r407-over-'||uuidv7()::text||'@example.com','Over',(SELECT id FROM app.roles WHERE builtin_key='student'),$1::jsonb)`, `{"x":"`+filler+`a"}`)
			if !db.IsCheckViolation(err, "users_preferences_bytes_check") {
				t.Fatalf("direct8193=%v", err)
			}
			if err := nested.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
	w := profileFixture(t)
	ctx := context.Background()
	for _, invalid := range []string{`[]`, `null`, `"text"`} {
		tx, err := w.tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `UPDATE app.users SET preferences=$2::jsonb WHERE id=$1::uuid`, w.id, invalid)
		if !db.IsCheckViolation(err, "users_preferences_object_check") {
			t.Fatalf("shape%s=%v", invalid, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPreferencesAuditFailureIsAtomicAndDisabledWritesAreRefused(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	in := prefWrite(w, `{"theme":"dark"}`)
	in.IP = ptr("bad-ip")
	if _, err := w.users.UpdatePreferences(ctx, in); err == nil {
		t.Fatal("audit failure accepted")
	}
	stored, err := w.users.FindUserByID(ctx, w.id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Preferences.Theme != nil || auditCount(t, w) != 0 {
		t.Fatal("audit failure committed preferences")
	}
	if _, err := w.tx.Exec(ctx, `UPDATE app.users SET disabled_at=now() WHERE id=$1::uuid`, w.id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.users.UpdatePreferences(ctx, prefWrite(w, `{}`)); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatal(err)
	}
}

func TestPreferencesConcurrentAtomicMergePreservesDisjointAndOrdersSameKey(t *testing.T) {
	for _, tc := range []struct {
		name, first, second, theme string
		compact                    bool
	}{{"disjoint", `{"theme":"dark"}`, `{"compactTables":true}`, "dark", true}, {"same key", `{"theme":"dark"}`, `{"theme":"light"}`, "light", false}} {
		t.Run(tc.name, func(t *testing.T) {
			race := newProfileRace(t)
			race.insert("Owned concurrency")
			w := profileWorld{pool: race.pool}
			ctx, id := race.ctx, race.id
			first := race.begin()
			users := repositories.NewUsers(db.NewContext(first))
			record := domain.PreferencesRecord{UserID: id, Patch: json.RawMessage(tc.first), Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
			if _, err := users.UpdatePreferences(ctx, record); err != nil {
				t.Fatal(err)
			}
			race.startWriter(func(ctx context.Context, users *repositories.Users) error {
				_, err := users.UpdatePreferences(ctx, domain.PreferencesRecord{UserID: id, Patch: json.RawMessage(tc.second), Now: record.Now})
				return err
			})
			if err := first.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := race.result(); err != nil {
				t.Fatal(err)
			}
			stored, err := repositories.NewUsers(db.NewContext(w.pool)).FindUserByID(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Preferences.Theme == nil || *stored.Preferences.Theme != tc.theme || tc.compact && (stored.Preferences.CompactTables == nil || !*stored.Preferences.CompactTables) {
				t.Fatalf("lost/incorrect merge=%+v", stored.Preferences)
			}
			var n int
			if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1::uuid`, id).Scan(&n); err != nil || n != 2 {
				t.Fatalf("audit=%d %v", n, err)
			}
		})
	}
}

func TestPreferencesDisabledRaceRechecksTheLockedCurrentRow(t *testing.T) {
	race := newProfileRace(t)
	race.insert("Owned disable race")
	w := profileWorld{pool: race.pool}
	ctx, id := race.ctx, race.id
	holder := race.begin()
	if _, err := holder.Exec(ctx, `UPDATE app.users SET disabled_at=now() WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	race.startWriter(func(ctx context.Context, users *repositories.Users) error {
		_, err := users.UpdatePreferences(ctx, domain.PreferencesRecord{UserID: id, Patch: json.RawMessage(`{"theme":"dark"}`), Now: time.Now()})
		return err
	})
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := race.result(); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatalf("disabled race=%v", err)
	}
	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1::uuid`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("disabled audit=%d %v", n, err)
	}
}

func waitProfileLock(t *testing.T, ctx context.Context, w profileWorld, pid int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := w.pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("writer never reached actual row-lock barrier")
		case <-ticker.C:
		}
	}
}
