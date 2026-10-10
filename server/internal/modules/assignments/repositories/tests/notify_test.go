//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

type told struct {
	mu   sync.Mutex
	sent []notificationscommand.Notify
	err  error
}

func (c *told) Handle(_ context.Context, n notificationscommand.Notify) (cqrs.Nothing, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return cqrs.Nothing{}, c.err
	}
	c.sent = append(c.sent, n)
	return cqrs.Nothing{}, nil
}

func (c *told) byUser() map[string]notificationscommand.Notify {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]notificationscommand.Notify{}
	for _, n := range c.sent {
		out[n.UserID] = n
	}
	return out
}

func (c *told) users() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.sent))
	for i, n := range c.sent {
		out[i] = n.UserID
	}
	slices.Sort(out)
	return out
}

func (w *reachWorld) announcing(tell *told, log *bytes.Buffer) *application.Application {
	logger := slog.New(slog.DiscardHandler)
	if log != nil {
		logger = slog.New(slog.NewTextHandler(log, nil))
	}
	return application.New(w.store).WithNotifier(tell, logger)
}

func (w *reachWorld) extend(t *testing.T, app *application.Application, owner, assignment string, minutes int, notify bool) domain.Assignment {
	t.Helper()
	req := as(owner, owner == w.admin)
	req.ID = assignment
	extended, err := app.Commands.Extend.Handle(context.Background(), command.Extend{Request: req, Minutes: minutes, Notify: notify, Now: time.Now()})
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	return extended
}

func (w *reachWorld) grant(t *testing.T, app *application.Application, owner, assignment string, in domain.OverrideInput) {
	t.Helper()
	req := as(owner, owner == w.admin)
	req.ID = assignment
	in.Reason, in.Now = "Ốm", time.Now()
	if _, err := app.Commands.SetOverrides.Handle(context.Background(), command.SetOverrides{Request: req, Input: in}); err != nil {
		t.Fatalf("setting overrides: %v", err)
	}
}

func (w *reachWorld) setOverride(t *testing.T, assignment, student string, closes time.Time) {
	t.Helper()
	w.id(t, `INSERT INTO app.assignment_student_overrides (assignment_id, student_id, closes_at, reason)
		VALUES ($1, $2, $3, 'Lý do') RETURNING student_id::text`, assignment, student, closes)
}

func TestExtendingWithNotifyTellsTheStudentsTheirNewClose(t *testing.T) {
	w := newReachWorld(t)
	tell := &told{}
	app := w.announcing(tell, nil)

	extended := w.extend(t, app, w.a, w.mineA, 30, true)
	sent := tell.byUser()
	if len(sent) != 1 || sent[w.studentA].Kind != notificationsdomain.AssignmentExtended {
		t.Fatalf("extending class A's assignment told %+v, want its one student", tell.sent)
	}
	n := sent[w.studentA]
	if p := n.Params.(notificationsdomain.Extended); p.Title != "Đề" || !p.ClosesAt.Equal(extended.ClosesAt) {
		t.Errorf("params %+v, want the title and the new close %v", p, extended.ClosesAt)
	}
	if want := (notificationsdomain.Target{Route: notificationsdomain.RouteStudentAssignment, AssignmentID: w.mineA}); *n.Target != want {
		t.Errorf("target %+v, want %+v", *n.Target, want)
	}
	if n.DedupeKey != "extended:"+w.mineA || n.Merge != notificationsdomain.Replace {
		t.Errorf("key %q, merge %d", n.DedupeKey, n.Merge)
	}
}

func TestExtendingWithoutNotifyTellsNobody(t *testing.T) {
	w := newReachWorld(t)
	tell := &told{}
	w.extend(t, w.announcing(tell, nil), w.a, w.mineA, 30, false)
	if len(tell.sent) != 0 {
		t.Errorf("an extension nobody asked to announce told %+v", tell.sent)
	}
}

func TestEveryStudentOfASharedAssignmentIsToldOnce(t *testing.T) {
	w := newReachWorld(t)
	tell := &told{}
	w.extend(t, w.announcing(tell, nil), w.admin, w.shared, 15, true)
	if want := sortedIDs(w.studentA, w.studentB); !slices.Equal(tell.users(), want) {
		t.Errorf("told %v, want %v: the members of both target classes, and the student named as well as in a class, once", tell.users(), want)
	}
}

func TestAnOverrideThatClosesNoEarlierThanTheNewCloseKeepsItsStudentUntold(t *testing.T) {
	w := newReachWorld(t)
	late, exact, early := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
	base := w.closesAt(t, w.mineA)
	w.setOverride(t, w.mineA, late, base.Add(2*time.Hour))
	w.setOverride(t, w.mineA, exact, base.Add(time.Hour))
	w.setOverride(t, w.mineA, early, base.Add(10*time.Minute))
	tell := &told{}

	extended := w.extend(t, w.announcing(tell, nil), w.a, w.mineA, 60, true)
	if want := sortedIDs(w.studentA, early); !slices.Equal(tell.users(), want) {
		t.Fatalf("told %v, want %v: the student without an override and the one whose override closed earlier", tell.users(), want)
	}
	for user, n := range tell.byUser() {
		if closes := n.Params.(notificationsdomain.Extended).ClosesAt; !closes.Equal(extended.ClosesAt) {
			t.Errorf("%s is told %v, want the new close %v", user, closes, extended.ClosesAt)
		}
	}
}

func TestADisabledStudentAndAnAccountThatIsNoStudentAreNotTold(t *testing.T) {
	w := newReachWorld(t)
	disabled := w.classmate(t, w.classA, w.a)
	notAStudent := w.user(t, "teacher", nil)
	w.id(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2) RETURNING user_id::text`, w.classA, notAStudent)
	w.id(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1 RETURNING id::text`, disabled)
	tell := &told{}

	w.extend(t, w.announcing(tell, nil), w.a, w.mineA, 30, true)
	if want := []string{w.studentA}; !slices.Equal(tell.users(), want) {
		t.Errorf("told %v, want only %v", tell.users(), want)
	}
}

func TestNothingIsToldForADraft(t *testing.T) {
	w := newReachWorld(t)
	w.id(t, `UPDATE app.assignments SET published_at = NULL WHERE id = $1 RETURNING id::text`, w.mineA)
	tell := &told{}
	w.extend(t, w.announcing(tell, nil), w.a, w.mineA, 30, true)
	if len(tell.sent) != 0 {
		t.Errorf("students were told of a draft: %+v", tell.sent)
	}
}

func TestAFailingNotifierNeverFailsTheExtension(t *testing.T) {
	w := newReachWorld(t)
	tell := &told{err: errors.New("the notifications store is away")}
	var log bytes.Buffer
	extended := w.extend(t, w.announcing(tell, &log), w.a, w.mineA, 30, true)
	if got := w.closesAt(t, w.mineA); !got.Equal(extended.ClosesAt) {
		t.Errorf("the close is %v, want the extended %v", got, extended.ClosesAt)
	}
	if !strings.Contains(log.String(), "announcement not delivered") || !strings.Contains(log.String(), "the notifications store is away") {
		t.Errorf("the failure was logged as %q", log.String())
	}
}

func TestAnOverrideWithNotifyTellsTheStudentsItGaveTimeTo(t *testing.T) {
	w := newReachWorld(t)
	second := w.classmate(t, w.classA, w.a)
	tell := &told{}
	app := w.announcing(tell, nil)
	base := w.closesAt(t, w.mineA)

	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{w.studentA, second}, ExtendBy: ptr(90), Notify: true})
	sent := tell.byUser()
	if want := sortedIDs(w.studentA, second); !slices.Equal(tell.users(), want) {
		t.Fatalf("told %v, want %v", tell.users(), want)
	}
	for user, n := range sent {
		if closes := n.Params.(notificationsdomain.Extended).ClosesAt; !closes.Equal(base.Add(90 * time.Minute)) {
			t.Errorf("%s is told %v, want 90 minutes past the assignment's close %v", user, closes, base)
		}
		if n.DedupeKey != "extended:"+w.mineA || *n.Target != (notificationsdomain.Target{Route: notificationsdomain.RouteStudentAssignment, AssignmentID: w.mineA}) {
			t.Errorf("%s: key %q target %+v", user, n.DedupeKey, *n.Target)
		}
	}
}

func TestAnOverrideTellsOnlyThoseItGaveTimePastTheAssignmentsClose(t *testing.T) {
	w := newReachWorld(t)
	second := w.classmate(t, w.classA, w.a)
	base := w.closesAt(t, w.mineA)
	tell := &told{}
	app := w.announcing(tell, nil)

	earlier := base.Add(-10 * time.Minute)
	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{w.studentA}, ClosesAt: &earlier, Notify: true})
	if len(tell.sent) != 0 {
		t.Errorf("an override closing before the assignment does gave nobody time, and %+v was sent", tell.sent)
	}

	later := base.Add(3 * time.Hour)
	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{second}, ClosesAt: &later, Notify: true})
	if want := []string{second}; !slices.Equal(tell.users(), want) {
		t.Errorf("told %v, want %v", tell.users(), want)
	}
}

func TestAnOverrideThatMovesNoCloseOrIsNotAskedToNotifyTellsNobody(t *testing.T) {
	w := newReachWorld(t)
	tell := &told{}
	app := w.announcing(tell, nil)

	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{w.studentA}, ExtraAttempts: ptr(2), Notify: true})
	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{w.studentA}, DurationMin: ptr(90), Notify: true})
	w.grant(t, app, w.a, w.mineA, domain.OverrideInput{StudentIDs: []string{w.studentA}, ExtendBy: ptr(60), Notify: false})
	if len(tell.sent) != 0 {
		t.Errorf("told %+v, want nobody: more attempts or time to sit move no close, and notify was off for the extension", tell.sent)
	}
}
