//go:build integration

package application_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	classesapp "quizzivy/internal/modules/classes/application"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
)

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func removeAccountsAt(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		owned := `SELECT id FROM app.users WHERE lower(email) = lower($1)`
		for _, stmt := range []string{
			`DELETE FROM app.refresh_tokens WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.audit_log WHERE actor_user_id IN (` + owned + `)`,
			`DELETE FROM app.class_members WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.user_identities WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.users WHERE lower(email) = lower($1)`,
		} {
			if _, err := pool.Exec(ctx, stmt, email); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
}

func refreshFamilies(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1::uuid`, userID)
}

func identities(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM app.user_identities WHERE user_id = $1::uuid`, userID)
}

func TestPasswordSignInOffRefusesARealAccountAndStoresNoSession(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	current := openPolicy()
	svc.SetSignInPolicy(switchablePolicy(func() organizationmodel.SignInPolicy { return current }))
	id, email := makeUser(t, pool)
	disabledID, disabledEmail := makeUser(t, pool, disabled)
	ctx := context.Background()
	attempts := map[string]command.Login{
		"the right password": {Email: email, Password: testPassword},
		"a wrong password":   {Email: email, Password: "sai-mật-khẩu"},
		"an unknown email":   {Email: "nobody-" + nonce(t) + "@example.com", Password: testPassword},
		"a disabled account": {Email: disabledEmail, Password: testPassword},
	}

	current = policyWith(passwordOff)
	for name, attempt := range attempts {
		if _, err := svc.Commands.Login.Handle(ctx, attempt); !errors.Is(err, domain.ErrPasswordSignInDisabled) {
			t.Errorf("%s: error = %v, want ErrPasswordSignInDisabled", name, err)
		}
	}
	if n := refreshFamilies(t, pool, id) + refreshFamilies(t, pool, disabledID); n != 0 {
		t.Fatalf("the refusals stored %d sessions", n)
	}

	current = openPolicy()
	session, err := svc.Commands.Login.Handle(ctx, attempts["the right password"])
	if err != nil {
		t.Fatalf("with password sign-in back on: %v", err)
	}
	if session.User.ID != id || refreshFamilies(t, pool, id) != 1 {
		t.Errorf("signed in as %s with %d sessions stored", session.User.ID, refreshFamilies(t, pool, id))
	}
	if _, err := svc.Commands.Login.Handle(ctx, attempts["a wrong password"]); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("a wrong password with password sign-in on: error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Commands.Login.Handle(ctx, attempts["a disabled account"]); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("a disabled account with password sign-in on: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestUnlinkingGoogleWithPasswordSignInOffKeepsTheLinkOfARealAccount(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	linkGoogleSubject(t, pool, id, "google-sub-"+email, email)
	current := policyWith(passwordOff)
	svc.SetSignInPolicy(switchablePolicy(func() organizationmodel.SignInPolicy { return current }))
	unlinkAs := command.UnlinkGoogle{Actor: actor.Actor{ID: id, IP: "203.0.113.31", UserAgent: "go-test"}}

	if _, err := svc.Commands.UnlinkGoogle.Handle(context.Background(), unlinkAs); !errors.Is(err, domain.ErrLastLoginMethod) {
		t.Fatalf("with password sign-in off: error = %v, want ErrLastLoginMethod", err)
	}
	if n := identities(t, pool, id); n != 1 {
		t.Fatalf("the refusal left %d identities, want 1", n)
	}

	current = openPolicy()
	if _, err := svc.Commands.UnlinkGoogle.Handle(context.Background(), unlinkAs); err != nil {
		t.Fatalf("with password sign-in on: %v", err)
	}
	if n := identities(t, pool, id); n != 0 {
		t.Errorf("%d identities remain after the unlink", n)
	}
}

func TestGoogleSignInOffRefusesARealLinkedAccountBeforeGoogleIsCalled(t *testing.T) {
	pool := newPool(t)
	linkedID, linkedEmail := makeUser(t, pool, googleOnly)
	identity := verifiedIdentity(linkedEmail)
	linkGoogleSubject(t, pool, linkedID, identity.Subject, linkedEmail)
	otherID, _ := makeUser(t, pool)
	svc, stub := googleService(t, pool, identity)
	svc.SetSignInPolicy(policyOf(policyWith(googleOff)))

	if _, err := signIn(svc, ""); !errors.Is(err, domain.ErrGoogleSignInDisabled) {
		t.Fatalf("sign-in: error = %v, want ErrGoogleSignInDisabled", err)
	}
	if _, err := link(svc, otherID); !errors.Is(err, domain.ErrGoogleSignInDisabled) {
		t.Fatalf("linking: error = %v, want ErrGoogleSignInDisabled", err)
	}

	if stub.exchanged {
		t.Error("Google was called")
	}
	if n := refreshFamilies(t, pool, linkedID); n != 0 {
		t.Errorf("the refusal stored %d sessions", n)
	}
	if n := identities(t, pool, otherID); n != 0 {
		t.Errorf("the refusal linked %d identities", n)
	}
}

func TestAnAllowedDomainListKeepsARealOutsideAddressFromSigningInOrLinking(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	svc, _ := googleService(t, pool, verifiedIdentity(email))
	svc.SetSignInPolicy(policyOf(policyWith(onlyDomains("school.edu"))))

	if _, err := signIn(svc, ""); !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
		t.Fatalf("a sign-in that would link by email: error = %v, want ErrGoogleDomainNotAllowed", err)
	}
	if _, err := link(svc, id); !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
		t.Fatalf("linking: error = %v, want ErrGoogleDomainNotAllowed", err)
	}

	if n := identities(t, pool, id); n != 0 {
		t.Errorf("the refusals linked %d identities", n)
	}
	if n := refreshFamilies(t, pool, id); n != 0 {
		t.Errorf("the refusals stored %d sessions", n)
	}
}

func TestAnAllowedDomainListAdmitsARealAddressInsideItInAnyCase(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	local, _, _ := strings.Cut(email, "@")
	capitals := local + "@EXAMPLE.COM"
	svc, _ := googleService(t, pool, verifiedIdentity(capitals))
	svc.SetSignInPolicy(policyOf(policyWith(onlyDomains("example.com"))))

	result, err := signIn(svc, "")
	if err != nil {
		t.Fatalf("a sign-in that links by email: %v", err)
	}
	if result.Session.User.ID != id || identities(t, pool, id) != 1 {
		t.Errorf("signed in as %s with %d identities linked", result.Session.User.ID, identities(t, pool, id))
	}
}

type joiningService struct {
	svc       *application.Application
	classID   string
	teacherID string
}

func newJoiningService(t *testing.T, pool *pgxpool.Pool, newcomer string, centre organizationmodel.SignInPolicy) joiningService {
	t.Helper()
	classID, teacherID := makeClassForEnrol(t, pool)
	removeAccountsAt(t, pool, newcomer)
	classes := classesapp.New(classesrepo.NewPostgres(db.NewContext(pool)), nil, joinKeys).WithSignInPolicy(policyOf(centre))
	svc := newService(t, pool)
	svc.SetGoogle(&stubGoogle{identity: verifiedIdentity(newcomer)}, classes.Commands.EnrolNewMember)
	svc.SetSignInPolicy(policyOf(centre))
	return joiningService{svc: svc, classID: classID, teacherID: teacherID}
}

func (j joiningService) members(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM app.class_members WHERE class_id = $1::uuid`, j.classID)
}

func (j joiningService) uses(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return count(t, pool, `SELECT coalesce(sum(uses_count), 0)::int FROM app.class_join_codes WHERE class_id = $1::uuid`, j.classID)
}

func TestAJoinCodeOutsideTheAllowedDomainsCreatesNoAccountAndSpendsNoUse(t *testing.T) {
	pool := newPool(t)
	email := "newcomer-" + nonce(t) + "@example.com"
	j := newJoiningService(t, pool, email, policyWith(onlyDomains("school.edu")))
	code := issueJoinCode(t, pool, j.classID, j.teacherID)

	if _, err := signIn(j.svc, code); !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
		t.Fatalf("error = %v, want ErrGoogleDomainNotAllowed", err)
	}

	if n := count(t, pool, `SELECT count(*) FROM app.users WHERE lower(email) = lower($1)`, email); n != 0 {
		t.Errorf("the refusal created %d accounts", n)
	}
	if members, uses := j.members(t, pool), j.uses(t, pool); members != 0 || uses != 0 {
		t.Errorf("the refusal enrolled %d members and spent %d uses", members, uses)
	}
}

func TestAJoinCodeInsideTheAllowedDomainsCreatesTheAccountWhateverTheCase(t *testing.T) {
	pool := newPool(t)
	email := "Newcomer-" + nonce(t) + "@EXAMPLE.com"
	j := newJoiningService(t, pool, email, policyWith(onlyDomains("example.com")))
	code := issueJoinCode(t, pool, j.classID, j.teacherID)

	result, err := signIn(j.svc, code)
	if err != nil {
		t.Fatalf("sign-in with a join code: %v", err)
	}
	if result.EnrolledClass == nil || result.EnrolledClass.ID != j.classID {
		t.Fatalf("enrolled class = %+v, want %s", result.EnrolledClass, j.classID)
	}
	if members, uses := j.members(t, pool), j.uses(t, pool); members != 1 || uses != 1 {
		t.Errorf("enrolled %d members and spent %d uses, want 1 and 1", members, uses)
	}
}

func TestJoiningOffAnswersAValidCodeThroughGoogleAsAnUnknownOne(t *testing.T) {
	pool := newPool(t)
	email := "newcomer-" + nonce(t) + "@example.com"
	j := newJoiningService(t, pool, email, policyWith(func(p *organizationmodel.SignInPolicy) { p.JoinCodes = false }))
	code := issueJoinCode(t, pool, j.classID, j.teacherID)

	_, refused := signIn(j.svc, code)
	_, unknown := signIn(j.svc, "ZZZZ-ZZZZ")

	var rejected domain.JoinCodeRejected
	if !errors.As(refused, &rejected) || rejected.Outcome != classesdomain.PreviewInvalid {
		t.Fatalf("a valid code: error = %v, want a rejection as an unknown code", refused)
	}
	if !reflect.DeepEqual(refused, unknown) {
		t.Errorf("a valid code answered %#v, an unknown code %#v", refused, unknown)
	}
	if n := count(t, pool, `SELECT count(*) FROM app.users WHERE lower(email) = lower($1)`, email); n != 0 {
		t.Errorf("the refusal created %d accounts", n)
	}
	if members, uses := j.members(t, pool), j.uses(t, pool); members != 0 || uses != 0 {
		t.Errorf("the refusal enrolled %d members and spent %d uses", members, uses)
	}
}

func TestJoiningOnStillTakesAValidCodeThroughGoogle(t *testing.T) {
	pool := newPool(t)
	email := "newcomer-" + nonce(t) + "@example.com"
	j := newJoiningService(t, pool, email, openPolicy())
	code := issueJoinCode(t, pool, j.classID, j.teacherID)

	result, err := signIn(j.svc, code)
	if err != nil {
		t.Fatalf("sign-in with a join code: %v", err)
	}
	if result.EnrolledClass == nil || j.members(t, pool) != 1 || j.uses(t, pool) != 1 {
		t.Errorf("enrolled class = %+v, members = %d, uses = %d", result.EnrolledClass, j.members(t, pool), j.uses(t, pool))
	}
}
