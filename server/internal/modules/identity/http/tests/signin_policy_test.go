package http_test

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	classesapp "quizzivy/internal/modules/classes/application"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/cqrs"
)

const signInPassword = "mật-khẩu-của-trung-tâm"

var storedSignInPassword = sync.OnceValue(func() string {
	hash, err := domain.Passwords.Hash(context.Background(), signInPassword)
	if err != nil {
		panic(err)
	}
	return hash
})

type signInStore struct {
	domain.Users
	user    domain.User
	touched []string
}

func (s *signInStore) FindUserByEmail(_ context.Context, email string) (domain.User, error) {
	s.touched = append(s.touched, "FindUserByEmail")
	if strings.EqualFold(email, s.user.Email) {
		return s.user, nil
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (s *signInStore) FindUserByID(_ context.Context, id string) (domain.User, error) {
	s.touched = append(s.touched, "FindUserByID")
	user := s.user
	user.ID = id
	return user, nil
}

func (s *signInStore) FindUserByProviderIdentity(context.Context, string, string) (domain.User, error) {
	s.touched = append(s.touched, "FindUserByProviderIdentity")
	return domain.User{}, domain.ErrUserNotFound
}

func (s *signInStore) CreateRefreshToken(context.Context, domain.RefreshTokenRecord, domain.SessionBasis) error {
	s.touched = append(s.touched, "CreateRefreshToken")
	return nil
}

func (s *signInStore) StampSignIn(context.Context, string, domain.SignInMethod, time.Time) error {
	s.touched = append(s.touched, "StampSignIn")
	return nil
}

func (s *signInStore) LinkIdentity(context.Context, string, string, string, string) error {
	s.touched = append(s.touched, "LinkIdentity")
	return nil
}

func (s *signInStore) UnlinkIdentity(context.Context, string, string) (bool, error) {
	s.touched = append(s.touched, "UnlinkIdentity")
	return true, nil
}

func (s *signInStore) WriteAudit(context.Context, audit.Entry) error {
	s.touched = append(s.touched, "WriteAudit")
	return nil
}

func signInStoreOf(email string) *signInStore {
	hash := storedSignInPassword()
	return &signInStore{user: domain.User{ID: uuid.NewString(), Email: email, FullName: "Nguyễn Văn An", PasswordHash: &hash}}
}

type spiedGoogle struct {
	email string
	calls atomic.Int32
}

func (g *spiedGoogle) Exchange(context.Context, string, string, string) (string, error) {
	g.calls.Add(1)
	return "an.id.token", nil
}

func (g *spiedGoogle) Verify(context.Context, string) (model.GoogleIdentity, error) {
	g.calls.Add(1)
	return model.GoogleIdentity{Subject: "google-sub-1", Email: g.email, EmailVerified: true, Name: "Nguyễn Văn An"}, nil
}

func signInHandler(t *testing.T, store domain.Users, policy organizationmodel.SignInPolicy, google *spiedGoogle) identityhttp.Identity {
	t.Helper()
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store, issuer, time.Hour, nil, nil)
	app.SetSignInPolicy(cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return policy, nil
		}))
	app.SetGoogle(google, nil)
	return identityhttp.NewIdentity(app, time.Hour, false, nil)
}

func centre(change func(*organizationmodel.SignInPolicy)) organizationmodel.SignInPolicy {
	policy := organizationmodel.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true}
	if change != nil {
		change(&policy)
	}
	return policy
}

func login(h identityhttp.Identity, email, password string) handlerCall {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.Login(ctx, openapi.LoginRequestObject{
			Body: &openapi.LoginJSONRequestBody{Email: openapi_types.Email(email), Password: password},
		})
		return render(w, openapi.LoginResponseObject.VisitLoginResponse, response, err)
	}
}

func googleAuth(h identityhttp.Identity, joinCode *string) handlerCall {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.GoogleAuth(ctx, openapi.GoogleAuthRequestObject{
			Body: &openapi.GoogleAuthJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "http://localhost:5173/auth/callback", JoinCode: joinCode},
		})
		return render(w, openapi.GoogleAuthResponseObject.VisitGoogleAuthResponse, response, err)
	}
}

func linkGoogle(h identityhttp.Identity) handlerCall {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.LinkGoogle(ctx, openapi.LinkGoogleRequestObject{
			Body: &openapi.LinkGoogleJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "http://localhost:5173/auth/callback"},
		})
		return render(w, openapi.LinkGoogleResponseObject.VisitLinkGoogleResponse, response, err)
	}
}

func unlinkGoogle(h identityhttp.Identity) handlerCall {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.UnlinkGoogle(ctx, openapi.UnlinkGoogleRequestObject{})
		return render(w, openapi.UnlinkGoogleResponseObject.VisitUnlinkGoogleResponse, response, err)
	}
}

type handlerCall func(context.Context, http.ResponseWriter) error

var googleCalls = []struct {
	name string
	call func(identityhttp.Identity) handlerCall
}{
	{"a sign-in", func(h identityhttp.Identity) handlerCall { return googleAuth(h, nil) }},
	{"a sign-in with a join code", func(h identityhttp.Identity) handlerCall { return googleAuth(h, ptr("ABCD2345")) }},
	{"linking", linkGoogle},
}

func ptr[T any](v T) *T { return &v }

func TestPasswordSignInOffAnswers403ToEveryLoginAlike(t *testing.T) {
	suspended := signInStoreOf("suspended@example.com")
	now := time.Now()
	suspended.user.DisabledAt = &now
	for _, language := range []struct {
		name, accept, want string
	}{
		{"vi", "", "Trung tâm đã tắt đăng nhập bằng mật khẩu. Hãy đăng nhập bằng Google."},
		{"en", "en", "Your centre has turned off password sign-in. Sign in with Google."},
	} {
		for name, attempt := range map[string]struct {
			store           *signInStore
			email, password string
		}{
			"the right password": {signInStoreOf("an@example.com"), "an@example.com", signInPassword},
			"a wrong password":   {signInStoreOf("an@example.com"), "an@example.com", "sai-mật-khẩu"},
			"an unknown email":   {signInStoreOf("an@example.com"), "nobody@example.com", signInPassword},
			"a disabled account": {suspended, "suspended@example.com", signInPassword},
		} {
			t.Run(language.name+"/"+name, func(t *testing.T) {
				h := signInHandler(t, attempt.store, centre(func(p *organizationmodel.SignInPolicy) { p.Password = false }), &spiedGoogle{})

				got := answerIn(t, language.accept, login(h, attempt.email, attempt.password))

				if got.status != http.StatusForbidden || got.Error.Code != openapi.PASSWORDSIGNINDISABLED || got.Error.Message != language.want {
					t.Errorf("answered %d %s %q, want 403 PASSWORD_SIGN_IN_DISABLED %q", got.status, got.Error.Code, got.Error.Message, language.want)
				}
				if len(attempt.store.touched) != 0 {
					t.Errorf("the user store was consulted: %v", attempt.store.touched)
				}
			})
		}
	}
}

func TestPasswordSignInOnStillAnswers401ToAnUnknownEmail(t *testing.T) {
	h := signInHandler(t, signInStoreOf("an@example.com"), centre(nil), &spiedGoogle{})

	got := answerIn(t, "", login(h, "nobody@example.com", signInPassword))

	if got.status != http.StatusUnauthorized || got.Error.Code != openapi.INVALIDCREDENTIALS {
		t.Errorf("answered %d %s, want 401 INVALID_CREDENTIALS", got.status, got.Error.Code)
	}
}

func TestGoogleSignInOffAnswers403OnSignInAndLinkingBeforeGoogleIsCalled(t *testing.T) {
	for _, language := range []struct {
		name, accept, want string
	}{
		{"vi", "", "Trung tâm đã tắt đăng nhập bằng Google."},
		{"en", "en", "Your centre has turned off Google sign-in."},
	} {
		for _, c := range googleCalls {
			t.Run(language.name+"/"+c.name, func(t *testing.T) {
				google := &spiedGoogle{email: "an@school.edu"}
				h := signInHandler(t, signInStoreOf("an@school.edu"), centre(func(p *organizationmodel.SignInPolicy) { p.Google = false }), google)

				got := answerIn(t, language.accept, c.call(h))

				if got.status != http.StatusForbidden || got.Error.Code != openapi.GOOGLESIGNINDISABLED || got.Error.Message != language.want {
					t.Errorf("answered %d %s %q, want 403 GOOGLE_SIGN_IN_DISABLED %q", got.status, got.Error.Code, got.Error.Message, language.want)
				}
				if calls := google.calls.Load(); calls != 0 {
					t.Errorf("Google was called %d times", calls)
				}
			})
		}
	}
}

func TestAGoogleAddressOutsideTheAllowedDomainsAnswers403OnSignInAndLinking(t *testing.T) {
	for _, language := range []struct {
		name, accept, want string
	}{
		{"vi", "", "Trung tâm chỉ cho phép đăng nhập bằng tài khoản Google thuộc một số tên miền nhất định. Tài khoản này không nằm trong số đó."},
		{"en", "en", "Your centre only allows Google accounts from certain domains, and this account is not one of them."},
	} {
		for _, c := range googleCalls {
			t.Run(language.name+"/"+c.name, func(t *testing.T) {
				store := signInStoreOf("an@gmail.com")
				h := signInHandler(t, store, centre(func(p *organizationmodel.SignInPolicy) { p.AllowedGoogleDomains = []string{"school.edu"} }), &spiedGoogle{email: "an@gmail.com"})

				got := answerIn(t, language.accept, c.call(h))

				if got.status != http.StatusForbidden || got.Error.Code != openapi.GOOGLEDOMAINNOTALLOWED || got.Error.Message != language.want {
					t.Errorf("answered %d %s %q, want 403 GOOGLE_DOMAIN_NOT_ALLOWED %q", got.status, got.Error.Code, got.Error.Message, language.want)
				}
				for _, write := range []string{"LinkIdentity", "CreateRefreshToken", "WriteAudit"} {
					if slices.Contains(store.touched, write) {
						t.Errorf("the refusal reached %s", write)
					}
				}
			})
		}
	}
}

func TestAGoogleAddressInsideTheAllowedDomainsIsNotRefusedForItsDomain(t *testing.T) {
	h := signInHandler(t, &signInStore{}, centre(func(p *organizationmodel.SignInPolicy) { p.AllowedGoogleDomains = []string{"school.edu"} }), &spiedGoogle{email: "An@School.EDU"})

	got := answerIn(t, "", googleAuth(h, nil))

	if got.status != http.StatusForbidden || got.Error.Code != openapi.ACCOUNTNOTPROVISIONED {
		t.Errorf("answered %d %s, want the 403 ACCOUNT_NOT_PROVISIONED of an address nobody registered", got.status, got.Error.Code)
	}
}

func TestUnlinkingGoogleWithPasswordSignInOffAnswers409(t *testing.T) {
	for _, language := range []struct {
		name, accept, want string
	}{
		{"vi", "", "Bạn không thể bỏ liên kết Google vì sẽ không còn cách nào đăng nhập: cần có mật khẩu và trung tâm phải đang cho đăng nhập bằng mật khẩu."},
		{"en", "en", "You cannot unlink Google because no way to sign in would be left: you need a password and your centre must allow password sign-in."},
	} {
		t.Run(language.name, func(t *testing.T) {
			store := signInStoreOf("an@example.com")
			h := signInHandler(t, store, centre(func(p *organizationmodel.SignInPolicy) { p.Password = false }), &spiedGoogle{})

			got := answerIn(t, language.accept, unlinkGoogle(h))

			if got.status != http.StatusConflict || got.Error.Code != openapi.LASTLOGINMETHOD || got.Error.Message != language.want {
				t.Errorf("answered %d %s %q, want 409 LAST_LOGIN_METHOD %q", got.status, got.Error.Code, got.Error.Message, language.want)
			}
			if slices.Contains(store.touched, "UnlinkIdentity") {
				t.Error("the identity was unlinked")
			}
		})
	}
}

func TestUnlinkingGoogleWithPasswordSignInOnAnswers204(t *testing.T) {
	store := signInStoreOf("an@example.com")
	h := signInHandler(t, store, centre(nil), &spiedGoogle{})

	response, err := h.UnlinkGoogle(contextAs(t, access.Principal{UserID: store.user.ID}), openapi.UnlinkGoogleRequestObject{})

	if _, ok := response.(openapi.UnlinkGoogle204Response); err != nil || !ok {
		t.Fatalf("answered %T (%v), want 204", response, err)
	}
}

const liveJoinCode = "ABCD2345"

type liveCodeRepository struct {
	classesdomain.Repository
}

func (liveCodeRepository) Enrol(_ context.Context, in classesdomain.EnrolInput) (classesdomain.EnrolResult, error) {
	keys, err := classesdomain.NewJoinCodeKeys([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		return classesdomain.EnrolResult{}, err
	}
	live := keys.Hash(liveJoinCode)
	if !slices.ContainsFunc(in.Code.Hashes(), func(hash []byte) bool { return bytes.Equal(hash, live) }) {
		return classesdomain.EnrolResult{Outcome: classesdomain.PreviewInvalid}, nil
	}
	return classesdomain.EnrolResult{
		Outcome: classesdomain.PreviewOK,
		UserID:  uuid.NewString(),
		Class:   classesdomain.EnrolledClass{ID: uuid.NewString(), Name: "Lớp 10A"},
	}, nil
}

func joiningHandler(t *testing.T, joinCodes bool) identityhttp.Identity {
	t.Helper()
	keys, err := classesdomain.NewJoinCodeKeys([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		t.Fatal(err)
	}
	policy := cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return centre(func(p *organizationmodel.SignInPolicy) { p.JoinCodes = joinCodes }), nil
		})
	classes := classesapp.New(liveCodeRepository{}, nil, keys).WithSignInPolicy(policy)
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &signInStore{user: domain.User{Email: "registered@example.com", FullName: "Trần Thị B"}}
	app := application.New(store, issuer, time.Hour, nil, nil)
	app.SetSignInPolicy(policy)
	app.SetGoogle(&spiedGoogle{email: "newcomer@example.com"}, classes.Commands.EnrolNewMember)
	return identityhttp.NewIdentity(app, time.Hour, false, nil)
}

func TestJoiningOffAnswersAGoogleSignInWithAValidCodeAsAnUnknownCodeIsAnswered(t *testing.T) {
	for _, language := range []struct {
		name, accept, message string
	}{
		{"vi", "", "Mã lớp không đúng. Vui lòng kiểm tra lại."},
		{"en", "en", "That class code is not right. Please check it."},
	} {
		t.Run(language.name, func(t *testing.T) {
			on, off := joiningHandler(t, true), joiningHandler(t, false)

			unknown := answerIn(t, language.accept, googleAuth(on, ptr("ZZZZ-ZZZZ")))
			if unknown.status != http.StatusNotFound || unknown.Error.Code != openapi.JOINCODEINVALID || unknown.Error.Message != language.message {
				t.Fatalf("an unknown code answered %d %s %q, want 404 JOIN_CODE_INVALID %q", unknown.status, unknown.Error.Code, unknown.Error.Message, language.message)
			}
			if live := answerIn(t, language.accept, googleAuth(on, ptr(liveJoinCode))); live.status != http.StatusOK {
				t.Fatalf("with joining on, the live code answered %d %s", live.status, live.Error.Code)
			}

			for _, typed := range []string{liveJoinCode, "ABCD-2345", "ZZZZ-ZZZZ"} {
				if refused := answerIn(t, language.accept, googleAuth(off, ptr(typed))); refused != unknown {
					t.Errorf("with joining off, %q answered %+v, want what an unknown code answers, %+v", typed, refused, unknown)
				}
			}
		})
	}
}
