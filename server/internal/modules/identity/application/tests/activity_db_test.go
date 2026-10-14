//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

func TestSigningInWithAPasswordStampsTheSignIn(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	var signedIn *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT last_sign_in_at FROM app.users WHERE id = $1`, id).Scan(&signedIn); err != nil || signedIn != nil {
		t.Fatalf("a new account reads signed in at %v (%v)", signedIn, err)
	}
	session, err := svc.Commands.Login.Handle(context.Background(), command.Login{Email: email, Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	var at, active *time.Time
	var method *string
	if err := pool.QueryRow(context.Background(),
		`SELECT last_sign_in_at, last_active_at, last_sign_in_method FROM app.users WHERE id = $1`, id).Scan(&at, &active, &method); err != nil {
		t.Fatal(err)
	}
	if at == nil || active == nil || !at.Equal(*active) {
		t.Fatalf("last_sign_in_at %v and last_active_at %v, want the same moment", at, active)
	}
	if method == nil || *method != string(domain.SignInPassword) {
		t.Errorf("method = %v, want password", method)
	}
	if session.User.LastSignInAt == nil {
		t.Error("the session's user does not carry the sign-in")
	}
}

func TestAFailedSignInStampsNothing(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	if _, err := svc.Commands.Login.Handle(context.Background(), command.Login{Email: email, Password: "sai-mật-khẩu"}); err == nil {
		t.Fatal("a wrong password signed in")
	}
	var at *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT last_sign_in_at FROM app.users WHERE id = $1`, id).Scan(&at); err != nil || at != nil {
		t.Errorf("last_sign_in_at = %v (%v), want null", at, err)
	}
}

func TestChangingThePasswordStampsWhenItChanged(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	before, err := svc.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: id})
	if err != nil || before.PasswordChangedAt != nil {
		t.Fatalf("before: %v, %v", before.PasswordChangedAt, err)
	}
	if _, err := svc.Commands.ChangePassword.Handle(ctx, command.ChangePassword{UserID: id, CurrentPassword: testPassword, NewPassword: newPassword}); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: id})
	if err != nil {
		t.Fatal(err)
	}
	if after.PasswordChangedAt == nil || time.Since(*after.PasswordChangedAt) > time.Minute {
		t.Errorf("password_changed_at = %v, want just now", after.PasswordChangedAt)
	}
}

func TestActivityIsWrittenOnceAndMovesLastActiveForward(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	svc.SetActivity(repositories.NewActivity(db.NewContext(pool)), time.FixedZone("Asia/Ho_Chi_Minh", 7*3600), nil)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	stamp(t, svc, id)
	stamp(t, svc, "0198f2a0-0000-7000-8000-000000000000")
	svc.FlushActivity(ctx)

	count := func() (days int, active *time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.user_activity_days WHERE user_id = $1),
			(SELECT last_active_at FROM app.users WHERE id = $1)`, id).Scan(&days, &active); err != nil {
			t.Fatal(err)
		}
		return days, active
	}
	days, active := count()
	if days != 1 || active == nil {
		t.Fatalf("%d day rows and last_active_at %v after one flush, want 1 and a time", days, active)
	}

	if err := repositories.NewActivity(db.NewContext(pool)).RecordActivity(ctx, []domain.ActivityEntry{
		{UserID: id, At: active.Add(-time.Hour), Day: time.Now().UTC().Truncate(24 * time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	_, later := count()
	if !later.Equal(*active) {
		t.Errorf("an older entry moved last_active_at from %v to %v", active, later)
	}
}

func TestTheStudentsListFiltersByAccountStatus(t *testing.T) {
	pool := newPool(t)
	svc := application.New(nil, nil, 0, repositories.NewStudents(db.NewContext(pool)), attemptsrepo.NewStudentStats(db.NewContext(pool)))
	tag := nonce(t)
	named := func(s *userSpec) { s.fullName = "Trạng thái " + tag }
	signedIn, _ := makeUser(t, pool, named)
	never, _ := makeUser(t, pool, named)
	reset, _ := makeUser(t, pool, named)
	off, _ := makeUser(t, pool, named, disabled)
	ctx := context.Background()
	for _, q := range []struct {
		sql string
		id  string
	}{
		{`UPDATE app.users SET last_sign_in_at = now(), last_active_at = now() WHERE id = $1`, signedIn},
		{`UPDATE app.users SET must_change_password = true WHERE id = $1`, reset},
		{`UPDATE app.users SET last_sign_in_at = now() WHERE id = $1`, off},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.id); err != nil {
			t.Fatal(err)
		}
	}
	yes, no := true, false

	list := func(in domain.StudentQuery) map[string]domain.AccountStatus {
		t.Helper()
		in.Scope, in.Query, in.Status = everyone, "Trạng thái "+tag, domain.StudentsAny
		got, err := svc.Queries.ListStudents.Handle(ctx, query.ListStudents{Query: in})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]domain.AccountStatus{}
		for _, s := range got.Items {
			out[s.ID] = s.Status()
		}
		return out
	}
	all := list(domain.StudentQuery{})
	want := map[string]domain.AccountStatus{
		signedIn: domain.AccountActive, never: domain.AccountNeverSignedIn,
		reset: domain.AccountMustChangePassword, off: domain.AccountDisabled,
	}
	for id, status := range want {
		if all[id] != status {
			t.Errorf("student %s is %q, want %q", id, all[id], status)
		}
	}

	for name, c := range map[string]struct {
		in   domain.StudentQuery
		want []string
	}{
		"neverSignedIn=true":  {domain.StudentQuery{NeverSignedIn: &yes}, []string{never, reset}},
		"neverSignedIn=false": {domain.StudentQuery{NeverSignedIn: &no}, []string{signedIn, off}},
		"never, no reset":     {domain.StudentQuery{NeverSignedIn: &yes, MustChange: &no}, []string{never}},
		"statuses active+reset": {domain.StudentQuery{Statuses: []domain.AccountStatus{
			domain.AccountActive, domain.AccountMustChangePassword}}, []string{signedIn, reset}},
		"statuses never+reset": {domain.StudentQuery{Statuses: []domain.AccountStatus{
			domain.AccountNeverSignedIn, domain.AccountMustChangePassword}}, []string{never, reset}},
		"statuses disabled": {domain.StudentQuery{Statuses: []domain.AccountStatus{domain.AccountDisabled}}, []string{off}},
	} {
		got := list(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: %d students, want %d: %v", name, len(got), len(c.want), got)
			continue
		}
		for _, id := range c.want {
			if _, ok := got[id]; !ok {
				t.Errorf("%s: missing %s in %v", name, id, got)
			}
		}
	}
}
