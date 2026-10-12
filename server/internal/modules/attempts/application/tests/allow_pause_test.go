//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"testing"
)

func audioPauseRule(t *testing.T, questions []domain.Question, id string) bool {
	t.Helper()
	for _, q := range questions {
		if q.ID == id {
			if q.Audio == nil {
				t.Fatalf("question %s lost its audio policy", id)
			}
			return q.Audio.AllowPause
		}
	}
	t.Fatalf("question %s is not on the paper", id)
	return false
}

func TestThePaperAndTheReviewCarryThePauseRuleFrozenWithTheQuestion(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	ctx := context.Background()
	service := newService(t, pool)
	reviews := repositories.NewReviews(db.NewContext(pool))

	start := func() domain.Session {
		t.Helper()
		session, err := service.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
		if err != nil {
			t.Fatalf("start: %v", err)
		}
		return session
	}
	reviewed := func(attempt string) bool {
		t.Helper()
		review, err := reviews.Get(ctx, everyone, attempt)
		if err != nil {
			t.Fatalf("review: %v", err)
		}
		for _, q := range review.Questions {
			if q.ID == w.listening {
				if q.Audio == nil {
					t.Fatal("the review lost the audio policy")
				}
				return q.Audio.AllowPause
			}
		}
		t.Fatal("the review lacks the listening question")
		return false
	}

	session := start()
	if !audioPauseRule(t, session.Questions, w.listening) || !reviewed(session.Attempt.ID) {
		t.Fatal("a question frozen without a pause rule reads as one that cannot be paused")
	}

	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET audio_allow_pause = false WHERE id = $1::uuid`, w.listening); err != nil {
		t.Fatal(err)
	}
	session = start()
	if audioPauseRule(t, session.Questions, w.listening) || reviewed(session.Attempt.ID) {
		t.Fatal("a question frozen with pausing off reads as one that can be paused")
	}

	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET audio_allow_pause = true WHERE id = $1::uuid`, w.listening); err != nil {
		t.Fatal(err)
	}
	session = start()
	if !audioPauseRule(t, session.Questions, w.listening) || !reviewed(session.Attempt.ID) {
		t.Fatal("a question frozen with pausing on reads as one that cannot be paused")
	}
}
