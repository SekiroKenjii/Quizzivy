//go:build integration

package repositories_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/core/adapters"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardquery "quizzivy/internal/modules/dashboard/application/query"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"
	"time"
)

func checkedProfileWorld(t *testing.T) *homeWorld {
	t.Helper()
	w := newHomeWorld(t)
	t.Cleanup(func() {
		if err := w.tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("profile world rollback: %v", err)
		}
	})
	return w
}

func TestStoredProfileZoneDrivesActualDashboardDaysAndPropagatesInvalidRows(t *testing.T) {
	w := checkedProfileWorld(t)
	ctx := context.Background()
	dbx := db.NewContext(w.tx)
	identity := identityapp.New(identityrepo.NewUsers(dbx), nil, time.Hour, nil, nil)
	app := dashboardapp.New(dashboardrepo.NewPostgres(dbx)).WithZones(adapters.ProfileZone{Query: identity.Queries.EffectiveZone})
	now := time.Date(2026, 11, 1, 3, 30, 0, 0, time.UTC)
	app.SetClock(func() time.Time { return now })
	for _, tc := range []struct {
		zone *string
		day  string
	}{{nil, "2026-11-01"}, {profileZonePointer("UTC"), "2026-11-01"}, {profileZonePointer("Asia/Tokyo"), "2026-11-01"}, {profileZonePointer("America/New_York"), "2026-10-31"}} {
		execHome(t, w, `UPDATE app.users SET time_zone=$2::text WHERE id=$1::uuid`, w.a, tc.zone)
		out, err := app.Queries.Summary.Handle(ctx, dashboardquery.Summary{Scope: access.Scope{UserID: w.a}, Range: "7d"})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Submissions.Days) != 7 || out.Submissions.Days[6].Date != tc.day {
			t.Fatalf("zone%v days=%+v", tc.zone, out.Submissions.Days)
		}
	}
	execHome(t, w, `UPDATE app.users SET time_zone='Local' WHERE id=$1::uuid`, w.a)
	if _, err := app.Queries.Summary.Handle(ctx, dashboardquery.Summary{Scope: access.Scope{UserID: w.a}}); !errors.Is(err, identitydomain.ErrTimeZoneInvalid) {
		t.Fatal(err)
	}
}

func TestChosenTeacherNameReachesClassesPreviewAndAssignmentIntroOnly(t *testing.T) {
	w := checkedProfileWorld(t)
	ctx := context.Background()
	dbx := db.NewContext(w.tx)
	classes := classesrepo.NewPostgres(dbx)
	assignments := assignmentsrepo.NewPostgres(dbx)
	keys, err := classesdomain.NewJoinCodeKeys([]byte(strings.Repeat("k", 32)), nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("ABCD2345"))
	if _, err := w.tx.Exec(ctx, `INSERT INTO app.class_join_codes(class_id,code_hash,code_hint,expires_at,created_by) VALUES ($1,$2,'2345',now()+interval '1 day',$3)`, w.classA, hash[:], w.a); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		public *string
		want   string
	}{{nil, "Trang chủ"}, {profileZonePointer("Cô An"), "Cô An"}, {nil, "Trang chủ"}} {
		execHome(t, w, `UPDATE app.users SET display_name=$2::text,phone='+84 123456',locale='en',time_zone='UTC',preferences='{"theme":"dark"}' WHERE id=$1::uuid`, w.a, tc.public)
		rows, err := classes.ListMine(ctx, w.s2)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.ID == w.classA {
				found = true
				if row.TeacherName == nil || *row.TeacherName != tc.want {
					t.Fatalf("class teacher=%v", row.TeacherName)
				}
			}
		}
		if !found {
			t.Fatal("own class absent")
		}
		preview, err := classes.LookupByCode(ctx, keys.LookupHashes("ABCD2345"))
		if err != nil || preview == nil || preview.TeacherName == nil || *preview.TeacherName != tc.want {
			t.Fatalf("preview=%+v %v", preview, err)
		}
		intro, err := assignments.StudentDetail(ctx, w.aA, w.s2)
		if err != nil || intro.TeacherName == nil || *intro.TeacherName != tc.want {
			t.Fatalf("intro teacher=%v %v", intro.TeacherName, err)
		}
	}
}
func profileZonePointer(s string) *string { return &s }
