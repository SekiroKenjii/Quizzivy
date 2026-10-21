package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

type issuingShelf struct {
	guardedShelf
	mu      sync.Mutex
	created []domain.WriteRequest
	resets  map[string]domain.WriteRequest
}

func (s *issuingShelf) Create(_ context.Context, req domain.WriteRequest, _ domain.NewStudent) (domain.Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, req)
	return domain.Student{ID: "created"}, nil
}

func (s *issuingShelf) ResetPassword(_ context.Context, req domain.WriteRequest, id, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resets[id] = req
	return nil
}

func issuingApp(policy ports.SignInPolicy) (*application.Application, *issuingShelf) {
	shelf := &issuingShelf{
		guardedShelf: guardedShelf{visible: map[string]bool{"first": true, "second": true}},
		resets:       map[string]domain.WriteRequest{},
	}
	held := &heldPermissions{sets: map[string]access.Set{
		"first":  access.NewSet(access.LearningTakeTests),
		"second": access.NewSet(access.LearningTakeTests),
	}}
	app := application.New(nil, nil, 0, shelf, noFigures{})
	app.SetPrincipals(held)
	app.SetSignInPolicy(policy)
	app.SetTemporaryPasswords(func(context.Context) (string, string, error) { return "temporary", "hash", nil })
	return app, shelf
}

func newStudent() domain.NewStudent {
	return domain.NewStudent{Email: "moi@example.com", FullName: "Vũ Minh Khôi"}
}

func TestEveryTemporaryPasswordIsIssuedForcedOrKeptAsTheCentreDecides(t *testing.T) {
	for name, c := range map[string]struct {
		policy ports.SignInPolicy
		keep   bool
	}{
		"with no policy port":         {nil, false},
		"with the change forced":      {policyOf(openPolicy()), false},
		"with the change not forced":  {policyOf(policyWith(keepTemporary)), true},
		"with every other way in off": {policyOf(policyWith(passwordOff, googleOff)), false},
	} {
		t.Run(name, func(t *testing.T) {
			app, shelf := issuingApp(c.policy)
			ctx := context.Background()

			if _, err := app.Commands.CreateStudent.Handle(ctx, command.CreateStudent{Request: writer(), Input: newStudent()}); err != nil {
				t.Fatalf("CreateStudent: %v", err)
			}
			if _, err := app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: writer(), ID: "first"}); err != nil {
				t.Fatalf("ResetStudentPassword: %v", err)
			}
			bulk, err := app.Commands.ResetStudentsPasswords.Handle(ctx, command.ResetStudentsPasswords{Request: writer(), IDs: []string{"first", "second"}})
			if err != nil || len(bulk.Reset) != 2 {
				t.Fatalf("ResetStudentsPasswords reset %d (failed %d): %v", len(bulk.Reset), len(bulk.Failed), err)
			}

			if len(shelf.created) != 1 || shelf.created[0].KeepTemporaryPassword != c.keep {
				t.Errorf("CreateStudent wrote with %+v, want keep = %v", shelf.created, c.keep)
			}
			for id, req := range shelf.resets {
				if req.KeepTemporaryPassword != c.keep {
					t.Errorf("a reset of %s wrote with keep = %v, want %v", id, req.KeepTemporaryPassword, c.keep)
				}
			}
			if len(shelf.resets) != 2 {
				t.Errorf("%d students were reset, want 2", len(shelf.resets))
			}
		})
	}
}

func TestACallerCannotAskForATemporaryPasswordToBeKeptWhileTheChangeIsForced(t *testing.T) {
	app, shelf := issuingApp(policyOf(openPolicy()))
	request := writer()
	request.KeepTemporaryPassword = true
	ctx := context.Background()

	if _, err := app.Commands.CreateStudent.Handle(ctx, command.CreateStudent{Request: request, Input: newStudent()}); err != nil {
		t.Fatalf("CreateStudent: %v", err)
	}
	if _, err := app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: request, ID: "first"}); err != nil {
		t.Fatalf("ResetStudentPassword: %v", err)
	}
	if _, err := app.Commands.ResetStudentsPasswords.Handle(ctx, command.ResetStudentsPasswords{Request: request, IDs: []string{"second"}}); err != nil {
		t.Fatalf("ResetStudentsPasswords: %v", err)
	}

	if shelf.created[0].KeepTemporaryPassword {
		t.Error("CreateStudent kept the temporary password the policy forces a change of")
	}
	for id, req := range shelf.resets {
		if req.KeepTemporaryPassword {
			t.Errorf("a reset of %s kept the temporary password the policy forces a change of", id)
		}
	}
}

func TestABulkResetStampsTheSameChoiceOnEveryStudentAndItsBatch(t *testing.T) {
	app, shelf := issuingApp(policyOf(policyWith(keepTemporary)))

	if _, err := app.Commands.ResetStudentsPasswords.Handle(context.Background(),
		command.ResetStudentsPasswords{Request: writer(), IDs: []string{"first", "second"}}); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"first", "second"} {
		req := shelf.resets[id]
		if !req.KeepTemporaryPassword || req.BatchID == "" || req.BatchID != shelf.resets["first"].BatchID {
			t.Errorf("%s was reset with %+v, want the one batch and keep = true", id, req)
		}
	}
}

func TestAPolicyThatCannotBeReadIssuesNoTemporaryPassword(t *testing.T) {
	unreadable := errors.New("the settings snapshot could not be read")
	app, shelf := issuingApp(unreadablePolicy(unreadable))
	ctx := context.Background()

	if _, err := app.Commands.CreateStudent.Handle(ctx, command.CreateStudent{Request: writer(), Input: newStudent()}); !errors.Is(err, unreadable) {
		t.Errorf("CreateStudent: error = %v, want the failure to read the policy", err)
	}
	if _, err := app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: writer(), ID: "first"}); !errors.Is(err, unreadable) {
		t.Errorf("ResetStudentPassword: error = %v, want the failure to read the policy", err)
	}
	bulk, err := app.Commands.ResetStudentsPasswords.Handle(ctx, command.ResetStudentsPasswords{Request: writer(), IDs: []string{"first", "second"}})
	if !errors.Is(err, unreadable) || len(bulk.Reset) != 0 || len(bulk.Failed) != 0 {
		t.Errorf("ResetStudentsPasswords: error = %v, reset %d, failed %d; want the failure to read the policy and nothing else", err, len(bulk.Reset), len(bulk.Failed))
	}

	if len(shelf.created) != 0 || len(shelf.resets) != 0 {
		t.Errorf("the shelf was written to: %d creates, %d resets", len(shelf.created), len(shelf.resets))
	}
}
