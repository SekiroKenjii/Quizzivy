package application_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/shared/cqrs"
)

const liveCode = "ABCD2345"

var typedForms = []string{liveCode, "ABCD-2345", "abcd-2345", "  abcd2345  "}

type liveCodeRepository struct {
	domain.Repository
	lookups atomic.Int32
	enrols  atomic.Int32
}

func (r *liveCodeRepository) isLive(code domain.JoinCodeLookup) bool {
	legacy := domain.JoinCodes.Hash(liveCode)
	return slices.ContainsFunc(code.Hashes(), func(hash []byte) bool { return bytes.Equal(hash, legacy) })
}

func (r *liveCodeRepository) LookupByCode(_ context.Context, code domain.JoinCodeLookup) (*domain.CodeRow, error) {
	r.lookups.Add(1)
	if !r.isLive(code) {
		return nil, nil
	}
	teacher := "Cô Lan"
	return &domain.CodeRow{
		CodeState:   domain.CodeState{SelfJoinEnabled: true, ExpiresAt: time.Now().Add(time.Hour)},
		ClassID:     "class-1",
		ClassName:   "Lớp 10A",
		Lookup:      domain.StoredLookup{Scheme: domain.LookupLegacy, Hash: domain.JoinCodes.Hash(liveCode)},
		TeacherName: &teacher,
	}, nil
}

func (r *liveCodeRepository) Enrol(_ context.Context, in domain.EnrolInput) (domain.EnrolResult, error) {
	r.enrols.Add(1)
	if !r.isLive(in.Code) {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}
	return domain.EnrolResult{
		Outcome:     domain.PreviewOK,
		UserID:      "user-1",
		Class:       domain.EnrolledClass{ID: "class-1", Name: "Lớp 10A"},
		StudentName: "Trần Thị B",
		TeacherID:   "teacher-1",
	}, nil
}

func (r *liveCodeRepository) touched() int32 { return r.lookups.Load() + r.enrols.Load() }

func joiningPolicy(on bool) ports.SignInPolicy {
	return cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return organizationmodel.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: on}, nil
		})
}

func switchableJoining(on *atomic.Bool) ports.SignInPolicy {
	return cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return organizationmodel.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: on.Load()}, nil
		})
}

func unreadableJoining(err error) ports.SignInPolicy {
	return cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return organizationmodel.SignInPolicy{}, err
		})
}

func joiningApp(repo domain.Repository, policy ports.SignInPolicy) *application.Application {
	keys, err := domain.NewJoinCodeKeys([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		panic(err)
	}
	return application.New(repo, nil, keys).WithSignInPolicy(policy)
}

func previewing(t *testing.T, app *application.Application, code string) domain.PreviewResult {
	t.Helper()
	result, err := app.Queries.Preview.Handle(context.Background(), query.Preview{Code: code})
	if err != nil {
		t.Fatalf("Preview(%q): %v", code, err)
	}
	return result
}

func newMemberEnrolment(t *testing.T, app *application.Application, code string) domain.EnrolResult {
	t.Helper()
	result, err := app.Commands.EnrolNewMember.Handle(context.Background(), command.EnrolNewMember{
		Member: domain.NewMember{Email: "moi@example.com", FullName: "Trần Thị B", Provider: "google", ProviderUserID: "google-sub-1"},
		Code:   code,
		Meta:   domain.Meta{IP: "203.0.113.44", UserAgent: "go-test"},
	})
	if err != nil {
		t.Fatalf("EnrolNewMember(%q): %v", code, err)
	}
	return result
}

func existingMemberEnrolment(t *testing.T, app *application.Application, code string) domain.EnrolResult {
	t.Helper()
	result, err := app.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{
		UserID: "user-1",
		Code:   code,
		Meta:   domain.Meta{IP: "203.0.113.44", UserAgent: "go-test"},
	})
	if err != nil {
		t.Fatalf("EnrolExisting(%q): %v", code, err)
	}
	return result
}

type joinPath struct {
	name string
	run  func(*testing.T, *application.Application, string) any
	ok   func(any) bool
}

var joinPaths = []joinPath{
	{
		"the preview",
		func(t *testing.T, app *application.Application, code string) any { return previewing(t, app, code) },
		func(result any) bool { return result.(domain.PreviewResult).Outcome == domain.PreviewOK },
	},
	{
		"an enrolment of a new member",
		func(t *testing.T, app *application.Application, code string) any {
			return newMemberEnrolment(t, app, code)
		},
		func(result any) bool { return result.(domain.EnrolResult).Outcome == domain.PreviewOK },
	},
	{
		"an enrolment of an existing member",
		func(t *testing.T, app *application.Application, code string) any {
			return existingMemberEnrolment(t, app, code)
		},
		func(result any) bool { return result.(domain.EnrolResult).Outcome == domain.PreviewOK },
	},
}

func TestJoiningOffAnswersAValidCodeAsAnUnknownOneOnEveryPathWithoutTouchingTheRepository(t *testing.T) {
	for _, path := range joinPaths {
		t.Run(path.name, func(t *testing.T) {
			on := joiningApp(&liveCodeRepository{}, joiningPolicy(true))
			if valid := path.run(t, on, liveCode); !path.ok(valid) {
				t.Fatalf("with joining on, the live code answered %+v", valid)
			}
			unknown := path.run(t, on, "ZZZZ-ZZZZ")
			if path.ok(unknown) {
				t.Fatalf("with joining on, an unknown code answered %+v", unknown)
			}

			repo := &liveCodeRepository{}
			off := joiningApp(repo, joiningPolicy(false))
			for _, typed := range slices.Concat(typedForms, []string{"ZZZZ-ZZZZ", "", "!!!"}) {
				if refused := path.run(t, off, typed); !reflect.DeepEqual(refused, unknown) {
					t.Errorf("with joining off, %q answered %+v, want what an unknown code answers, %+v", typed, refused, unknown)
				}
			}
			if touched := repo.touched(); touched != 0 {
				t.Errorf("the repository was touched %d times", touched)
			}
		})
	}
}

func TestJoiningSwitchedOffAndBackOnFollowsTheCentreOnTheNextRequest(t *testing.T) {
	var on atomic.Bool
	on.Store(true)
	app := joiningApp(&liveCodeRepository{}, switchableJoining(&on))

	for _, path := range joinPaths {
		on.Store(true)
		if !path.ok(path.run(t, app, liveCode)) {
			t.Fatalf("%s with joining on refused the live code", path.name)
		}
		on.Store(false)
		if path.ok(path.run(t, app, liveCode)) {
			t.Errorf("%s with joining off took the live code", path.name)
		}
		on.Store(true)
		if !path.ok(path.run(t, app, liveCode)) {
			t.Errorf("%s with joining back on refused the live code", path.name)
		}
	}
}

func TestWithNoPolicyPortJoiningIsOn(t *testing.T) {
	app := joiningApp(&liveCodeRepository{}, nil)

	for _, path := range joinPaths {
		if !path.ok(path.run(t, app, liveCode)) {
			t.Errorf("%s refused the live code", path.name)
		}
	}
}

func TestAPolicyThatCannotBeReadFailsEveryJoinPathAsItselfAndTouchesNoCode(t *testing.T) {
	unreadable := errors.New("the settings snapshot could not be read")
	repo := &liveCodeRepository{}
	app := joiningApp(repo, unreadableJoining(unreadable))
	ctx := context.Background()

	if _, err := app.Queries.Preview.Handle(ctx, query.Preview{Code: liveCode}); !errors.Is(err, unreadable) {
		t.Errorf("Preview: error = %v, want the failure to read the policy", err)
	}
	if _, err := app.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Code: liveCode}); !errors.Is(err, unreadable) {
		t.Errorf("EnrolNewMember: error = %v, want the failure to read the policy", err)
	}
	if _, err := app.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: "user-1", Code: liveCode}); !errors.Is(err, unreadable) {
		t.Errorf("EnrolExisting: error = %v, want the failure to read the policy", err)
	}
	if touched := repo.touched(); touched != 0 {
		t.Errorf("the repository was touched %d times", touched)
	}
}
