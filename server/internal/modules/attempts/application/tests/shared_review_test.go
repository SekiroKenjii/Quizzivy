//go:build integration

package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"

	"github.com/google/uuid"
)

func TestSharedReviewContextSeparatesReleasedAndTeacherTranscripts(t *testing.T) {
	pool := newPool(t)
	svc, w, session, first, second := sharedAudioAttempt(t, pool)
	ctx := context.Background()
	for _, item := range []struct {
		id, transcript string
		released       bool
	}{
		{first, "Released group transcript", true}, {second, "Private group transcript", false},
	} {
		if _, err := pool.Exec(ctx, `UPDATE app.test_version_group_recordings SET transcript=$2,show_transcript_after_submit=$3 WHERE id=$1`, item.id, item.transcript, item.released); err != nil {
			t.Fatal(err)
		}
	}
	session, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.student})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: domain.GroupPlayInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID, RecordingID: first, PlayID: uuid.NewString(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.student}); !errors.Is(err, domain.ErrAttemptInProgress) {
		t.Fatalf("live attempt exposed result: %v", err)
	}
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	for mask := range 8 {
		setReview(t, pool, w, mask&1 != 0, mask&2 != 0, mask&4 != 0)
		result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.student})
		if err != nil {
			t.Fatal(err)
		}
		if result.SharedContext == nil || !reflect.DeepEqual(result.SharedContext.Groups, session.Groups) {
			t.Fatal("review policy dropped the frozen materials")
		}
		if !reflect.DeepEqual(result.SharedContext.Transcripts, map[string]string{first: "Released group transcript"}) {
			t.Fatalf("policy %d leaked/withheld shared transcript: %+v", mask, result.SharedContext.Transcripts)
		}
		if result.SharedContext.AudioPlays[first] != 1 || result.SharedContext.AudioPlays[second] != 0 {
			t.Fatal("result merged shared recording counts")
		}
	}
	if _, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.outsider}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("outsider result: %v", err)
	}
	dbx := db.NewContext(pool)
	papers := testsapp.New(testsrepo.NewPostgres(dbx, nil, nil))
	full := application.New(repositories.NewTimelines(dbx), repositories.NewReviews(dbx), repositories.NewPostgres(dbx)).WithGroupContexts(papers.Queries.GroupContexts)
	review, err := full.Queries.Review.Handle(ctx, query.Review{AttemptID: session.Attempt.ID})
	if err != nil || review.SharedContext == nil || len(review.SharedContext.Transcripts) != 2 || review.SharedContext.Transcripts[second] != "Private group transcript" {
		t.Fatalf("teacher lost shared transcript: %+v, %v", review.SharedContext, err)
	}
	if review.SharedContext.AudioPlays[first] != 1 {
		t.Fatal("teacher lost shared listening count")
	}
	byQ, err := full.Queries.AnswersForQuestion.Handle(ctx, query.AnswersForQuestion{AssignmentID: w.assignment, QuestionID: w.essay})
	if err != nil || byQ.SharedContext == nil {
		t.Fatalf("cross-attempt context: %v", err)
	}
	if len(byQ.SharedContext.Groups) != 1 || len(byQ.SharedContext.Transcripts) != 1 || byQ.SharedContext.Transcripts[second] != "Private group transcript" || byQ.SharedContext.AudioPlays != nil {
		t.Fatalf("cross-attempt context includes unrelated groups/counts: %+v", byQ.SharedContext)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_group_recordings SET show_transcript_after_submit=false WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.student})
	if err != nil || result.SharedContext == nil || len(result.SharedContext.Transcripts) != 0 {
		t.Fatalf("score/key/explanation flags overrode transcript policy: %+v, %v", result.SharedContext, err)
	}
	missing := application.New(nil, repositories.NewReviews(dbx), repositories.NewPostgres(dbx))
	if _, err := missing.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.student}); !errors.Is(err, domain.ErrGroupContextUnavailable) {
		t.Fatalf("missing result reader: %v", err)
	}
	if _, err := missing.Queries.Review.Handle(ctx, query.Review{AttemptID: session.Attempt.ID}); !errors.Is(err, domain.ErrGroupContextUnavailable) {
		t.Fatalf("missing teacher reader: %v", err)
	}
}
