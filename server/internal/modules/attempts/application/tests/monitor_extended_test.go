//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/modules/attempts/application/query"
	"testing"
	"time"
)

func TestTheMonitorSaysWhereAStudentsOwnCloseIsOnlyWhenItReachesPastTheAssignments(t *testing.T) {
	pool := newPool(t)
	early := time.Now().Add(-time.Minute)
	o := openAssignment()
	o.closedAt = &early
	w := seedWorld(t, pool, o)
	svc := newService(t, pool)
	people := enrol(t, pool, w, 3)
	reopened, sooner, plain := people[0], people[1], people[2]
	until := later(2 * time.Hour)
	giveOverride(t, pool, w, reopened, until, nil, 0)
	giveOverride(t, pool, w, sooner, later(-time.Hour), nil, 1)

	monitor, err := svc.Queries.Monitor.Handle(context.Background(), query.Monitor{AssignmentID: w.assignment, Scope: everyone})
	if err != nil {
		t.Fatal(err)
	}
	extended := map[string]*time.Time{}
	for _, row := range monitor.Rows {
		extended[row.StudentID] = row.ExtendedTo
	}
	if got := extended[reopened]; got == nil || !got.Equal(*until) {
		t.Errorf("the student reopened by an override past the early close reads %v, want %v", got, *until)
	}
	for name, student := range map[string]string{"one whose override closes before the assignment": sooner, "one with no override": plain} {
		if got := extended[student]; got != nil {
			t.Errorf("%s reads extended to %v, want nothing", name, *got)
		}
	}
}

func TestTheMonitorMarksAnOverridePastTheWindowOfAnAssignmentStillOpen(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	people := enrol(t, pool, w, 2)
	until := later(5 * time.Hour)
	giveOverride(t, pool, w, people[0], until, nil, 0)

	monitor, err := svc.Queries.Monitor.Handle(context.Background(), query.Monitor{AssignmentID: w.assignment, Scope: everyone})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range monitor.Rows {
		switch {
		case row.StudentID == people[0] && (row.ExtendedTo == nil || !row.ExtendedTo.Equal(*until)):
			t.Errorf("the student with a later close reads %v, want %v", row.ExtendedTo, *until)
		case row.StudentID == people[1] && row.ExtendedTo != nil:
			t.Errorf("a student without one reads extended to %v", *row.ExtendedTo)
		}
	}
}
