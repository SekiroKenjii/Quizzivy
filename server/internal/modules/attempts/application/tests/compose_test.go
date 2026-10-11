package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

type composingStore struct {
	domain.Repository
	saved    domain.SaveInput
	reasons  map[string]string
	flagged  string
	saveCall int
}

func (s *composingStore) Save(_ context.Context, in domain.SaveInput, _ time.Time) (domain.SaveResult, domain.Milestones, error) {
	s.saveCall++
	s.saved = in
	return domain.SaveResult{}, domain.Milestones{}, nil
}

func (s *composingStore) record(kind, reason string) (domain.Attempt, error) {
	if s.reasons == nil {
		s.reasons = map[string]string{}
	}
	s.reasons[kind] = reason
	return domain.Attempt{}, nil
}

func (s *composingStore) Extend(_ context.Context, _ domain.Request, _ string, _ int, reason string, _ time.Time) (domain.Attempt, error) {
	return s.record("extend", reason)
}

func (s *composingStore) Void(_ context.Context, _ domain.Request, _, reason string, _ time.Time) (domain.Attempt, error) {
	return s.record("void", reason)
}

func (s *composingStore) Reset(_ context.Context, _ domain.Request, _, reason string, _ time.Time) (domain.Attempt, error) {
	return s.record("reset", reason)
}

func (s *composingStore) Flag(_ context.Context, _ domain.Request, _ string, _ bool, reason string, _ time.Time) (domain.Attempt, error) {
	s.flagged = reason
	return domain.Attempt{}, nil
}

type composingReviews struct {
	domain.ReviewRepository
	note   *string
	items  []domain.GradeItem
	writes int
}

func (r *composingReviews) SetNote(_ context.Context, _ access.Scope, _ string, note *string) error {
	r.writes++
	r.note = note
	return nil
}

func (r *composingReviews) Grade(_ context.Context, _ access.Scope, _, _ string, items []domain.GradeItem) (domain.Score, error) {
	r.writes++
	r.items = items
	return domain.Score{}, nil
}

func newComposing() (*application.Application, *composingStore, *composingReviews) {
	store, reviews := &composingStore{}, &composingReviews{}
	return application.New(nil, reviews, store), store, reviews
}

func answerPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSaveStoresTheTypedTextOfAnswersComposedAndNothingElse(t *testing.T) {
	app, store, _ := newComposing()
	blank, option := "0195aaaa-0000-7000-8000-000000000001", "0195aaaa-0000-7000-8000-000000000002"
	input := domain.SaveInput{AttemptID: "attempt", StudentID: "student", SessionID: "session", Answers: []domain.Answer{
		{QuestionID: "q-text", Payload: answerPayload(t, map[string]any{"type": "text", "value": norm.NFD.String("Hà Nội")})},
		{QuestionID: "q-fill", Payload: answerPayload(t, map[string]any{"type": "fill_blank", "values": map[string]any{blank: norm.NFD.String("Huế")}})},
		{QuestionID: "q-choice", Payload: answerPayload(t, map[string]any{"type": "choice", "optionIds": []any{option}})},
	}, Events: []domain.Event{{Kind: "focus_lost"}}}
	before := string(input.Answers[0].Payload)
	if _, err := app.Commands.Save.Handle(context.Background(), command.Save{Input: input}); err != nil {
		t.Fatal(err)
	}
	got := store.saved
	if got.AttemptID != "attempt" || got.StudentID != "student" || got.SessionID != "session" || len(got.Events) != 1 || got.Events[0].Kind != "focus_lost" {
		t.Fatalf("saved=%+v", got)
	}
	byQuestion := map[string]map[string]any{}
	for _, answer := range got.Answers {
		var payload map[string]any
		if err := json.Unmarshal(answer.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		byQuestion[answer.QuestionID] = payload
	}
	if byQuestion["q-text"]["value"] != "Hà Nội" {
		t.Fatalf("text=%v", byQuestion["q-text"])
	}
	if byQuestion["q-fill"]["values"].(map[string]any)[blank] != "Huế" {
		t.Fatalf("fill=%v", byQuestion["q-fill"])
	}
	if ids := byQuestion["q-choice"]["optionIds"].([]any); len(ids) != 1 || ids[0] != option {
		t.Fatalf("choice=%v", byQuestion["q-choice"])
	}
	if string(input.Answers[0].Payload) != before {
		t.Fatal("the caller's answers were rewritten")
	}
}

func TestSaveOfNoAnswersStaysNoAnswers(t *testing.T) {
	app, store, _ := newComposing()
	if _, err := app.Commands.Save.Handle(context.Background(), command.Save{Input: domain.SaveInput{AttemptID: "attempt"}}); err != nil {
		t.Fatal(err)
	}
	if store.saved.Answers != nil {
		t.Fatalf("answers=%v", store.saved.Answers)
	}
}

func TestEveryReasonAnInterventionRecordsIsComposed(t *testing.T) {
	app, store, _ := newComposing()
	reason := norm.NFD.String("Máy bị hỏng")
	ctx := context.Background()
	if _, err := app.Commands.Extend.Handle(ctx, command.Extend{AttemptID: "a", Minutes: 5, Reason: reason}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Void.Handle(ctx, command.Void{AttemptID: "a", Reason: reason}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Reset.Handle(ctx, command.Reset{AttemptID: "a", Reason: reason}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Flag.Handle(ctx, command.Flag{AttemptID: "a", Flagged: true, Reason: reason}); err != nil {
		t.Fatal(err)
	}
	for kind, got := range map[string]string{"extend": store.reasons["extend"], "void": store.reasons["void"], "reset": store.reasons["reset"], "flag": store.flagged} {
		if got != "Máy bị hỏng" {
			t.Errorf("%s reason=%q", kind, got)
		}
	}
}

func TestANoteIsStoredComposedAndAClearedOneStaysCleared(t *testing.T) {
	app, _, reviews := newComposing()
	note := norm.NFD.String("Bài làm tốt")
	if _, err := app.Commands.SetNote.Handle(context.Background(), command.SetNote{AttemptID: "a", Note: &note}); err != nil {
		t.Fatal(err)
	}
	if reviews.note == nil || *reviews.note != "Bài làm tốt" {
		t.Fatalf("note=%v", reviews.note)
	}
	if _, err := app.Commands.SetNote.Handle(context.Background(), command.SetNote{AttemptID: "a"}); err != nil {
		t.Fatal(err)
	}
	if reviews.note != nil {
		t.Fatalf("a cleared note became %q", *reviews.note)
	}
}

func TestANoteComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	app, _, reviews := newComposing()
	note := strings.Repeat("क़", domain.MaxTeacherNote)
	_, err := app.Commands.SetNote.Handle(context.Background(), command.SetNote{AttemptID: "a", Note: &note})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "note" || reviews.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, reviews.writes)
	}
}

func TestGradeCommentsAreStoredComposedAndPointsAreNot(t *testing.T) {
	app, _, reviews := newComposing()
	comment := norm.NFD.String("Trả lời đúng")
	items := []domain.GradeItem{{QuestionID: "q-1", Points: 1.5, Comment: &comment}, {QuestionID: "q-2", Points: 0}}
	if _, err := app.Commands.Grade.Handle(context.Background(), command.Grade{AttemptID: "a", GraderID: "g", Items: items}); err != nil {
		t.Fatal(err)
	}
	if len(reviews.items) != 2 || *reviews.items[0].Comment != "Trả lời đúng" || reviews.items[0].Points != 1.5 || reviews.items[0].QuestionID != "q-1" || reviews.items[1].Comment != nil {
		t.Fatalf("items=%+v", reviews.items)
	}
	if *items[0].Comment != norm.NFD.String("Trả lời đúng") {
		t.Fatal("the caller's comment was rewritten")
	}
}
