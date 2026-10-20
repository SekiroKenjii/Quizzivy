//go:build integration

package maintenance_test

import (
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/maintenance"
	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/db"
)

func TestTheSettingsImpactCountsWhatTheSweepsDryRunCountsUnderTheAppRole(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" && os.Getenv("CI") != "" {
		t.Fatal("APP_DATABASE_URL is not set in CI; the impact count must be read as the app role")
	}
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	iso := newIsolated(t)
	w := &sweepWorld{t: t, ctx: iso.ctx, q: iso.owner, conn: iso.role, margin: time.Minute, role: maintenanceRole}
	w.seed()
	w.exec(`GRANT USAGE ON SCHEMA public TO quizzivy_app`)
	app, err := pgxpool.New(iso.ctx, databaseURL(t, dsn, iso.name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	var role string
	if err := app.QueryRow(iso.ctx, `SELECT current_user::text`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("the impact is read as %q (%v), want quizzivy_app", role, err)
	}

	cutoff := w.cutoff(12)
	old := w.closedAt(cutoff.Add(-24 * time.Hour))
	attempt := w.attempt(old, w.user("Học viên", "student"), "submitted", cutoff.Add(-24*time.Hour))
	w.answers(attempt)
	w.event(attempt, w.cutoff(6).Add(-time.Hour))
	w.event(attempt, w.cutoff(6).Add(time.Hour))
	swept := w.attempt(old, w.user("Học viên đã xoá bài", "student"), "graded", cutoff.Add(-24*time.Hour))
	w.answers(swept)
	w.exec(`UPDATE app.attempt_answers SET payload = $2::text::jsonb WHERE attempt_id = $1`, swept, removedPayload)
	extended := w.user("Học viên được gia hạn", "student")
	w.override(old, extended, cutoff.AddDate(0, 6, 0))
	w.answers(w.attempt(old, extended, "submitted", cutoff.Add(-24*time.Hour)))
	recent := w.closedAt(cutoff.Add(24 * time.Hour))
	w.answers(w.attempt(recent, w.user("Học viên gần đây", "student"), "submitted", cutoff.Add(24*time.Hour)))
	w.disable(w.user("Học viên đã khoá", "student"), cutoff.Add(-time.Hour))
	sitting := w.user("Học viên đang làm bài", "student")
	w.disable(sitting, cutoff.Add(-time.Hour))
	w.attempt(old, sitting, "in_progress", cutoff.Add(-24*time.Hour))
	owner := w.user("Giáo viên có lớp", "teacher")
	w.disable(owner, cutoff.Add(-time.Hour))
	w.id(`INSERT INTO app.classes(name, teacher_id) VALUES ('Lớp còn dạy', $1) RETURNING id::text`, owner)
	w.disable(w.user("Học viên mới khoá", "student"), cutoff.Add(time.Hour))

	impact := repositories.NewPostgres(db.NewContext(app))
	for _, policy := range []domain.Retention{
		{KeepAnswersMonths: months(12), KeepFocusEventsMonths: 6, AnonymiseDisabledAfterMonths: months(12)},
		{KeepAnswersMonths: months(24), KeepFocusEventsMonths: 24, AnonymiseDisabledAfterMonths: months(24)},
		{KeepFocusEventsMonths: 12},
	} {
		w.retention(policy.KeepAnswersMonths, policy.KeepFocusEventsMonths, policy.AnonymiseDisabledAfterMonths)
		dry := w.sweep(maintenance.SweepOptions{})
		got, err := impact.RetentionImpact(iso.ctx, policy)
		if err != nil {
			t.Fatal(err)
		}
		if (got.Answers != nil) != dry.Answers.Enabled || (got.Accounts != nil) != dry.Accounts.Enabled {
			t.Errorf("policy %+v: the impact is %+v, the dry run enabled answers %v and accounts %v",
				policy, got, dry.Answers.Enabled, dry.Accounts.Enabled)
			continue
		}
		if got.FocusEvents != dry.FocusEvents.Events {
			t.Errorf("policy %+v: the impact counts %d events, the dry run %d", policy, got.FocusEvents, dry.FocusEvents.Events)
		}
		if a := got.Answers; a != nil && (a.Attempts != dry.Answers.Attempts || a.Answers != dry.Answers.Answers) {
			t.Errorf("policy %+v: the impact counts answers %+v, the dry run %+v", policy, *a, dry.Answers)
		}
		if a := got.Accounts; a != nil && (a.Anonymise != dry.Accounts.Anonymise || a.SkippedOwners != dry.Accounts.SkippedOwners) {
			t.Errorf("policy %+v: the impact counts accounts %+v, the dry run %+v", policy, *a, dry.Accounts)
		}
		if policy.KeepFocusEventsMonths == 6 && (got.Answers.Attempts != 1 || got.Answers.Answers != 2 ||
			got.FocusEvents != 1 || got.Accounts.Anonymise != 1 || got.Accounts.SkippedOwners != 1) {
			t.Errorf("the twelve-month policy counted %+v %+v %+v, want 1 attempt, 2 answers, 1 event, 1 account and 1 owner",
				*got.Answers, got.FocusEvents, *got.Accounts)
		}
	}
}
