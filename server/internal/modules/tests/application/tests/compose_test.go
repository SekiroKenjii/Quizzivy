package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"

	"golang.org/x/text/unicode/norm"
)

type composeRepository struct {
	domain.Repository
	created   domain.CreateInput
	updated   domain.UpdateRequest
	published domain.PublishRequest
	writes    int
}

func (r *composeRepository) Create(_ context.Context, in domain.CreateInput) (domain.Test, error) {
	r.writes++
	r.created = in
	return domain.Test{}, nil
}

func (r *composeRepository) Update(_ context.Context, in domain.UpdateRequest) (domain.Test, error) {
	r.writes++
	r.updated = in
	return domain.Test{}, nil
}

func (r *composeRepository) Publish(_ context.Context, in domain.PublishRequest, _ time.Time, _ func(domain.DraftContent) error) (domain.Version, error) {
	r.writes++
	r.published = in
	return domain.Version{}, nil
}

type composeGroups struct {
	domain.GroupRepository
	created domain.CreateGroupInput
	updated domain.UpdateGroupInput
	writes  int
}

func (r *composeGroups) Create(_ context.Context, in domain.CreateGroupInput) (domain.StoredGroup, error) {
	r.writes++
	r.created = in
	return domain.StoredGroup{}, nil
}

func (r *composeGroups) Update(_ context.Context, in domain.UpdateGroupInput) (domain.StoredGroup, error) {
	r.writes++
	r.updated = in
	return domain.StoredGroup{}, nil
}

func nfdText(s string) string { return norm.NFD.String(s) }

func TestCreateStoresTheTitleAndDescriptionComposed(t *testing.T) {
	repo := &composeRepository{}
	description := nfdText("Mô tả đề")
	_, err := application.New(repo).Commands.Create.Handle(context.Background(), command.Create{Title: nfdText("Đề kiểm tra"), Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.Title != "Đề kiểm tra" || *repo.created.Description != "Mô tả đề" {
		t.Fatalf("created=%+v", repo.created)
	}
	if repo.created.Title != norm.NFC.String(repo.created.Title) {
		t.Fatal("the title is still decomposed")
	}
}

func TestCreateRefusesATitleComposingLeavesOverItsLimitBeforeAnyWrite(t *testing.T) {
	repo := &composeRepository{}
	_, err := application.New(repo).Commands.Create.Handle(context.Background(), command.Create{Title: strings.Repeat("क़", domain.MaxTestTitle)})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "title" || repo.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, repo.writes)
	}
}

func TestUpdateStoresTheOutlineComposed(t *testing.T) {
	repo := &composeRepository{}
	title := nfdText("Đề kiểm tra")
	_, err := application.New(repo).Commands.Update.Handle(context.Background(), command.Update{Input: domain.UpdateInput{
		Title: &title, SetSections: true, Sections: []domain.SectionInput{{Title: nfdText("Phần nghe")}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if *repo.updated.Input.Title != "Đề kiểm tra" || repo.updated.Input.Sections[0].Title != "Phần nghe" {
		t.Fatalf("updated=%+v", repo.updated.Input)
	}
}

func TestUpdateRefusesATitleComposingLeavesOverItsLimitBeforeAnyWrite(t *testing.T) {
	repo := &composeRepository{}
	title := strings.Repeat("क़", domain.MaxTestTitle)
	_, err := application.New(repo).Commands.Update.Handle(context.Background(), command.Update{Input: domain.UpdateInput{Title: &title}})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "title" || repo.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, repo.writes)
	}
}

func TestPublishStoresTheChangeNoteComposedAndTrimmed(t *testing.T) {
	repo := &composeRepository{}
	note := "  " + nfdText("Sửa đề") + "  "
	_, err := application.New(repo).Commands.Publish.Handle(context.Background(), command.Publish{Request: domain.PublishRequest{ChangeNote: &note}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.published.ChangeNote == nil || *repo.published.ChangeNote != "Sửa đề" {
		t.Fatalf("note=%v", repo.published.ChangeNote)
	}
}

func TestPublishRefusesANoteComposingLeavesOverItsLimitBeforeAnyWrite(t *testing.T) {
	repo := &composeRepository{}
	note := strings.Repeat("क़", domain.MaxChangeNote)
	_, err := application.New(repo).Commands.Publish.Handle(context.Background(), command.Publish{Request: domain.PublishRequest{ChangeNote: &note}})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "changeNote" || repo.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, repo.writes)
	}
}

func groupBundle() domain.GroupBundle {
	return domain.GroupBundle{
		Group:     domain.QuestionGroup{ID: "01935000-0000-7000-8000-0000000000b1", Title: nfdText("Nhóm đọc hiểu")},
		Questions: []domain.GroupQuestion{},
	}
}

func TestGroupCommandsStoreTheBundleComposed(t *testing.T) {
	repo := &composeGroups{}
	app := application.New(&composeRepository{}).WithGroups(repo, nil)
	if _, err := app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{Bundle: groupBundle()}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.UpdateGroup.Handle(context.Background(), command.UpdateGroup{Mutation: model.GroupMutation{ID: "g"}, Bundle: groupBundle()}); err != nil {
		t.Fatal(err)
	}
	if repo.created.Bundle.Group.Title != "Nhóm đọc hiểu" || repo.updated.Bundle.Group.Title != "Nhóm đọc hiểu" {
		t.Fatalf("created=%q updated=%q", repo.created.Bundle.Group.Title, repo.updated.Bundle.Group.Title)
	}
}

func TestGroupCommandsRefuseAMemberComposingLeavesOverALimitBeforeAnyWrite(t *testing.T) {
	repo := &composeGroups{}
	app := application.New(&composeRepository{}).WithGroups(repo, nil)
	alt := strings.Repeat("क़", questions.MaxMediaAltLength)
	bundle := groupBundle()
	bundle.Questions = []domain.GroupQuestion{{ID: "q-1", Input: questions.Input{Type: questions.ShortAnswer, Prompt: "Mô tả", Points: "1", MediaAlt: &alt}}}
	for name, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{Bundle: bundle})
			return err
		},
		"update": func() error {
			_, err := app.Commands.UpdateGroup.Handle(context.Background(), command.UpdateGroup{Mutation: model.GroupMutation{ID: "g"}, Bundle: bundle})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *domain.GroupError
			if err := run(); !errors.As(err, &invalid) || invalid.Rule != "group_content" || invalid.QuestionID != "q-1" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if repo.writes != 0 {
		t.Fatalf("a refused group reached the repository %d times", repo.writes)
	}
}
