package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

type composingRepository struct {
	domain.Repository
	created  domain.CreateInput
	updated  domain.UpdateInput
	enrolled domain.EnrolInput
	writes   int
}

func (r *composingRepository) Create(_ context.Context, in domain.CreateInput) (domain.Class, error) {
	r.writes++
	r.created = in
	return domain.Class{}, nil
}

func (r *composingRepository) Update(_ context.Context, _ access.Scope, _ string, in domain.UpdateInput) (domain.Class, error) {
	r.writes++
	r.updated = in
	return domain.Class{}, nil
}

func (r *composingRepository) Enrol(_ context.Context, in domain.EnrolInput) (domain.EnrolResult, error) {
	r.writes++
	r.enrolled = in
	return domain.EnrolResult{}, nil
}

func decomposedText(s string) string { return norm.NFD.String(s) }

func newComposingApplication(repo *composingRepository) *application.Application {
	keys, err := domain.NewJoinCodeKeys([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		panic(err)
	}
	return application.New(repo, nil, keys)
}

func TestCreateStoresTheNameAndDescriptionComposed(t *testing.T) {
	repo := &composingRepository{}
	description := decomposedText("Lớp buổi tối")
	_, err := newComposingApplication(repo).Commands.Create.Handle(context.Background(), command.Create{Name: decomposedText("Lớp 10A"), Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.Name != "Lớp 10A" || *repo.created.Description != "Lớp buổi tối" {
		t.Fatalf("created=%+v", repo.created)
	}
}

func TestUpdateStoresTheNameAndDescriptionComposedAndKeepsWhatWasNotSent(t *testing.T) {
	repo := &composingRepository{}
	name := decomposedText("Lớp 10B")
	selfJoin := true
	_, err := newComposingApplication(repo).Commands.Update.Handle(context.Background(), command.Update{ClassID: "c", Input: domain.UpdateInput{Name: &name, SelfJoinEnabled: &selfJoin}})
	if err != nil {
		t.Fatal(err)
	}
	if *repo.updated.Name != "Lớp 10B" || repo.updated.Description != nil || repo.updated.SelfJoinEnabled == nil || !*repo.updated.SelfJoinEnabled {
		t.Fatalf("updated=%+v", repo.updated)
	}
}

func TestANameComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	name := strings.Repeat("क़", domain.MaxClassName)
	repo := &composingRepository{}
	app := newComposingApplication(repo)
	for label, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.Create.Handle(context.Background(), command.Create{Name: name})
			return err
		},
		"update": func() error {
			_, err := app.Commands.Update.Handle(context.Background(), command.Update{ClassID: "c", Input: domain.UpdateInput{Name: &name}})
			return err
		},
	} {
		t.Run(label, func(t *testing.T) {
			var invalid *validation.Error
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "name" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if repo.writes != 0 {
		t.Fatalf("a refused name reached the repository %d times", repo.writes)
	}
}

func TestASignUpFromAProviderStoresTheFullNameComposedAndTheIdentityUntouched(t *testing.T) {
	repo := &composingRepository{}
	member := domain.NewMember{Email: "Hoc.Sinh@Example.com", FullName: decomposedText("Trần Thị B"), Provider: "google", ProviderUserID: "google-sub-0123"}
	if _, err := newComposingApplication(repo).Commands.EnrolNewMember.Handle(context.Background(), command.EnrolNewMember{Member: member, Code: "ABCD2345"}); err != nil {
		t.Fatal(err)
	}
	got := repo.enrolled.NewMember
	if got == nil || got.FullName != "Trần Thị B" || got.Email != "Hoc.Sinh@Example.com" || got.Provider != "google" || got.ProviderUserID != "google-sub-0123" {
		t.Fatalf("enrolled=%+v", got)
	}
	if member.FullName != decomposedText("Trần Thị B") {
		t.Fatal("the command's member was rewritten")
	}
}
