package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

type composingShelf struct {
	guardedShelf
	created domain.NewStudent
	patched domain.StudentPatch
}

func (c *composingShelf) Create(_ context.Context, _ domain.WriteRequest, in domain.NewStudent) (domain.Student, error) {
	c.writes = append(c.writes, "create")
	c.created = in
	return domain.Student{}, nil
}

func (c *composingShelf) Update(_ context.Context, _ domain.WriteRequest, in domain.StudentPatch) (domain.Student, error) {
	c.writes = append(c.writes, "update")
	c.patched = in
	return domain.Student{ID: in.ID}, nil
}

func composingStudents() (*application.Application, *composingShelf) {
	shelf := &composingShelf{guardedShelf: guardedShelf{visible: map[string]bool{"pupil": true}}}
	held := &heldPermissions{sets: map[string]access.Set{"pupil": access.NewSet(access.LearningTakeTests)}}
	app := application.New(nil, nil, 0, shelf, noFigures{})
	app.SetPrincipals(held)
	return app, shelf
}

func writer() domain.WriteRequest {
	return domain.WriteRequest{ActorID: "teacher", All: true, Grants: access.NewSet(access.All()...)}
}

func TestACreatedStudentIsStoredWithTheNameComposedAndTheEmailAsGiven(t *testing.T) {
	app, shelf := composingStudents()
	_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{
		Email: norm.NFD.String("Hoc.Sinh.é") + "@Example.com", FullName: norm.NFD.String("Nguyễn Văn Á"), ClassIDs: []string{"class-id"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := shelf.created
	if got.FullName != "Nguyễn Văn Á" || got.Email != norm.NFD.String("Hoc.Sinh.é")+"@Example.com" || got.ClassIDs[0] != "class-id" || got.Hash == "" {
		t.Fatalf("created=%+v", got)
	}
}

func TestAnUpdatedStudentIsStoredWithTheNameComposedAndTheEmailAsGiven(t *testing.T) {
	app, shelf := composingStudents()
	name, email := norm.NFD.String("Trần Thị B"), norm.NFD.String("Mới")+"@Example.com"
	if _, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: domain.StudentPatch{ID: "pupil", FullName: &name, Email: &email}}); err != nil {
		t.Fatal(err)
	}
	got := shelf.patched
	if *got.FullName != "Trần Thị B" || *got.Email != norm.NFD.String("Mới")+"@Example.com" || got.ID != "pupil" {
		t.Fatalf("patched=%+v", got)
	}
}

func TestAStudentNameComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	name := strings.Repeat("क़", domain.MaxFullNameLength)
	app, shelf := composingStudents()
	for label, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{Email: "a@example.com", FullName: name}})
			return err
		},
		"update": func() error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: domain.StudentPatch{ID: "pupil", FullName: &name}})
			return err
		},
	} {
		t.Run(label, func(t *testing.T) {
			var invalid *validation.Error
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "fullName" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if len(shelf.writes) != 0 {
		t.Fatalf("a refused name reached the repository: %v", shelf.writes)
	}
}

func TestAProfileStoresItsNamesComposed(t *testing.T) {
	repo := &profileUsers{user: domain.User{ID: "actor"}}
	app := application.New(repo, nil, time.Hour, nil, nil)
	patch := domain.ProfilePatch{FullName: profileName(norm.NFD.String("Nguyễn An")), DisplayNameSet: true, DisplayName: profileName(norm.NFD.String("Cô An")), PhoneSet: true, Phone: profileName("+84 123456")}
	if _, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: patch}); err != nil {
		t.Fatal(err)
	}
	got := repo.got.Patch
	if *got.FullName != "Nguyễn An" || *got.DisplayName != "Cô An" || *got.Phone != "+84 123456" {
		t.Fatalf("patch=%+v", got)
	}
}

func TestAProfileNameComposingLeavesOverItsLimitIsRefusedByTheNamedError(t *testing.T) {
	for label, patch := range map[string]domain.ProfilePatch{
		"full name":    {FullName: profileName(strings.Repeat("क़", 101))},
		"display name": {DisplayNameSet: true, DisplayName: profileName(strings.Repeat("क़", 41))},
	} {
		t.Run(label, func(t *testing.T) {
			repo := &profileUsers{user: domain.User{ID: "actor"}}
			_, err := application.New(repo, nil, time.Hour, nil, nil).Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: patch})
			if !errors.Is(err, domain.ErrNameTooLong) && !errors.Is(err, domain.ErrDisplayNameInvalid) {
				t.Fatalf("err=%v", err)
			}
			if repo.calls != 0 {
				t.Fatal("a refused profile reached the repository")
			}
		})
	}
}
