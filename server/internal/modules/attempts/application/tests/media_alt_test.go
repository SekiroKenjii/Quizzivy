//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/modules/attempts/application/command"
	"testing"
)

func TestThePaperCarriesTheAltTextFrozenWithItsQuestion(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	alt := "Một chú mèo ngồi trên ghế"
	if _, err := pool.Exec(context.Background(), `UPDATE app.test_version_questions SET media_alt = $2 WHERE id = $1`, w.choice, alt); err != nil {
		t.Fatal(err)
	}

	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	carrying := 0
	for _, q := range session.Questions {
		if q.MediaAlt == nil {
			continue
		}
		carrying++
		if q.ID != w.choice || *q.MediaAlt != alt {
			t.Errorf("question %s carries alt text %q, want only %s with %q", q.ID, *q.MediaAlt, w.choice, alt)
		}
	}
	if carrying != 1 {
		t.Errorf("%d questions carry alt text, want 1 of %d", carrying, len(session.Questions))
	}
}
