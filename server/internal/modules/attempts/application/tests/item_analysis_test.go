//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

func mark(v float64) *float64 { return &v }

func saved(t *testing.T, pool *pgxpool.Pool, attempt, question string, payload map[string]any, auto, manual *float64, requiresManual bool) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, auto_score, manual_score, graded_at)
		VALUES ($1::uuid, $2::uuid, $3::jsonb, $4, $5, $6, CASE WHEN $6::numeric IS NULL THEN NULL ELSE now() END)`,
		attempt, question, raw, requiresManual, auto, manual); err != nil {
		t.Fatal(err)
	}
}

func picked(option string) map[string]any {
	return map[string]any{"type": "choice", "optionIds": []string{option}}
}

func essay(text string) map[string]any { return map[string]any{"type": "text", "value": text} }

func emptyBlank() map[string]any {
	return map[string]any{"type": "fill_blank", "values": map[string]any{}}
}

func analysisOf(t *testing.T, pool *pgxpool.Pool, scope access.Scope, assignment string) (domain.ItemAnalysis, error) {
	t.Helper()
	return newService(t, pool).Queries.ItemAnalysis.Handle(context.Background(), query.ItemAnalysis{AssignmentID: assignment, Scope: scope})
}

func TestTheItemAnalysisRanksTheQuestionsByHowOftenStudentsGotThemRight(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	people := enrol(t, pool, w, 6)
	a, b, c, d, e, f := people[0], people[1], people[2], people[3], people[4], people[5]

	first := handIn(t, pool, w, a, 1, "submitted")
	saved(t, pool, first, w.choice, picked("x"), mark(0), nil, false)
	latest := handIn(t, pool, w, a, 2, "submitted")
	saved(t, pool, latest, w.choice, picked("x"), mark(5), nil, false)
	saved(t, pool, latest, w.blank, emptyBlank(), mark(5), nil, false)
	saved(t, pool, latest, w.essay, essay("Tôi dậy lúc sáu giờ."), nil, mark(5), true)

	kept := handIn(t, pool, w, b, 1, "submitted")
	saved(t, pool, kept, w.choice, picked("x"), mark(5), nil, false)
	saved(t, pool, kept, w.blank, emptyBlank(), mark(2.5), nil, false)
	saved(t, pool, kept, w.essay, essay("Tôi ăn sáng."), nil, mark(2.5), true)
	voided := handIn(t, pool, w, b, 2, "voided")
	saved(t, pool, voided, w.choice, picked("x"), mark(0), nil, false)

	wrong := handIn(t, pool, w, c, 1, "submitted")
	saved(t, pool, wrong, w.choice, picked("x"), mark(0), nil, false)
	saved(t, pool, wrong, w.blank, emptyBlank(), mark(0), nil, false)
	saved(t, pool, wrong, w.essay, essay("Chưa chấm."), nil, nil, true)

	skipped := handIn(t, pool, w, d, 1, "submitted")
	saved(t, pool, skipped, w.choice, map[string]any{"type": "choice", "optionIds": []string{}}, mark(0), nil, false)
	saved(t, pool, skipped, w.blank, emptyBlank(), mark(5), nil, false)

	graded := handIn(t, pool, w, e, 1, "submitted")
	if _, err := pool.Exec(context.Background(), `UPDATE app.attempts SET status = 'graded', graded_at = now() WHERE id = $1::uuid`, graded); err != nil {
		t.Fatal(err)
	}
	saved(t, pool, graded, w.choice, picked("x"), mark(5), nil, false)
	saved(t, pool, graded, w.blank, emptyBlank(), mark(5), nil, false)
	saved(t, pool, graded, w.essay, essay("Tôi đi học."), nil, mark(5), true)

	sitting := handIn(t, pool, w, f, 1, "in_progress")
	saved(t, pool, sitting, w.choice, picked("x"), mark(5), nil, false)

	got, err := analysisOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	if got.HandedIn != 5 {
		t.Errorf("handedIn = %d, want 5: one paper for each of the five students who handed in, the latest, the voided and the unfinished ones left out", got.HandedIn)
	}

	type figures struct {
		number, answered int
		rate             float64
	}
	want := []struct {
		question string
		figures
	}{
		{w.listening, figures{4, 0, 0}},
		{w.essay, figures{3, 4, 0.5}},
		{w.choice, figures{1, 4, 0.6}},
		{w.blank, figures{2, 0, 0.6}},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("%d items, want %d: %+v", len(got.Items), len(want), got.Items)
	}
	for i, expect := range want {
		item := got.Items[i]
		if item.QuestionID != expect.question || item.Number != expect.number || item.Answered != expect.answered ||
			item.CorrectRate == nil || *item.CorrectRate != expect.rate {
			t.Errorf("item %d = {%s #%d answered %d rate %v}, want {%s #%d answered %d rate %v}",
				i, item.QuestionID, item.Number, item.Answered, deref(item.CorrectRate), expect.question, expect.number, expect.answered, expect.rate)
		}
	}
	if got.Items[0].PromptExcerpt == "" || got.Items[2].Type != "single_choice" {
		t.Errorf("the excerpt and type are not carried: %+v", got.Items)
	}
}

func deref(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func TestAQuestionNoPaperHasAMarkForSortsLastWithNoRate(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	people := enrol(t, pool, w, 1)
	only := handIn(t, pool, w, people[0], 1, "submitted")
	saved(t, pool, only, w.essay, essay("Chưa chấm."), nil, nil, true)

	got, err := analysisOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{w.choice, w.blank, w.listening, w.essay}
	if len(got.Items) != len(order) {
		t.Fatalf("%d items, want %d", len(got.Items), len(order))
	}
	for i, question := range order {
		if got.Items[i].QuestionID != question {
			t.Errorf("item %d is %s, want %s", i, got.Items[i].QuestionID, question)
		}
	}
	if last := got.Items[3]; last.CorrectRate != nil || last.Answered != 1 {
		t.Errorf("the question awaiting its only mark reads rate %v, answered %d; want no rate and 1 answered", deref(last.CorrectRate), last.Answered)
	}
	if first := got.Items[0]; first.CorrectRate == nil || *first.CorrectRate != 0 {
		t.Errorf("a question left unanswered reads rate %v, want 0", deref(first.CorrectRate))
	}
}

func TestAnAssignmentWithNoPapersHasAFigurePerQuestionAndNoRates(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	got, err := analysisOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	if got.HandedIn != 0 || len(got.Items) != 4 {
		t.Fatalf("handedIn %d with %d items, want 0 and 4", got.HandedIn, len(got.Items))
	}
	for i, item := range got.Items {
		if item.CorrectRate != nil || item.Answered != 0 || item.Number != i+1 {
			t.Errorf("item %d = %+v, want its place on the paper, nothing answered and no rate", i, item)
		}
	}
}

func TestTheItemAnalysisRangesOverTheStudentsTheReaderReaches(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	ctx := context.Background()
	ours := enrol(t, pool, w, 2)

	coTeacher, theirClass, theirStudent := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		for _, c := range []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM app.attempt_answers WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, []any{theirStudent}},
			{`DELETE FROM app.attempts WHERE student_id = $1::uuid`, []any{theirStudent}},
			{`DELETE FROM app.assignment_classes WHERE class_id = $1::uuid`, []any{theirClass}},
			{`DELETE FROM app.class_members WHERE class_id = $1::uuid`, []any{theirClass}},
			{`DELETE FROM app.classes WHERE id = $1::uuid`, []any{theirClass}},
			{`DELETE FROM app.users WHERE id = ANY($1::uuid[])`, []any{[]string{theirStudent, coTeacher}}},
		} {
			if _, err := pool.Exec(ctx, c.sql, c.args...); err != nil {
				t.Errorf("cleanup %q: %v", c.sql, err)
			}
		}
	})
	for _, u := range []struct{ id, name, role string }{
		{coTeacher, "Giáo viên khác", "teacher"}, {theirStudent, "Học viên lớp khác", "student"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO app.users (id, email, full_name, role_id)
			VALUES ($1::uuid, $2, $3, (SELECT id FROM app.roles WHERE builtin_key = $4::text))`,
			u.id, u.id+"@example.com", u.name, u.role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.classes (id, name, teacher_id) VALUES ($1::uuid, $2, $3::uuid)`,
		theirClass, "Lớp khác "+theirClass, coTeacher); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`,
		theirClass, theirStudent, coTeacher); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1::uuid, $2::uuid)`, w.assignment, theirClass); err != nil {
		t.Fatal(err)
	}
	for _, student := range []string{ours[0], ours[1], theirStudent} {
		paper := handIn(t, pool, w, student, 1, "submitted")
		saved(t, pool, paper, w.choice, picked("x"), mark(5), nil, false)
	}

	for name, c := range map[string]struct {
		scope access.Scope
		want  int
	}{
		"the teacher who made the assignment": {access.Scope{UserID: w.admin}, 3},
		"a teacher who reaches one class":     {access.Scope{UserID: coTeacher}, 1},
		"scope.all":                           {everyone, 3},
	} {
		got, err := analysisOf(t, pool, c.scope, w.assignment)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.HandedIn != c.want {
			t.Errorf("%s: handedIn = %d, want %d", name, got.HandedIn, c.want)
		}
	}

	stranger := access.Scope{UserID: uuid.NewString()}
	if _, err := analysisOf(t, pool, stranger, w.assignment); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a teacher who reaches nothing here got %v, want ErrNotFound", err)
	}
	if _, err := analysisOf(t, pool, everyone, uuid.NewString()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a missing assignment got %v, want ErrNotFound", err)
	}
}
