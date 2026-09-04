package attempts_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

func setReview(t *testing.T, pool *pgxpool.Pool, w world, score, correct, explanations bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		UPDATE app.assignments
		   SET review_show_score = $2, review_show_correct_answers = $3, review_show_explanations = $4
		 WHERE id = $1::uuid`, w.assignment, score, correct, explanations); err != nil {
		t.Fatal(err)
	}
}

func submitted(t *testing.T, pool *pgxpool.Pool) (*attempts.Service, world, attempts.Session) {
	t.Helper()
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	if _, err := svc.Submit(context.Background(), session.Attempt.ID, w.student, attempts.Manual); err != nil {
		t.Fatal(err)
	}
	return svc, w, session
}

func byID(r attempts.Result) map[string]attempts.ResultQuestion {
	out := map[string]attempts.ResultQuestion{}
	for _, q := range r.Questions {
		out[q.ID] = q
	}
	return out
}

// Three flags, eight policies. Each flag withholds exactly one thing and
// leaves the rest alone (S-09b).
func TestEachReviewFlagReleasesExactlyItsOwnBlock(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	ctx := context.Background()

	for _, tc := range []struct{ score, correct, explanations bool }{
		{false, false, false}, {true, false, false}, {false, true, false}, {false, false, true},
		{true, true, false}, {true, false, true}, {false, true, true}, {true, true, true},
	} {
		setReview(t, pool, w, tc.score, tc.correct, tc.explanations)
		result, err := svc.Result(ctx, session.Attempt.ID, w.student)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		qs := byID(result)
		choice, blank, essay := qs[w.choice], qs[w.blank], qs[w.essay]

		if (result.Score != nil) != tc.score {
			t.Errorf("%+v: score present=%v", tc, result.Score != nil)
		}
		if (choice.Earned != nil) != tc.score {
			t.Errorf("%+v: choice.earned present=%v", tc, choice.Earned != nil)
		}
		if essay.Earned != nil || !essay.PendingManual {
			t.Errorf("%+v: the essay is pending, so it has no earned value whatever the policy", tc)
		}
		if (choice.CorrectOptions != nil) != tc.correct {
			t.Errorf("%+v: correctOptions present=%v", tc, choice.CorrectOptions != nil)
		}
		if (blank.CorrectAnswers != nil) != tc.correct {
			t.Errorf("%+v: correctAnswers present=%v", tc, blank.CorrectAnswers != nil)
		}
		if (choice.Explanation != nil) != tc.explanations {
			t.Errorf("%+v: explanation present=%v", tc, choice.Explanation != nil)
		}
		if tc.score && (result.Score.Earned != 15 || result.Score.Total != 10 || result.Score.PendingManual != 1) {
			t.Errorf("%+v: score %+v", tc, *result.Score)
		}
	}
}

func TestTheSampleAnswerNeverReachesTheResultEvenWithEverythingOn(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	setReview(t, pool, w, true, true, true)

	result, err := svc.Result(context.Background(), session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secretSampleAnswer) {
		t.Error("the sample answer is in the result")
	}
	if !strings.Contains(string(encoded), secretExplanation) {
		t.Error("with showExplanations on, the explanation should be released")
	}
	// One canonical answer per blank is released, and the fixture's blanks
	// each hold exactly one, so the same string is both.
	if !strings.Contains(string(encoded), secretBlankAnswer) {
		t.Error("with showCorrectAnswers on, the canonical blank answer should be released")
	}
}

func TestTheTranscriptFollowsTheQuestionsOwnFlagAndNothingElse(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	ctx := context.Background()

	setReview(t, pool, w, false, false, false)
	result, err := svc.Result(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if q := byID(result)[w.listening]; q.Transcript == nil || *q.Transcript != secretTranscript {
		t.Errorf("transcript withheld although the question releases it after submit")
	}

	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET audio_show_transcript_after = false WHERE id = $1::uuid`, w.listening); err != nil {
		t.Fatal(err)
	}
	setReview(t, pool, w, true, true, true)
	result, err = svc.Result(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if q := byID(result)[w.listening]; q.Transcript != nil {
		t.Errorf("transcript released although the question's flag is off; the review policy has no say")
	}
}

func TestAResultIsRefusedWhileTheAttemptIsLiveAndForbiddenToAnyoneElse(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := svc.Result(ctx, session.Attempt.ID, w.student); !errors.Is(err, attempts.ErrAttemptInProgress) {
		t.Errorf("live attempt: %v, want ErrAttemptInProgress", err)
	}
	if _, err := svc.Submit(ctx, session.Attempt.ID, w.student, attempts.Manual); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Result(ctx, session.Attempt.ID, w.outsider); !errors.Is(err, attempts.ErrForbidden) {
		t.Errorf("outsider: %v, want ErrForbidden", err)
	}
	if _, err := svc.Void(ctx, teacher(w), session.Attempt.ID, "huỷ"); err != nil {
		t.Fatal(err)
	}
	auditRows(t, pool, session.Attempt.ID, "attempt.voided")
	if _, err := svc.Result(ctx, session.Attempt.ID, w.student); !errors.Is(err, attempts.ErrAttemptVoided) {
		t.Errorf("voided: %v, want ErrAttemptVoided", err)
	}
}
