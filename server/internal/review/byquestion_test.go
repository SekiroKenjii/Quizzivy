package review_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/review"
)

func TestOneQuestionIsReadAcrossEveryHandedInPaper(t *testing.T) {
	pool := newPool(t)
	store := review.NewStore(pool)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()

	var assignmentID string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignmentID); err != nil {
		t.Fatal(err)
	}

	byQ, err := store.AnswersForQuestion(ctx, assignmentID, p.essay)
	if err != nil {
		t.Fatalf("answers for question: %v", err)
	}
	if byQ.Number != 2 || byQ.Count != 2 {
		t.Errorf("the essay is question %d of %d, want 2 of 2", byQ.Number, byQ.Count)
	}
	if len(byQ.ManualIDs) != 1 || byQ.ManualIDs[0] != p.essay {
		t.Errorf("manual questions %v, want just the essay", byQ.ManualIDs)
	}
	if byQ.Question.SampleAnswer == nil || *byQ.Question.SampleAnswer != "I get up at six." {
		t.Errorf("the rubric side lost the sample answer: %v", byQ.Question.SampleAnswer)
	}
	if len(byQ.Items) != 1 {
		t.Fatalf("%d papers, want 1", len(byQ.Items))
	}
	item := byQ.Items[0]
	if item.AttemptID != p.attempt || item.StudentName != "Nguyễn Đức Minh" || item.AttemptNo != 1 {
		t.Errorf("row %+v does not name the paper", item)
	}
	if string(item.Payload) == "" || item.ManualScore != nil {
		t.Errorf("row %+v should carry the unmarked answer", item)
	}

	if _, err := store.AnswersForQuestion(ctx, assignmentID, p.choice); err != nil {
		t.Errorf("a non-manual question on the paper is still readable: %v", err)
	}
	if _, err := store.AnswersForQuestion(ctx, assignmentID, "00000000-0000-7000-8000-000000000000"); !errors.Is(err, review.ErrQuestionNotOnPaper) {
		t.Errorf("a question off the paper: %v, want ErrQuestionNotOnPaper", err)
	}
	if _, err := store.AnswersForQuestion(ctx, "00000000-0000-7000-8000-000000000000", p.essay); !errors.Is(err, review.ErrNotFound) {
		t.Errorf("an unknown assignment: %v, want ErrNotFound", err)
	}
}
