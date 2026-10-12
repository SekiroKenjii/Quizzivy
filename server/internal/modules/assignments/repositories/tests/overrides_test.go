//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/access"
)

func ptr(n int) *int { return &n }

func after(offset time.Duration) *time.Time {
	at := time.Now().Add(offset).Truncate(time.Second)
	return &at
}

func (w *reachWorld) classmate(t *testing.T, class, teacher string) string {
	t.Helper()
	student := w.user(t, "student", &teacher)
	w.id(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2) RETURNING user_id::text`, class, student)
	return student
}

func (w *reachWorld) window(t *testing.T, owner string, class string, opens, closes time.Duration) string {
	t.Helper()
	in := input(w.versionA, []string{class}, nil)
	now := time.Now()
	in.OpensAt, in.ClosesAt = now.Add(opens), now.Add(closes)
	return w.create(t, as(owner, false), in)
}

func (w *reachWorld) closesAt(t *testing.T, id string) time.Time {
	t.Helper()
	a, err := w.store.Get(context.Background(), access.Scope{All: true}, id)
	if err != nil {
		t.Fatal(err)
	}
	return a.ClosesAt
}

func (w *reachWorld) auditRows(t *testing.T, action, assignment string) []map[string]any {
	t.Helper()
	rows, err := w.tx.Query(context.Background(), `
		SELECT actor_user_id::text, diff FROM app.audit_log
		 WHERE action = $1 AND entity = 'assignment' AND entity_id = $2::uuid
		 ORDER BY id`, action, assignment)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var actor string
		var raw []byte
		if err := rows.Scan(&actor, &raw); err != nil {
			t.Fatal(err)
		}
		var diff map[string]any
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatal(err)
		}
		diff["actor"] = actor
		out = append(out, diff)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func overrideOf(t *testing.T, found []domain.StudentOverride, student string) domain.StudentOverride {
	t.Helper()
	i := slices.IndexFunc(found, func(o domain.StudentOverride) bool { return o.StudentID == student })
	if i < 0 {
		t.Fatalf("no override for %s in %+v", student, found)
	}
	return found[i]
}

func TestExtendingMovesTheCloseForEveryoneAndRecordsWhy(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA
	req.IP, req.UserAgent = "203.0.113.7", "go-test"
	before := w.closesAt(t, w.mineA)

	extended, err := w.store.Extend(ctx, req, 30, true, time.Now())
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if want := before.Add(30 * time.Minute); !extended.ClosesAt.Equal(want) {
		t.Errorf("closesAt = %v, want %v", extended.ClosesAt, want)
	}
	if got := w.closesAt(t, w.mineA); !got.Equal(extended.ClosesAt) {
		t.Errorf("the stored close is %v, the answer said %v", got, extended.ClosesAt)
	}

	rows := w.auditRows(t, "assignment.extended", w.mineA)
	if len(rows) != 1 || rows[0]["actor"] != w.a || rows[0]["minutes"] != float64(30) || rows[0]["notify"] != true {
		t.Fatalf("audit rows = %v, want one by the actor naming 30 minutes and notify", rows)
	}
	closes, _ := rows[0]["closes_at"].(map[string]any)
	if closes["old"] == nil || closes["new"] == nil || closes["old"] == closes["new"] {
		t.Errorf("the audited close = %v, want an old and a different new", closes)
	}
}

func TestExtendingAnAssignmentThatHasNotClosedYet(t *testing.T) {
	w := newReachWorld(t)
	scheduled := w.window(t, w.a, w.classA, time.Hour, 2*time.Hour)
	req := as(w.a, false)
	req.ID = scheduled
	before := w.closesAt(t, scheduled)
	if _, err := w.store.Extend(context.Background(), req, 10080, false, time.Now()); err != nil {
		t.Fatalf("extending a scheduled assignment: %v", err)
	}
	if got := w.closesAt(t, scheduled); !got.Equal(before.Add(7 * 24 * time.Hour)) {
		t.Errorf("closesAt = %v, want a week after %v", got, before)
	}
}

func TestExtendingRefusesWhatHasClosedAndWhatIsAnotherTeachers(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	w.id(t, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1 RETURNING id::text`, w.mineA)
	ended := w.window(t, w.a, w.classA, -3*time.Hour, -time.Hour)
	before := w.state(t, w.mineA)

	for label, c := range map[string]struct {
		req  domain.Request
		id   string
		want error
	}{
		"closed early":        {as(w.a, false), w.mineA, domain.ErrClosed},
		"its window ended":    {as(w.a, false), ended, domain.ErrClosed},
		"another teacher's":   {as(w.b, false), w.mineA, domain.ErrNotFound},
		"a missing one":       {as(w.a, false), uuid.NewString(), domain.ErrNotFound},
		"another's, an Admin": {as(w.admin, true), w.mineB, nil},
	} {
		c.req.ID = c.id
		if _, err := w.store.Extend(ctx, c.req, 15, false, time.Now()); !errors.Is(err, c.want) {
			t.Errorf("extending %s: %v, want %v", label, err, c.want)
		}
	}
	if got := w.state(t, w.mineA); got != before {
		t.Errorf("a refused extension changed the assignment:\nbefore %s\nafter  %s", before, got)
	}
	if rows := w.auditRows(t, "assignment.extended", w.mineA); len(rows) != 0 {
		t.Errorf("a refused extension was audited: %v", rows)
	}
}

func TestAnOverrideIsMergedNotReplacedAndEveryWriteIsAudited(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA

	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ExtraAttempts: ptr(1), Reason: "  đau ốm  ", Notify: true, Now: time.Now(),
	}); err != nil {
		t.Fatalf("first override: %v", err)
	}
	written, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, DurationMin: ptr(90), Reason: "thêm giờ", Now: time.Now(),
	})
	if err != nil {
		t.Fatalf("second override: %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("written = %+v, want the one student", written)
	}
	got := written[0]
	if got.ExtraAttempts != 1 || got.DurationMin == nil || *got.DurationMin != 90 || got.ClosesAt != nil || got.Reason != "thêm giờ" {
		t.Errorf("merged override = %+v, want the attempt kept, 90 minutes added, no close and the new reason", got)
	}
	if got.StudentName == "" {
		t.Error("the override names no student")
	}

	rows := w.auditRows(t, "assignment.override_set", w.mineA)
	if len(rows) != 2 {
		t.Fatalf("audit rows = %v, want one per write", rows)
	}
	if rows[0]["reason"] != "đau ốm" || rows[0]["studentId"] != w.studentA || rows[0]["actor"] != w.a || rows[0]["notify"] != true {
		t.Errorf("first audit row = %v, want the trimmed reason, the student, the actor and notify", rows[0])
	}
	extra, _ := rows[1]["extra_attempts"].(map[string]any)
	duration, _ := rows[1]["duration_minutes"].(map[string]any)
	if extra["old"] != float64(1) || extra["new"] != float64(1) || duration["old"] != nil || duration["new"] != float64(90) {
		t.Errorf("second audit row = %v, want attempts 1 to 1 and minutes null to 90", rows[1])
	}

	cleared, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ExtraAttempts: ptr(0), Reason: "bỏ lượt thêm", Now: time.Now(),
	})
	if err != nil {
		t.Fatalf("clearing the attempts while minutes remain: %v", err)
	}
	if got := cleared[0]; got.ExtraAttempts != 0 || got.DurationMin == nil || *got.DurationMin != 90 {
		t.Errorf("after clearing the attempts the override is %+v, want 0 attempts and the 90 minutes kept", got)
	}
}

func TestExtendingByStartsFromTheStudentsOwnClose(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA
	base := w.closesAt(t, w.mineA)

	for step, want := range []time.Time{base.Add(30 * time.Minute), base.Add(60 * time.Minute)} {
		written, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
			StudentIDs: []string{w.studentA}, ExtendBy: ptr(30), Reason: "gia hạn", Now: time.Now(),
		})
		if err != nil {
			t.Fatalf("extension %d: %v", step+1, err)
		}
		if got := written[0].ClosesAt; got == nil || !got.Equal(want) {
			t.Errorf("after extension %d the student closes at %v, want %v", step+1, got, want)
		}
	}
	if got := w.closesAt(t, w.mineA); !got.Equal(base) {
		t.Errorf("the assignment closes at %v, want it untouched at %v", got, base)
	}
}

func TestAnOverrideOpensAClosedAssignmentForThatStudentAlone(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	other := w.classmate(t, w.classA, w.a)
	ended := w.window(t, w.a, w.classA, -3*time.Hour, -time.Hour)
	req := as(w.a, false)
	req.ID = ended

	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ExtendBy: ptr(30), Reason: "gia hạn", Now: time.Now(),
	}); !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("extending a student whose close has passed: %v, want ErrClosed", err)
	}
	if found, err := w.store.Overrides(ctx, access.Scope{All: true}, ended); err != nil || len(found) != 0 {
		t.Fatalf("a refused request left %+v (%v)", found, err)
	}

	until := after(2 * time.Hour)
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ClosesAt: until, DurationMin: ptr(30), Reason: "mở lại cho em", Now: time.Now(),
	}); err != nil {
		t.Fatalf("reopening for one student: %v", err)
	}

	now := time.Now()
	sections, err := w.store.ForStudent(ctx, w.studentA, now)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(sections.DueNow, func(c domain.StudentCard) bool { return c.ID == ended })
	if i < 0 {
		t.Fatalf("the reopened student's due now holds %d cards, none of them the assignment", len(sections.DueNow))
	}
	card := sections.DueNow[i]
	if status := domain.Schedule.StatusAt(now, card.PublishedAt, card.OpensAt, card.ClosesAt, card.ClosedAt); status != domain.Open {
		t.Errorf("the student's status is %s, want open", status)
	}
	if !card.ClosesAt.Equal(*until) || card.DurationMin != 30 {
		t.Errorf("the card closes at %v for %d minutes, want %v for 30", card.ClosesAt, card.DurationMin, *until)
	}

	elsewhere, err := w.store.ForStudent(ctx, other, now)
	if err != nil {
		t.Fatal(err)
	}
	for name, cards := range map[string][]domain.StudentCard{"due now": elsewhere.DueNow, "upcoming": elsewhere.Upcoming, "completed": elsewhere.Completed} {
		if slices.ContainsFunc(cards, func(c domain.StudentCard) bool { return c.ID == ended }) {
			t.Errorf("a classmate without an override has the closed assignment under %s", name)
		}
	}
	if _, err := w.store.StudentDetail(ctx, ended, other); err != nil {
		t.Fatalf("the classmate's intro: %v", err)
	}
}

func TestAStudentsCardAndIntroCarryTheirOwnWindow(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	other := w.classmate(t, w.classA, w.a)
	req := as(w.a, false)
	req.ID = w.mineA
	until := after(3 * time.Hour)
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ClosesAt: until, DurationMin: ptr(90), ExtraAttempts: ptr(2), Reason: "nhu cầu riêng", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	own, err := w.store.StudentDetail(ctx, w.mineA, w.studentA)
	if err != nil {
		t.Fatal(err)
	}
	if !own.ClosesAt.Equal(*until) || own.DurationMin != 90 || own.MaxAttempts != 3 {
		t.Errorf("the student's intro says %v, %d minutes, %d attempts; want %v, 90, 3", own.ClosesAt, own.DurationMin, own.MaxAttempts, *until)
	}
	plain, err := w.store.StudentDetail(ctx, w.mineA, other)
	if err != nil {
		t.Fatal(err)
	}
	if plain.ClosesAt.Equal(*until) || plain.DurationMin != 45 || plain.MaxAttempts != 1 {
		t.Errorf("a classmate's intro says %v, %d minutes, %d attempts; want the assignment's own, 45 and 1", plain.ClosesAt, plain.DurationMin, plain.MaxAttempts)
	}
	sections, err := w.store.ForStudent(ctx, w.studentA, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(sections.DueNow, func(c domain.StudentCard) bool { return c.ID == w.mineA })
	if i < 0 || sections.DueNow[i].MaxAttempts != 3 || !sections.DueNow[i].ClosesAt.Equal(*until) {
		t.Errorf("the student's home card = %+v, want 3 attempts closing at %v", sections.DueNow, *until)
	}
}

func TestAnOverrideNamesOnlyStudentsOfTheAssignmentTheActorReaches(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	disabled := w.classmate(t, w.classA, w.a)
	w.id(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1 RETURNING id::text`, disabled)
	missing := uuid.NewString()
	req := as(w.a, false)
	req.ID = w.shared
	in := func(ids ...string) domain.OverrideInput {
		return domain.OverrideInput{StudentIDs: ids, ExtraAttempts: ptr(1), Reason: "kiểm tra", Now: time.Now()}
	}

	for label, c := range map[string]struct {
		ids  []string
		want []string
	}{
		"a student of another teacher's class":  {[]string{w.studentB}, []string{w.studentB}},
		"a student who does not exist":          {[]string{missing}, []string{missing}},
		"a teacher":                             {[]string{w.b}, []string{w.b}},
		"a disabled student":                    {[]string{disabled}, []string{disabled}},
		"a good one with a bad one, both named": {[]string{w.studentA, w.studentB, missing}, sortedIDs(w.studentB, missing)},
	} {
		_, err := w.store.SetOverrides(ctx, req, in(c.ids...))
		var refused *domain.NotTargetedError
		if !errors.As(err, &refused) {
			t.Errorf("%s: %v, want a NotTargetedError", label, err)
			continue
		}
		if got := sortedIDs(refused.StudentIDs...); !slices.Equal(got, c.want) {
			t.Errorf("%s: the error names %v, want %v", label, got, c.want)
		}
	}
	if found, err := w.store.Overrides(ctx, access.Scope{All: true}, w.shared); err != nil || len(found) != 0 {
		t.Errorf("refused requests left %+v (%v), want nothing written", found, err)
	}

	admin := as(w.admin, true)
	admin.ID = w.shared
	if _, err := w.store.SetOverrides(ctx, admin, in(w.studentA, w.studentB)); err != nil {
		t.Errorf("the Admin overriding both classes' students: %v", err)
	}
	notOurs := as(w.b, false)
	notOurs.ID = w.mineA
	if _, err := w.store.SetOverrides(ctx, notOurs, in(w.studentA)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B overriding A's assignment: %v, want not found", err)
	}
}

func TestAnOverrideThatWouldChangeNothingIsRefusedAndNothingIsKept(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA
	_, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ExtraAttempts: ptr(0), Reason: "không đổi gì", Now: time.Now(),
	})
	fields := fieldsOf(t, err)
	if _, ok := fields["extraAttempts"]; !ok {
		t.Errorf("fields = %v, want one on extraAttempts", fields)
	}
	if found, err := w.store.Overrides(ctx, access.Scope{All: true}, w.mineA); err != nil || len(found) != 0 {
		t.Errorf("the refused request left %+v (%v)", found, err)
	}
	if rows := w.auditRows(t, "assignment.override_set", w.mineA); len(rows) != 0 {
		t.Errorf("the refused request was audited: %v", rows)
	}
}

func TestOverridesAreListedAndRemovedWithinTheActorsReach(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	admin := as(w.admin, true)
	admin.ID = w.shared
	if _, err := w.store.SetOverrides(ctx, admin, domain.OverrideInput{
		StudentIDs: []string{w.studentA, w.studentB}, ClosesAt: after(5 * time.Hour), ExtraAttempts: ptr(1), Reason: "cả hai", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	for label, c := range map[string]struct {
		scope access.Scope
		want  []string
	}{
		"A":         {access.Scope{UserID: w.a}, []string{w.studentA}},
		"B":         {access.Scope{UserID: w.b}, []string{w.studentB}},
		"scope.all": {access.Scope{UserID: w.admin, All: true}, sortedIDs(w.studentA, w.studentB)},
	} {
		found, err := w.store.Overrides(ctx, c.scope, w.shared)
		if err != nil {
			t.Fatalf("%s listing: %v", label, err)
		}
		var got []string
		for _, o := range found {
			got = append(got, o.StudentID)
		}
		if !slices.Equal(sortedIDs(got...), c.want) {
			t.Errorf("%s lists %v, want %v", label, got, c.want)
		}
	}
	if _, err := w.store.Overrides(ctx, access.Scope{UserID: w.b}, w.mineA); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B listing A's assignment: %v, want not found", err)
	}
	if _, err := w.store.Overrides(ctx, access.Scope{UserID: w.a}, uuid.NewString()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("listing a missing assignment: %v, want not found", err)
	}

	remove := func(req domain.Request, student string) error {
		req.ID = w.shared
		return w.store.DeleteOverride(ctx, req, student, time.Now())
	}
	b := as(w.b, false)
	b.IP = "203.0.113.9"
	if err := remove(b, w.studentA); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B removing A's student's override: %v, want not found", err)
	}
	if err := remove(b, uuid.NewString()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B removing a missing override: %v, want not found", err)
	}
	if err := remove(b, w.studentB); err != nil {
		t.Fatalf("B removing their student's override: %v", err)
	}
	if err := remove(b, w.studentB); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("removing it twice: %v, want not found the second time", err)
	}
	rows := w.auditRows(t, "assignment.override_removed", w.shared)
	if len(rows) != 1 || rows[0]["studentId"] != w.studentB || rows[0]["actor"] != w.b || rows[0]["reason"] != "cả hai" {
		t.Errorf("removal audit = %v, want one by B for B's student with the reason it had", rows)
	}
	left, err := w.store.Overrides(ctx, access.Scope{All: true}, w.shared)
	if err != nil || len(left) != 1 || left[0].StudentID != w.studentA {
		t.Errorf("left %+v (%v), want only A's student's", left, err)
	}
	if got := overrideOf(t, left, w.studentA); got.ExtraAttempts != 1 {
		t.Errorf("A's student's override = %+v", got)
	}
}

func TestTheOverrideTableHoldsItsOwnBounds(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	missing := uuid.NewString()
	for name, c := range map[string]struct {
		student, columns, values, constraint string
	}{
		"no time limit under a minute":  {w.studentA, "duration_minutes, reason", "0, 'lý do'", "assignment_student_overrides_duration_check"},
		"no time limit over ten hours":  {w.studentA, "duration_minutes, reason", "601, 'lý do'", "assignment_student_overrides_duration_check"},
		"no more than ten extra tries":  {w.studentA, "extra_attempts, reason", "11, 'lý do'", "assignment_student_overrides_extra_attempts_check"},
		"no empty reason":               {w.studentA, "closes_at, reason", "now(), ''", "assignment_student_overrides_reason_check"},
		"no reason past 500 characters": {w.studentA, "closes_at, reason", "now(), repeat('a', 501)", "assignment_student_overrides_reason_check"},
		"no row that changes nothing":   {w.studentA, "extra_attempts, reason", "0, 'lý do'", "assignment_student_overrides_changes_check"},
		"no override without a reason":  {w.studentA, "extra_attempts, reason", "1, NULL", ""},
		"no override on a missing user": {missing, "extra_attempts, reason", "1, 'lý do'", "assignment_student_overrides_student_id_fkey"},
	} {
		sp, err := w.tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = sp.Exec(ctx, `INSERT INTO app.assignment_student_overrides (assignment_id, student_id, `+c.columns+`) VALUES ($1, $2, `+c.values+`)`, w.mineA, c.student)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Errorf("%s: %v, want a database error", name, err)
		} else if c.constraint != "" && pgErr.ConstraintName != c.constraint {
			t.Errorf("%s: refused by %q, want %q", name, pgErr.ConstraintName, c.constraint)
		}
		if err := sp.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnOverrideGoesWithItsAssignmentAndItsStudent(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA
	leaving := w.classmate(t, w.classA, w.a)
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA, leaving}, ExtraAttempts: ptr(1), Reason: "kiểm tra", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	count := func() string {
		return w.id(t, `SELECT count(*)::text FROM app.assignment_student_overrides WHERE assignment_id = $1`, w.mineA)
	}
	if got := count(); got != "2" {
		t.Fatalf("%s overrides, want 2", got)
	}
	w.id(t, `DELETE FROM app.class_members WHERE user_id = $1 RETURNING user_id::text`, leaving)
	w.id(t, `DELETE FROM app.users WHERE id = $1 RETURNING id::text`, leaving)
	if got := count(); got != "1" {
		t.Errorf("%s overrides after the student was deleted, want 1", got)
	}
	w.id(t, `DELETE FROM app.assignments WHERE id = $1 RETURNING id::text`, w.mineA)
	if got := count(); got != "0" {
		t.Errorf("%s overrides after the assignment was deleted, want 0", got)
	}
}
