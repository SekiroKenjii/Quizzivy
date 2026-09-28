package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/stats"
)

type guardedShelf struct {
	domain.Students
	visible map[string]bool
	writes  []string
}

func (g *guardedShelf) Get(_ context.Context, _ access.Scope, id string) (domain.Student, error) {
	if !g.visible[id] {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	return domain.Student{ID: id}, nil
}

func (g *guardedShelf) Update(_ context.Context, _ domain.WriteRequest, in domain.StudentPatch) (domain.Student, error) {
	g.writes = append(g.writes, "update")
	return domain.Student{ID: in.ID}, nil
}

func (g *guardedShelf) Delete(context.Context, domain.WriteRequest, string, time.Time) error {
	g.writes = append(g.writes, "delete")
	return nil
}

func (g *guardedShelf) ResetPassword(context.Context, domain.WriteRequest, string, string, time.Time) error {
	g.writes = append(g.writes, "reset")
	return nil
}

type noFigures struct{}

func (noFigures) StudentStats(context.Context, access.Scope, []string) (map[string]stats.Student, error) {
	return map[string]stats.Student{}, nil
}

type heldPermissions struct {
	sets map[string]access.Set
}

func (h *heldPermissions) Forget(string) {}

func (h *heldPermissions) Resolve(_ context.Context, id string) (access.Principal, error) {
	set, ok := h.sets[id]
	if !ok {
		return access.Principal{}, accessdomain.ErrUnknownUser
	}
	return access.Principal{UserID: id, Permissions: set}, nil
}

func guarded() (*application.Application, *guardedShelf) {
	shelf := &guardedShelf{visible: map[string]bool{"pupil": true, "grader": true, "gone": true}}
	held := &heldPermissions{sets: map[string]access.Set{
		"pupil":  access.NewSet(access.LearningTakeTests),
		"grader": access.NewSet(access.LearningTakeTests, access.TeachingGrading),
	}}
	app := application.New(nil, nil, 0, shelf, noFigures{})
	app.SetPrincipals(held)
	return app, shelf
}

func TestTheSubsetRuleAndAccountManagementGuardEveryStudentWrite(t *testing.T) {
	manager := domain.WriteRequest{ActorID: "manager", Grants: access.NewSet(access.PeopleUsersManage, access.PeopleStudentsResetPassword)}
	teacher := domain.WriteRequest{ActorID: "teacher", Grants: access.NewSet(access.PeopleStudentsCreate, access.PeopleStudentsResetPassword, access.TeachingGrading)}
	everyone := domain.WriteRequest{ActorID: "admin", All: true, Grants: access.NewSet(access.All()...)}
	yes := true
	name := "Tên mới"
	for label, c := range map[string]struct {
		run    func(*application.Application) error
		want   error
		writes int
	}{
		"a Teacher disabling": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: teacher, Input: domain.StudentPatch{ID: "pupil", Disabled: &yes}})
			return err
		}, domain.ErrForbidden, 0},
		"a manager disabling a student who grades": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: manager, Input: domain.StudentPatch{ID: "grader", Disabled: &yes}})
			return err
		}, domain.ErrForbidden, 0},
		"a manager disabling a student": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: manager, Input: domain.StudentPatch{ID: "pupil", Disabled: &yes}})
			return err
		}, nil, 1},
		"a Teacher renaming a student": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: teacher, Input: domain.StudentPatch{ID: "pupil", FullName: &name}})
			return err
		}, nil, 1},
		"a Teacher renaming a student who holds a key the Teacher lacks": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: domain.WriteRequest{ActorID: "teacher", Grants: access.NewSet(access.PeopleStudentsCreate)}, Input: domain.StudentPatch{ID: "grader", FullName: &name}})
			return err
		}, domain.ErrForbidden, 0},
		"anyone disabling an unreached student": {func(app *application.Application) error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: everyone, Input: domain.StudentPatch{ID: "stranger", Disabled: &yes}})
			return err
		}, domain.ErrStudentNotFound, 0},
		"a manager deleting a student who grades": {func(app *application.Application) error {
			_, err := app.Commands.DeleteStudent.Handle(context.Background(), command.DeleteStudent{Request: manager, ID: "grader"})
			return err
		}, domain.ErrForbidden, 0},
		"the Admin deleting a student who grades": {func(app *application.Application) error {
			_, err := app.Commands.DeleteStudent.Handle(context.Background(), command.DeleteStudent{Request: everyone, ID: "grader"})
			return err
		}, nil, 1},
		"a Teacher resetting a student who holds a key the Teacher lacks": {func(app *application.Application) error {
			_, err := app.Commands.ResetStudentPassword.Handle(context.Background(), command.ResetStudentPassword{Request: domain.WriteRequest{ActorID: "teacher", Grants: access.NewSet(access.PeopleStudentsResetPassword)}, ID: "grader"})
			return err
		}, domain.ErrForbidden, 0},
		"a Teacher resetting a student": {func(app *application.Application) error {
			_, err := app.Commands.ResetStudentPassword.Handle(context.Background(), command.ResetStudentPassword{Request: teacher, ID: "pupil"})
			return err
		}, nil, 1},
		"deleting an account the resolver no longer knows": {func(app *application.Application) error {
			_, err := app.Commands.DeleteStudent.Handle(context.Background(), command.DeleteStudent{Request: everyone, ID: "gone"})
			return err
		}, domain.ErrStudentNotFound, 0},
	} {
		t.Run(label, func(t *testing.T) {
			app, shelf := guarded()
			err := c.run(app)
			if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
				t.Errorf("answered %v, want %v", err, c.want)
			}
			if len(shelf.writes) != c.writes {
				t.Errorf("wrote %v, want %d writes", shelf.writes, c.writes)
			}
		})
	}
}
