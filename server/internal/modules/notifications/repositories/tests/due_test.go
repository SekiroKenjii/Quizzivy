//go:build integration

package repositories_test

import (
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/domain"
)

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

func TestAnAssignmentIsOpenedToAStudentOnceItOpensAndUntilSevenDaysPass(t *testing.T) {
	cases := []struct {
		name  string
		opens time.Duration
		want  bool
	}{
		{"opened a moment ago", -time.Minute, true},
		{"opens exactly now", 0, true},
		{"opens a second from now", time.Second, false},
		{"opened a second inside the seven days", -7*day + time.Second, true},
		{"opened exactly seven days ago", -7 * day, false},
		{"opened eight days ago", -8 * day, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			assignment := w.assignment(spec{opens: at(c.opens), closes: at(20 * day)})
			opened := ofKind(w.due(w.student), domain.AssignmentOpened)
			if (len(opened) == 1) != c.want {
				t.Fatalf("%d opened notices, want %v", len(opened), c.want)
			}
			if !c.want {
				return
			}
			if opened[0].DedupeKey != "opened:"+assignment {
				t.Errorf("dedupe key %q", opened[0].DedupeKey)
			}
			if got := opened[0].Params.(domain.Opened); got.Title != "Đề kiểm tra Unit 5" || !got.ClosesAt.Equal(at(20*day)) {
				t.Errorf("params %+v", got)
			}
			if want := (domain.Target{Route: domain.RouteStudentAssignment, AssignmentID: assignment}); *opened[0].Target != want {
				t.Errorf("target %+v, want %+v", *opened[0].Target, want)
			}
		})
	}
}

func TestAnAssignmentPublishedAfterItOpenedOpensWhenItWasPublished(t *testing.T) {
	w := newDueWorld(t)
	w.assignment(spec{opens: at(-30 * day), closes: at(5 * day), published: moment(-time.Hour)})
	if opened := ofKind(w.due(w.student), domain.AssignmentOpened); len(opened) != 1 {
		t.Errorf("%d opened notices for a test published an hour ago, want one", len(opened))
	}
	w2 := newDueWorld(t)
	w2.assignment(spec{opens: at(-30 * day), closes: at(5 * day), published: moment(-8 * day)})
	if opened := ofKind(w2.due(w2.student), domain.AssignmentOpened); len(opened) != 0 {
		t.Errorf("%d opened notices for a test published eight days ago, want none", len(opened))
	}
}

func TestNothingIsOpenedOrDueForAClosedAssignmentADraftOrAStrangersTest(t *testing.T) {
	w := newDueWorld(t)
	w.assignment(spec{opens: at(-2 * day), closes: at(-time.Second)})
	w.assignment(spec{opens: at(-time.Hour), closes: at(hour), draft: true})
	stranger := w.user("teacher")
	w.assignment(spec{creator: stranger, classes: []string{w.classOf(stranger)}, opens: at(-time.Hour), closes: at(hour)})
	if got := w.due(w.student); len(got) != 0 {
		t.Errorf("a closed assignment, a draft and another class's test earned %d notices: %+v", len(got), got)
	}
}

func TestAReminderFallsADayAndAnHourBeforeTheStudentsClose(t *testing.T) {
	cases := []struct {
		name   string
		closes time.Duration
		leads  []domain.Lead
	}{
		{"a day and a second ahead", day + time.Second, nil},
		{"exactly a day ahead", day, []domain.Lead{domain.LeadDay}},
		{"an hour and a second ahead", hour + time.Second, []domain.Lead{domain.LeadDay}},
		{"exactly an hour ahead", hour, []domain.Lead{domain.LeadHour}},
		{"a second ahead", time.Second, []domain.Lead{domain.LeadHour}},
		{"closing now", 0, nil},
		{"closed a second ago", -time.Second, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			assignment := w.assignment(spec{opens: at(-3 * day), closes: at(c.closes)})
			var want []string
			for _, lead := range c.leads {
				want = append(want, domain.DueSoonKey(assignment, at(c.closes), lead))
			}
			slices.Sort(want)
			got := ofKind(w.due(w.student), domain.AssignmentDueSoon)
			if !slices.Equal(keysOf(got), want) {
				t.Fatalf("reminders %v, want %v", keysOf(got), want)
			}
			for _, n := range got {
				if p := n.Params.(domain.DueSoon); p.Title != "Đề kiểm tra Unit 5" || !p.ClosesAt.Equal(at(c.closes)) {
					t.Errorf("params %+v", p)
				}
			}
		})
	}
}

func TestAReminderIsNotMadeForAMomentBeforeTheTestWasAvailable(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-2 * hour), closes: at(hour)})
	got := ofKind(w.due(w.student), domain.AssignmentDueSoon)
	want := []string{domain.DueSoonKey(assignment, at(hour), domain.LeadHour)}
	if !slices.Equal(keysOf(got), want) {
		t.Errorf("reminders %v, want only the hour: the day before the close was before it opened", keysOf(got))
	}
}

func TestAStudentWhoHandedInOrHasNoAttemptLeftIsNotReminded(t *testing.T) {
	deadline := at(30 * time.Minute)
	cases := []struct {
		name        string
		maxAttempts int
		status      string
		deadline    time.Time
		reminded    bool
	}{
		{"has not started", 1, "", time.Time{}, true},
		{"submitted", 3, "submitted", at(-time.Hour), false},
		{"graded", 3, "graded", at(-time.Hour), false},
		{"timed out", 3, "timed_out", at(-time.Hour), false},
		{"an attempt left open past its deadline", 3, "in_progress", at(-time.Minute), false},
		{"a live attempt that is the only one", 1, "in_progress", deadline, false},
		{"a live attempt with more to come", 3, "in_progress", deadline, true},
		{"only a voided attempt", 1, "voided", at(-time.Hour), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			assignment := w.assignment(spec{opens: at(-3 * day), closes: at(hour), maxAttempts: c.maxAttempts})
			if c.status != "" {
				w.attempt(assignment, w.student, c.status, c.deadline)
			}
			got := ofKind(w.due(w.student), domain.AssignmentDueSoon)
			if (len(got) == 1) != c.reminded || len(got) > 1 {
				t.Errorf("%d reminders, reminded = %v", len(got), c.reminded)
			}
		})
	}
}

func TestAnEarlyCloseSetsTheMomentAndEndsTheReminders(t *testing.T) {
	w := newDueWorld(t)
	w.assignment(spec{opens: at(-3 * day), closes: at(2 * day), closedAt: moment(-time.Hour)})
	if got := w.due(w.student); len(got) != 0 {
		t.Errorf("an assignment closed an hour ago earned %d notices: %+v", len(got), got)
	}

	soon := newDueWorld(t)
	assignment := soon.assignment(spec{opens: at(-3 * day), closes: at(3 * day), closedAt: moment(30 * time.Minute)})
	got := ofKind(soon.due(soon.student), domain.AssignmentDueSoon)
	want := []string{domain.DueSoonKey(assignment, at(30*time.Minute), domain.LeadHour)}
	if !slices.Equal(keysOf(got), want) {
		t.Fatalf("reminders %v, want the hour's, keyed on the early close %v", keysOf(got), want)
	}
	for _, n := range got {
		if !n.Params.(domain.DueSoon).ClosesAt.Equal(at(30 * time.Minute)) {
			t.Errorf("the reminder names %v as the close, want the early close", n.Params.(domain.DueSoon).ClosesAt)
		}
	}
}

func TestAnOverrideMovesOnlyThatStudentsReminders(t *testing.T) {
	w := newDueWorld(t)
	second := w.user("student")
	w.join(w.class, second)
	assignment := w.assignment(spec{opens: at(-10 * day), closes: at(hour)})
	w.override(assignment, second, moment(2*day+hour), 0)

	if got := ofKind(w.due(w.student), domain.AssignmentDueSoon); len(got) != 1 {
		t.Errorf("the student without an override has %d reminders, want the hour's", len(got))
	}
	if got := w.due(second); len(got) != 0 {
		t.Errorf("a student whose override closes two days later has %+v", got)
	}

	later := ofKind(w.dueAt(second, at(2*day)), domain.AssignmentDueSoon)
	wantKey := domain.DueSoonKey(assignment, at(2*day+hour), domain.LeadHour)
	if !slices.Contains(keysOf(later), wantKey) {
		t.Errorf("two days on, the override's student has %v, want the hour before their own close %s", keysOf(later), wantKey)
	}
}

func TestAnOverrideThatReachesPastAnEarlyCloseKeepsThatStudentOpen(t *testing.T) {
	w := newDueWorld(t)
	second := w.user("student")
	w.join(w.class, second)
	assignment := w.assignment(spec{opens: at(-3 * day), closes: at(2 * day), closedAt: moment(-time.Hour)})
	w.override(assignment, second, moment(hour), 0)

	if got := w.due(w.student); len(got) != 0 {
		t.Errorf("a student closed an hour ago has %+v", got)
	}
	got := ofKind(w.due(second), domain.AssignmentDueSoon)
	want := []string{domain.DueSoonKey(assignment, at(hour), domain.LeadHour)}
	if !slices.Equal(keysOf(got), want) {
		t.Errorf("the student the override reopened has %v, want the hour's reminder for their own close, %v", keysOf(got), want)
	}
}

func TestAnExtendedCloseEarnsItsRemindersAgainAndAnUnchangedOneDoesNot(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-10 * day), closes: at(2 * hour)})
	dayFirst := w.due(w.student)
	if written, err := w.store.InsertAbsent(t.Context(), w.student, dayFirst); err != nil || written != 1 {
		t.Fatalf("writing the first reminder: %d, %v", written, err)
	}
	if again, err := w.store.InsertAbsent(t.Context(), w.student, w.due(w.student)); err != nil || again != 0 {
		t.Errorf("the same close wrote %d more, %v; want none", again, err)
	}

	hourLater := w.dueAt(w.student, at(hour+time.Minute))
	if written, err := w.store.InsertAbsent(t.Context(), w.student, hourLater); err != nil || written != 1 {
		t.Fatalf("writing the hour's reminder an hour on: %d, %v; want 1", written, err)
	}

	w.exec(`UPDATE app.assignments SET closes_at = $2 WHERE id = $1::uuid`, assignment, at(5*hour))
	later := w.dueAt(w.student, at(4*hour+30*time.Minute))
	got := ofKind(later, domain.AssignmentDueSoon)
	newClose := at(5 * hour)
	if want := []string{domain.DueSoonKey(assignment, newClose, domain.LeadHour)}; !slices.Equal(keysOf(got), want) {
		t.Errorf("after the extension: %v, want the hour's reminder for the new close %v", keysOf(got), want)
	}
	if written, err := w.store.InsertAbsent(t.Context(), w.student, later); err != nil || written != 1 {
		t.Errorf("writing the extended close's reminder: %d, %v; want 1", written, err)
	}
	var held int
	if err := w.tx.QueryRow(t.Context(), `SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'assignment.due_soon'`, w.student).Scan(&held); err != nil || held != 3 {
		t.Errorf("the student holds %d reminders, want the day's and the hour's for the old close and the hour's for the new: %v", held, err)
	}
}

func TestAFirstReadInsideTheLastHourYieldsTheHoursReminderAlone(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-10 * day), closes: at(40 * time.Minute)})
	got := ofKind(w.due(w.student), domain.AssignmentDueSoon)
	if want := []string{domain.DueSoonKey(assignment, at(40*time.Minute), domain.LeadHour)}; !slices.Equal(keysOf(got), want) {
		t.Errorf("reminders %v, want the hour's alone: the day's would only repeat it", keysOf(got))
	}
}

func TestTheDaysReminderIsStillWrittenWhileTheHoursIsAhead(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-10 * day), closes: at(23 * hour)})
	got := ofKind(w.due(w.student), domain.AssignmentDueSoon)
	if want := []string{domain.DueSoonKey(assignment, at(23*hour), domain.LeadDay)}; !slices.Equal(keysOf(got), want) {
		t.Errorf("reminders %v, want the day's", keysOf(got))
	}
}

func TestATeacherIsToldHowManyStudentsTheyReachHaveNotHandedInAnHourBeforeTheClose(t *testing.T) {
	w := newDueWorld(t)
	teacherX, teacherY, stranger := w.creator, w.user("teacher"), w.user("teacher")
	classX, classY := w.class, w.classOf(teacherY)
	creator := w.user("teacher")
	x1, x2 := w.student, w.user("student")
	y1, y2, y3 := w.user("student"), w.user("student"), w.user("student")
	gone, notAStudent := w.user("student"), w.user("teacher")
	w.join(classX, x2)
	w.join(classX, gone)
	w.join(classX, notAStudent)
	for _, y := range []string{y1, y2, y3} {
		w.join(classY, y)
	}
	w.disabled(gone)

	assignment := w.assignment(spec{creator: creator, classes: []string{classX, classY}, opens: at(-2 * day), closes: at(30 * time.Minute)})
	count := func(user string) int {
		t.Helper()
		got := ofKind(w.due(user), domain.AssignmentClosing)
		if len(got) == 0 {
			return 0
		}
		if len(got) != 1 {
			t.Fatalf("%s has %d closing notices", user, len(got))
		}
		closing := got[0].Params.(domain.Closing)
		if closing.Title != "Đề kiểm tra Unit 5" || got[0].DedupeKey != domain.ClosingKey(assignment, at(30*time.Minute)) {
			t.Errorf("closing notice %+v under %q", closing, got[0].DedupeKey)
		}
		if want := (domain.Target{Route: domain.RouteAssignment, AssignmentID: assignment}); *got[0].Target != want {
			t.Errorf("target %+v, want %+v", *got[0].Target, want)
		}
		return closing.NotSubmitted
	}

	if c, x, y, z := count(creator), count(teacherX), count(teacherY), count(stranger); c != 5 || x != 2 || y != 3 || z != 0 {
		t.Fatalf("not submitted: creator %d, class X %d, class Y %d, a stranger %d; want 5, 2, 3, 0", c, x, y, z)
	}

	w.attempt(assignment, x1, "submitted", at(-time.Hour))
	w.attempt(assignment, y1, "timed_out", at(-time.Hour))
	w.attempt(assignment, y2, "in_progress", at(-time.Minute))
	w.attempt(assignment, y3, "voided", at(-time.Hour))
	if c, x, y := count(creator), count(teacherX), count(teacherY); c != 2 || x != 1 || y != 1 {
		t.Errorf("after three hand in: creator %d, class X %d, class Y %d; want 2, 1, 1 (a voided paper is not a hand-in)", c, x, y)
	}

	w.override(assignment, x2, moment(2*day), 0)
	if c, x := count(creator), count(teacherX); c != 1 || x != 0 {
		t.Errorf("a student whose override closes later still counts: creator %d, class X %d; want 1, 0", c, x)
	}
}

func TestAClosingNoticeFallsExactlyAnHourBeforeTheAssignmentClose(t *testing.T) {
	cases := []struct {
		name   string
		closes time.Duration
		want   bool
	}{
		{"an hour and a second", hour + time.Second, false},
		{"exactly an hour", hour, true},
		{"a second", time.Second, true},
		{"now", 0, false},
		{"a second ago", -time.Second, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			w.assignment(spec{opens: at(-2 * day), closes: at(c.closes)})
			if got := ofKind(w.due(w.creator), domain.AssignmentClosing); (len(got) == 1) != c.want {
				t.Errorf("%d closing notices, want %v", len(got), c.want)
			}
		})
	}
}

func TestNobodyIsToldOfAClosingWhenEveryoneHasHandedIn(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-2 * day), closes: at(30 * time.Minute)})
	w.attempt(assignment, w.student, "submitted", at(-time.Hour))
	if got := ofKind(w.due(w.creator), domain.AssignmentClosing); len(got) != 0 {
		t.Errorf("the only student handed in, and the teacher is told %+v", got)
	}
}

func TestAnEarlyCloseAHourAwayWarnsTheTeacherOfThatClose(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{opens: at(-2 * day), closes: at(3 * day), closedAt: moment(30 * time.Minute)})
	got := ofKind(w.due(w.creator), domain.AssignmentClosing)
	if len(got) != 1 || got[0].DedupeKey != domain.ClosingKey(assignment, at(30*time.Minute)) {
		t.Errorf("closing notices %+v, want one keyed on the early close", got)
	}
}

func TestAResultAnAfterCloseReleaseJustMadeReadyIsAnnouncedWithoutAScore(t *testing.T) {
	cases := []struct {
		name   string
		spec   spec
		status string
		manual bool
		want   bool
	}{
		{"graded, released by a close an hour ago", spec{release: "after_close", closes: at(-hour)}, "graded", false, true},
		{"submitted with nothing left to mark", spec{release: "after_close", closes: at(-hour)}, "submitted", false, true},
		{"timed out with nothing left to mark", spec{release: "after_close", closes: at(-hour)}, "timed_out", false, true},
		{"still waiting for a mark", spec{release: "after_close", closes: at(-hour)}, "submitted", true, false},
		{"voided", spec{release: "after_close", closes: at(-hour)}, "voided", false, false},
		{"not yet closed", spec{release: "after_close", closes: at(hour)}, "graded", false, false},
		{"released on submit, so announced when it was graded", spec{release: "on_submit", closes: at(-hour)}, "graded", false, false},
		{"a policy that hides the score", spec{release: "after_close", closes: at(-hour), hideScore: true}, "graded", false, false},
		{"closed a second inside the seven days", spec{release: "after_close", closes: at(-7*day + time.Second)}, "graded", false, true},
		{"closed exactly seven days ago", spec{release: "after_close", closes: at(-7 * day)}, "graded", false, false},
		{"released by an early close", spec{release: "after_close", closes: at(day), closedAt: moment(-hour)}, "graded", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			c.spec.opens = c.spec.closes.Add(-3 * day)
			if c.spec.closedAt != nil {
				c.spec.opens = at(-3 * day)
			}
			assignment := w.assignment(c.spec)
			attempt := w.attempt(assignment, w.student, c.status, at(-2*hour))
			if c.manual {
				w.pending(attempt)
			}
			got := ofKind(w.due(w.student), domain.ResultReady)
			if (len(got) == 1) != c.want {
				t.Fatalf("%d result notices, want %v", len(got), c.want)
			}
			if !c.want {
				return
			}
			if got[0].Params != (domain.Ready{Title: "Đề kiểm tra Unit 5"}) || got[0].DedupeKey != domain.ReadyKey(attempt) {
				t.Errorf("params %+v under %q", got[0].Params, got[0].DedupeKey)
			}
			if want := (domain.Target{Route: domain.RouteResult, AssignmentID: assignment, AttemptID: attempt}); *got[0].Target != want {
				t.Errorf("target %+v, want %+v", *got[0].Target, want)
			}
		})
	}
}

func TestAStudentWhoseOverrideHasNotClosedHasNoResultReady(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{release: "after_close", opens: at(-3 * day), closes: at(-hour)})
	w.attempt(assignment, w.student, "graded", at(-2*hour))
	w.override(assignment, w.student, moment(day), 0)
	if got := ofKind(w.due(w.student), domain.ResultReady); len(got) != 0 {
		t.Errorf("a student whose own close is tomorrow is told their result is ready: %+v", got)
	}
}

func TestOnlyAnEnabledStudentIsAddressed(t *testing.T) {
	w := newDueWorld(t)
	teacher, disabled := w.user("teacher"), w.user("student")
	w.join(w.class, teacher)
	w.join(w.class, disabled)
	w.disabled(disabled)
	w.assignment(spec{opens: at(-time.Hour), closes: at(hour)})
	if got := w.due(teacher); len(ofKind(got, domain.AssignmentOpened))+len(ofKind(got, domain.AssignmentDueSoon)) != 0 {
		t.Errorf("a teacher in the class is reminded as a student: %+v", got)
	}
	if got := w.due(disabled); len(got) != 0 {
		t.Errorf("a disabled student has %+v", got)
	}
	if got := w.due(w.student); len(got) == 0 {
		t.Error("the enabled student has nothing, so the checks above prove nothing")
	}
}

func TestAStudentNamedOnAnAssignmentIsAddressedWithoutAClass(t *testing.T) {
	w := newDueWorld(t)
	named := w.user("student")
	w.assignment(spec{classes: []string{}, students: []string{named}, opens: at(-time.Hour), closes: at(5 * day)})
	if got := ofKind(w.due(named), domain.AssignmentOpened); len(got) != 1 {
		t.Errorf("a named student has %d opened notices, want one", len(got))
	}
	if got := w.due(w.student); len(got) != 0 {
		t.Errorf("a classmate who is not named has %+v", got)
	}
}
