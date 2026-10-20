package application_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/cqrs"
)

const policyPassword = "mật-khẩu-của-trung-tâm"

var storedPolicyPassword = sync.OnceValue(func() string {
	hash, err := domain.Passwords.Hash(context.Background(), policyPassword)
	if err != nil {
		panic(err)
	}
	return hash
})

func openPolicy() organizationmodel.SignInPolicy {
	return organizationmodel.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true}
}

func policyWith(changes ...func(*organizationmodel.SignInPolicy)) organizationmodel.SignInPolicy {
	policy := openPolicy()
	for _, change := range changes {
		change(&policy)
	}
	return policy
}

func passwordOff(p *organizationmodel.SignInPolicy)   { p.Password = false }
func googleOff(p *organizationmodel.SignInPolicy)     { p.Google = false }
func keepTemporary(p *organizationmodel.SignInPolicy) { p.ForcePasswordChange = false }

func onlyDomains(domains ...string) func(*organizationmodel.SignInPolicy) {
	return func(p *organizationmodel.SignInPolicy) { p.AllowedGoogleDomains = domains }
}

func switchablePolicy(read func() organizationmodel.SignInPolicy) ports.SignInPolicy {
	return cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return read(), nil
		})
}

func policyOf(policy organizationmodel.SignInPolicy) ports.SignInPolicy {
	return switchablePolicy(func() organizationmodel.SignInPolicy { return policy })
}

func unreadablePolicy(err error) ports.SignInPolicy {
	return cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return organizationmodel.SignInPolicy{}, err
		})
}

type policyStore struct {
	domain.Users
	calls      []string
	byID       map[string]domain.User
	byEmail    map[string]domain.User
	byIdentity map[string]domain.User
	unlinked   int
	linked     int
}

func newPolicyStore() *policyStore {
	return &policyStore{
		byID:       map[string]domain.User{},
		byEmail:    map[string]domain.User{},
		byIdentity: map[string]domain.User{},
	}
}

func (s *policyStore) add(user domain.User, googleSubject string) *policyStore {
	s.byID[user.ID] = user
	s.byEmail[strings.ToLower(user.Email)] = user
	if googleSubject != "" {
		s.byIdentity[googleSubject] = user
	}
	return s
}

func (s *policyStore) called(name string) bool { return slices.Contains(s.calls, name) }

func (s *policyStore) wrote() bool {
	return s.called("CreateRefreshToken") || s.called("LinkIdentity") || s.called("UnlinkIdentity") || s.called("WriteAudit")
}

func (s *policyStore) FindUserByEmail(_ context.Context, email string) (domain.User, error) {
	s.calls = append(s.calls, "FindUserByEmail")
	user, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *policyStore) FindUserByID(_ context.Context, id string) (domain.User, error) {
	s.calls = append(s.calls, "FindUserByID")
	user, ok := s.byID[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *policyStore) FindUserByProviderIdentity(_ context.Context, _, subject string) (domain.User, error) {
	s.calls = append(s.calls, "FindUserByProviderIdentity")
	user, ok := s.byIdentity[subject]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *policyStore) CreateRefreshToken(context.Context, domain.RefreshTokenRecord, domain.SessionBasis) error {
	s.calls = append(s.calls, "CreateRefreshToken")
	return nil
}

func (s *policyStore) StampSignIn(context.Context, string, domain.SignInMethod, time.Time) error {
	s.calls = append(s.calls, "StampSignIn")
	return nil
}

func (s *policyStore) LinkIdentity(context.Context, string, string, string, string) error {
	s.calls = append(s.calls, "LinkIdentity")
	s.linked++
	return nil
}

func (s *policyStore) UnlinkIdentity(context.Context, string, string) (bool, error) {
	s.calls = append(s.calls, "UnlinkIdentity")
	s.unlinked++
	return true, nil
}

func (s *policyStore) WriteAudit(context.Context, audit.Entry) error {
	s.calls = append(s.calls, "WriteAudit")
	return nil
}

func policyUser(id, email string) domain.User {
	hash := storedPolicyPassword()
	return domain.User{ID: id, Email: email, FullName: "Nguyễn Văn An", PasswordHash: &hash}
}

func withoutPassword(user domain.User) domain.User {
	user.PasswordHash = nil
	return user
}

func suspended(user domain.User) domain.User {
	now := time.Now()
	user.DisabledAt = &now
	return user
}

func policyApp(t *testing.T, store domain.Users, policy ports.SignInPolicy) *application.Application {
	t.Helper()
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store, issuer, 30*24*time.Hour, nil, nil)
	app.SetSignInPolicy(policy)
	return app
}

type countingGoogle struct {
	identity  model.GoogleIdentity
	exchanges atomic.Int32
	verifies  atomic.Int32
}

func (g *countingGoogle) Exchange(context.Context, string, string, string) (string, error) {
	g.exchanges.Add(1)
	return "an.id.token", nil
}

func (g *countingGoogle) Verify(context.Context, string) (model.GoogleIdentity, error) {
	g.verifies.Add(1)
	return g.identity, nil
}

func (g *countingGoogle) calls() int32 { return g.exchanges.Load() + g.verifies.Load() }

type refusingEnroller struct {
	calls atomic.Int32
}

func (e *refusingEnroller) handler() ports.SelfEnroller {
	return cqrs.HandlerFunc[classescommand.EnrolNewMember, classesdomain.EnrolResult](
		func(context.Context, classescommand.EnrolNewMember) (classesdomain.EnrolResult, error) {
			e.calls.Add(1)
			return classesdomain.EnrolResult{Outcome: classesdomain.PreviewInvalid}, nil
		})
}
