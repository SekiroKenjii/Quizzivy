//go:build integration

package application_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	accessapp "quizzivy/internal/modules/access/application"
	accessdomain "quizzivy/internal/modules/access/domain"
	accessrepo "quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/secrets"
	"quizzivy/internal/shared/access"
)

type usersWorld struct {
	admin  *pgxpool.Pool
	app    *application.Application
	marker string
	policy organizationmodel.SignInPolicy
	actor  string
}

func newUsersWorld(t *testing.T, withOutbox bool) *usersWorld {
	t.Helper()
	admin, appdb := newPool(t), linkAppPool(t)
	key, err := secrets.ParseKey(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := secrets.New(key, secrets.PurposeEmailOutbox)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	w := &usersWorld{admin: admin, marker: "adm" + nonce(t) + ".example.test", policy: openPolicy()}
	dbx := db.NewContext(appdb)
	var outbox repositories.Outbox
	if withOutbox {
		outbox = email.NewOutbox(sealer)
	}
	links := repositories.NewSetPasswordLinks(dbx, outbox)
	w.app = application.New(repositories.NewUsers(dbx), issuer, 30*24*time.Hour, nil, nil)
	w.app.SetPasswordLinks(links)
	w.app.SetSignInPolicy(switchablePolicy(func() organizationmodel.SignInPolicy { return w.policy }))
	w.app.SetAdminUsers(repositories.NewAdminUsers(dbx, links), accessapp.New(accessrepo.NewPostgres(dbx)).Queries.AssignableRole)
	t.Cleanup(func() {
		ctx := context.Background()
		owned := `SELECT id FROM app.users WHERE email LIKE '%@' || $1`
		for _, stmt := range []string{
			`DELETE FROM app.email_outbox WHERE to_address LIKE '%@' || $1`,
			`DELETE FROM app.class_members WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.classes WHERE teacher_id IN (` + owned + `)`,
			`DELETE FROM app.audit_log WHERE entity_id IN (` + owned + `) OR actor_user_id IN (` + owned + `)`,
			`DELETE FROM app.users WHERE email LIKE '%@' || $1`,
		} {
			if _, err := admin.Exec(ctx, stmt, w.marker); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
	w.actor = w.insertUser(t, "actor", "teacher")
	return w
}

func (w *usersWorld) insertUser(t *testing.T, name, builtin string) string {
	t.Helper()
	var id string
	if err := w.admin.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = $3))
		RETURNING id::text`, name+"-"+uuid.NewString()[:8]+"@"+w.marker, "Tài khoản "+name, builtin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *usersWorld) role(t *testing.T, builtin string) string {
	t.Helper()
	var id string
	if err := w.admin.QueryRow(context.Background(), `SELECT id::text FROM app.roles WHERE builtin_key = $1`, builtin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *usersWorld) class(t *testing.T, archived bool) string {
	t.Helper()
	var id string
	if err := w.admin.QueryRow(context.Background(), `
		INSERT INTO app.classes (name, teacher_id, archived_at)
		VALUES ($1, $2::uuid, CASE WHEN $3 THEN now() END) RETURNING id::text`,
		"Lớp "+nonce(t), w.actor, archived).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *usersWorld) email(name string) string {
	return name + "-" + uuid.NewString()[:8] + "@" + w.marker
}

func (w *usersWorld) everything() domain.WriteRequest {
	return domain.WriteRequest{ActorID: w.actor, All: true, IP: "203.0.113.5", UserAgent: "users-test",
		Grants: access.NewSet(access.All()...).Without(access.LearningTakeTests)}
}

func (w *usersWorld) peopleOnly() domain.WriteRequest {
	return domain.WriteRequest{ActorID: w.actor, Grants: access.NewSet(access.PeopleUsersManage, access.TeachingGrading)}
}

func (w *usersWorld) create(req domain.WriteRequest, in domain.NewUser) (command.CreateUserResult, error) {
	return w.app.Commands.CreateUser.Handle(context.Background(), command.CreateUser{Request: req, Input: in})
}

func (w *usersWorld) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := w.admin.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *usersWorld) nothingWritten(t *testing.T, email string) {
	t.Helper()
	if n := w.count(t, `SELECT count(*) FROM app.users WHERE lower(email) = lower($1)`, email); n != 0 {
		t.Errorf("a refused create left %d account(s)", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.email_outbox WHERE lower(to_address) = lower($1)`, email); n != 0 {
		t.Errorf("a refused create left %d message(s) in the outbox", n)
	}
}

func TestATemporaryPasswordAccountKeepsOnlyItsHashAndIsAuditedWithItsRole(t *testing.T) {
	w := newUsersWorld(t, true)
	in := domain.NewUser{Email: w.email("gv"), FullName: "Lê Thị Hồng", RoleID: w.role(t, "teacher"), Delivery: domain.DeliveryTemporaryPassword}

	result, err := w.create(w.everything(), in)
	if err != nil {
		t.Fatal(err)
	}
	if result.TemporaryPassword == "" {
		t.Fatal("no temporary password was returned")
	}
	user := result.User
	if user.Role.BuiltinKey != "teacher" || !user.HasPassword || user.HasGoogle || user.Status != domain.AccountMustChangePassword || user.ClassCount != 0 || user.FirstClass != nil {
		t.Errorf("row = %+v, want a Teacher on a temporary password with no class", user)
	}

	var hash, creator string
	var must bool
	if err := w.admin.QueryRow(context.Background(), `SELECT password_hash, must_change_password, created_by::text FROM app.users WHERE id = $1::uuid`, user.ID).Scan(&hash, &must, &creator); err != nil {
		t.Fatal(err)
	}
	if hash == result.TemporaryPassword || !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("stored %q, want the Argon2id hash and never the password", hash)
	}
	if ok, err := domain.Passwords.Verify(context.Background(), result.TemporaryPassword, hash); err != nil || !ok {
		t.Errorf("the stored hash does not verify the returned password: %v %v", ok, err)
	}
	if !must || creator != w.actor {
		t.Errorf("must_change_password = %v, created_by = %s, want true and the actor %s", must, creator, w.actor)
	}
	if w.count(t, `SELECT count(*) FROM app.email_outbox WHERE lower(to_address) = lower($1)`, in.Email) != 0 {
		t.Error("a temporary password account was sent an email")
	}

	var action string
	var diff []byte
	var ip, agent *string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT action, diff, host(ip), user_agent FROM app.audit_log
		 WHERE entity_id = $1::uuid AND actor_user_id = $2::uuid`, user.ID, w.actor).Scan(&action, &diff, &ip, &agent); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(diff, &got); err != nil {
		t.Fatal(err)
	}
	var roleName string
	if err := w.admin.QueryRow(context.Background(), `SELECT name FROM app.roles WHERE id = $1::uuid`, in.RoleID).Scan(&roleName); err != nil {
		t.Fatal(err)
	}
	if action != "user.created" || got["role"] != roleName || got["roleId"] != in.RoleID || got["delivery"] != "temporary_password" {
		t.Errorf("audit = %s %v", action, got)
	}
	if ip == nil || *ip != "203.0.113.5" || agent == nil || *agent != "users-test" {
		t.Errorf("audit origin = %v %v, want the request's", ip, agent)
	}
}

func TestACentreThatAllowsItMayLetANewAccountKeepItsTemporaryPassword(t *testing.T) {
	w := newUsersWorld(t, true)
	w.policy = policyWith(keepTemporary)
	result, err := w.create(w.everything(), domain.NewUser{Email: w.email("gv"), FullName: "Phạm Quang", RoleID: w.role(t, "teacher"), Delivery: domain.DeliveryTemporaryPassword})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Status != domain.AccountNeverSignedIn {
		t.Errorf("status = %s, want never_signed_in: the password is not marked for change", result.User.Status)
	}
}

func TestAnEmailLinkAccountHasNoPasswordAndItsInviteCommitsWithIt(t *testing.T) {
	w := newUsersWorld(t, true)
	in := domain.NewUser{Email: w.email("hv"), FullName: "Đỗ Minh Anh", RoleID: w.role(t, "student"), Delivery: domain.DeliveryEmailLink}

	result, err := w.create(w.everything(), in)
	if err != nil {
		t.Fatal(err)
	}
	if result.TemporaryPassword != "" {
		t.Error("an email link account was given a temporary password")
	}
	user := result.User
	if user.HasPassword || user.Status != domain.AccountNeverSignedIn {
		t.Errorf("row = %+v, want no password and never signed in", user)
	}

	var purpose, by string
	var used, revoked bool
	if err := w.admin.QueryRow(context.Background(), `
		SELECT purpose, created_by::text, used_at IS NOT NULL, revoked_at IS NOT NULL
		  FROM app.set_password_tokens WHERE user_id = $1::uuid`, user.ID).Scan(&purpose, &by, &used, &revoked); err != nil {
		t.Fatal(err)
	}
	if purpose != "invite" || by != w.actor || used || revoked {
		t.Errorf("link = %s by %s used %v revoked %v, want a live invite from the actor", purpose, by, used, revoked)
	}
	var template string
	var sealed bool
	if err := w.admin.QueryRow(context.Background(), `
		SELECT template, secret_ciphertext IS NOT NULL FROM app.email_outbox WHERE lower(to_address) = lower($1)`, in.Email).Scan(&template, &sealed); err != nil {
		t.Fatal(err)
	}
	if template != "account_invite" || !sealed {
		t.Errorf("message = %s sealed %v, want an account_invite carrying a sealed link", template, sealed)
	}
}

func TestAnEmailLinkWhilePasswordSignInIsOffSendsASignInInviteAndNoLink(t *testing.T) {
	w := newUsersWorld(t, true)
	w.policy = policyWith(passwordOff)
	in := domain.NewUser{Email: w.email("hv"), FullName: "Võ Thanh", RoleID: w.role(t, "student"), Delivery: domain.DeliveryEmailLink}
	result, err := w.create(w.everything(), in)
	if err != nil {
		t.Fatal(err)
	}
	if n := w.count(t, `SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1::uuid`, result.User.ID); n != 0 {
		t.Errorf("%d link(s) were issued while password sign-in is off", n)
	}
	var template string
	if err := w.admin.QueryRow(context.Background(), `SELECT template FROM app.email_outbox WHERE lower(to_address) = lower($1)`, in.Email).Scan(&template); err != nil {
		t.Fatal(err)
	}
	if template != "sign_in_invite" {
		t.Errorf("template = %s, want sign_in_invite", template)
	}
}

func TestATemporaryPasswordIsRefusedWhilePasswordSignInIsOff(t *testing.T) {
	w := newUsersWorld(t, true)
	w.policy = policyWith(passwordOff)
	in := domain.NewUser{Email: w.email("gv"), FullName: "Bùi Lan", RoleID: w.role(t, "teacher"), Delivery: domain.DeliveryTemporaryPassword}
	if _, err := w.create(w.everything(), in); !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Fatalf("error = %v, want ErrPasswordSignInDisabled", err)
	}
	w.nothingWritten(t, in.Email)
}

func TestAnInviteThatCannotBeSentTakesTheAccountBackWithIt(t *testing.T) {
	cases := []struct {
		name   string
		outbox bool
	}{{"a server with no key to seal a link", false}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newUsersWorld(t, c.outbox)
			in := domain.NewUser{Email: w.email("hv"), FullName: "Hoàng Yến", RoleID: w.role(t, "student"), Delivery: domain.DeliveryEmailLink}
			if _, err := w.create(w.everything(), in); !errors.Is(err, domain.ErrSetPasswordUnavailable) {
				t.Fatalf("error = %v, want ErrSetPasswordUnavailable", err)
			}
			w.nothingWritten(t, in.Email)
			if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user.created' AND actor_user_id = $1::uuid`, w.actor); n != 0 {
				t.Errorf("a rolled-back create left %d audit row(s)", n)
			}
		})
	}
}

func TestARoleTheCallerMayNotGiveIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	w := newUsersWorld(t, true)
	cases := []struct {
		name   string
		req    domain.WriteRequest
		roleID string
		want   error
	}{
		{"the Assistant, which no one gives", w.everything(), w.role(t, "assistant"), accessdomain.ErrNotAssignable},
		{"a role with permissions the caller lacks", w.peopleOnly(), w.role(t, "teacher"), accessdomain.ErrTargetRoleNotAllowed},
		{"the Admin role, from a caller who is not one", w.peopleOnly(), w.role(t, "admin"), accessdomain.ErrTargetRoleNotAllowed},
		{"a role that does not exist", w.everything(), uuid.NewString(), accessdomain.ErrRoleNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := domain.NewUser{Email: w.email("x"), FullName: "Ngô Bảo", RoleID: c.roleID, Delivery: domain.DeliveryEmailLink}
			if _, err := w.create(c.req, in); !errors.Is(err, c.want) {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
			w.nothingWritten(t, in.Email)
		})
	}
}

func TestACallerMayGiveAStudentRoleWhateverTheyHold(t *testing.T) {
	w := newUsersWorld(t, true)
	in := domain.NewUser{Email: w.email("hv"), FullName: "Mai Chi", RoleID: w.role(t, "student"), Delivery: domain.DeliveryTemporaryPassword}
	if _, err := w.create(w.peopleOnly(), in); err != nil {
		t.Fatalf("a caller holding only people.users.manage and grading could not add a Student: %v", err)
	}
}

func TestAClassIsGivenOnlyToAStudentTargetByACallerWhoReachesEveryClass(t *testing.T) {
	w := newUsersWorld(t, true)
	open, archived := w.class(t, false), w.class(t, true)
	student, teacher := w.role(t, "student"), w.role(t, "teacher")
	cases := []struct {
		name  string
		req   domain.WriteRequest
		role  string
		class string
		want  error
	}{
		{"a Teacher in a class", w.everything(), teacher, open, domain.ErrClassNotAllowed},
		{"a Student, by a caller without scope.all", w.peopleOnly(), student, open, domain.ErrClassNotAllowed},
		{"a Student in an archived class", w.everything(), student, archived, domain.ErrClassNotFound},
		{"a Student in a class that does not exist", w.everything(), student, uuid.NewString(), domain.ErrClassNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := domain.NewUser{Email: w.email("hv"), FullName: "Trịnh Khoa", RoleID: c.role, ClassID: c.class, Delivery: domain.DeliveryEmailLink}
			if _, err := w.create(c.req, in); !errors.Is(err, c.want) {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
			w.nothingWritten(t, in.Email)
		})
	}

	in := domain.NewUser{Email: w.email("hv"), FullName: "Trịnh Khoa", RoleID: student, ClassID: open, Delivery: domain.DeliveryEmailLink}
	result, err := w.create(w.everything(), in)
	if err != nil {
		t.Fatal(err)
	}
	if result.User.ClassCount != 1 || result.User.FirstClass == nil {
		t.Errorf("classes = %d %v, want the one class", result.User.ClassCount, result.User.FirstClass)
	}
	var via, addedBy string
	if err := w.admin.QueryRow(context.Background(), `SELECT joined_via::text, added_by::text FROM app.class_members WHERE class_id = $1::uuid AND user_id = $2::uuid`, open, result.User.ID).Scan(&via, &addedBy); err != nil {
		t.Fatal(err)
	}
	if via != "admin" || addedBy != w.actor {
		t.Errorf("membership = %s by %s, want admin by the actor", via, addedBy)
	}
}

func TestAnEmailAlreadyInUseInAnyCaseIsRefusedWithNoSecondInvite(t *testing.T) {
	w := newUsersWorld(t, true)
	in := domain.NewUser{Email: w.email("hv"), FullName: "Cao Duy", RoleID: w.role(t, "student"), Delivery: domain.DeliveryEmailLink}
	if _, err := w.create(w.everything(), in); err != nil {
		t.Fatal(err)
	}
	again := in
	again.Email = strings.ToUpper(in.Email)
	if _, err := w.create(w.everything(), again); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("error = %v, want ErrEmailTaken", err)
	}
	if n := w.count(t, `SELECT count(*) FROM app.email_outbox WHERE lower(to_address) = lower($1)`, in.Email); n != 1 {
		t.Errorf("%d message(s) for the address, want the first invite only", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user.created' AND actor_user_id = $1::uuid`, w.actor); n != 1 {
		t.Errorf("%d audit row(s), want one", n)
	}
}

func TestAFullNameIsStoredComposed(t *testing.T) {
	w := newUsersWorld(t, true)
	decomposed := "Nguyễn Văn A"
	in := domain.NewUser{Email: w.email("hv"), FullName: decomposed, RoleID: w.role(t, "student"), Delivery: domain.DeliveryEmailLink}
	result, err := w.create(w.everything(), in)
	if err != nil {
		t.Fatal(err)
	}
	if result.User.FullName != "Nguyễn Văn A" {
		t.Errorf("name = %q, want %q", result.User.FullName, "Nguyễn Văn A")
	}
}

func TestWithoutADirectoryCreateUserIsNotAvailable(t *testing.T) {
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(nil, issuer, time.Hour, nil, nil)
	if app.Commands.CreateUser != nil {
		t.Error("CreateUser is available before SetAdminUsers")
	}
}
