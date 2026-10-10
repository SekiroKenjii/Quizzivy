package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/domain"

	"golang.org/x/text/unicode/norm"
)

type composingRepository struct {
	domain.Repository
	created   domain.WriteInput
	updated   domain.WriteInput
	overrides domain.OverrideInput
	writes    int
}

func (r *composingRepository) Create(_ context.Context, _ domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	r.writes++
	r.created = in
	return domain.Assignment{}, nil
}

func (r *composingRepository) Update(_ context.Context, _ domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	r.writes++
	r.updated = in
	return domain.Assignment{}, nil
}

func (r *composingRepository) SetOverrides(_ context.Context, _ domain.Request, in domain.OverrideInput) ([]domain.StudentOverride, error) {
	r.writes++
	r.overrides = in
	return nil, nil
}

func TestAnAssignmentIsStoredWithItsNoteForStudentsComposed(t *testing.T) {
	repo := &composingRepository{}
	app := application.New(repo)
	note := norm.NFD.String("Làm bài cẩn thận")
	if _, err := app.Commands.Create.Handle(context.Background(), command.Create{Input: domain.WriteInput{StudentNote: &note}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Update.Handle(context.Background(), command.Update{Input: domain.WriteInput{StudentNote: &note, StudentNoteSet: true}}); err != nil {
		t.Fatal(err)
	}
	if *repo.created.StudentNote != "Làm bài cẩn thận" || *repo.updated.StudentNote != "Làm bài cẩn thận" || !repo.updated.StudentNoteSet {
		t.Fatalf("created=%v updated=%v", *repo.created.StudentNote, *repo.updated.StudentNote)
	}
	if note != norm.NFD.String("Làm bài cẩn thận") {
		t.Fatal("the caller's note was rewritten")
	}
}

func TestAnAssignmentWithoutANoteKeepsItAbsent(t *testing.T) {
	repo := &composingRepository{}
	if _, err := application.New(repo).Commands.Create.Handle(context.Background(), command.Create{}); err != nil {
		t.Fatal(err)
	}
	if repo.created.StudentNote != nil {
		t.Fatalf("a note appeared: %q", *repo.created.StudentNote)
	}
}

func TestAnOverrideIsStoredWithItsReasonComposed(t *testing.T) {
	repo := &composingRepository{}
	extend := 15
	in := domain.OverrideInput{StudentIDs: []string{"student-id"}, ExtendBy: &extend, Reason: norm.NFD.String("Máy bị hỏng"), Notify: true}
	if _, err := application.New(repo).Commands.SetOverrides.Handle(context.Background(), command.SetOverrides{Input: in}); err != nil {
		t.Fatal(err)
	}
	got := repo.overrides
	if got.Reason != "Máy bị hỏng" || got.StudentIDs[0] != "student-id" || *got.ExtendBy != 15 || !got.Notify {
		t.Fatalf("override=%+v", got)
	}
}

func TestAReasonComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	repo := &composingRepository{}
	reason := strings.Repeat("क़", domain.MaxOverrideReason)
	_, err := application.New(repo).Commands.SetOverrides.Handle(context.Background(), command.SetOverrides{Input: domain.OverrideInput{Reason: reason}})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "reason" || repo.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, repo.writes)
	}
}
