package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/auth"
)

const newPassword = "mật-khẩu-mới-dài-hơn"

func TestChangingPasswordKillsASecondDevice(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	mine := login(t, svc, email)   // the device doing the changing
	theirs := login(t, svc, email) // a second device, its own family

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID:           id,
		CurrentPassword:  testPassword,
		NewPassword:      newPassword,
		KeepRefreshToken: mine,
		IP:               "203.0.113.4",
		UserAgent:        "go-test",
	}); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: theirs}); err == nil {
		t.Error("the second device can still refresh after the password change")
	}
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: mine}); err != nil {
		t.Errorf("the changing device was signed out too: %v", err)
	}
}

func TestTheNewPasswordWorksAndTheOldOneDoesNot(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: id, CurrentPassword: testPassword, NewPassword: newPassword,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Login(ctx, auth.LoginInput{Email: email, Password: newPassword}); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
	if _, err := svc.Login(ctx, auth.LoginInput{Email: email, Password: testPassword}); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("the old password still works: %v", err)
	}
}

func TestChangingPasswordClearsMustChangePassword(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.users SET must_change_password = true WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: id, CurrentPassword: testPassword, NewPassword: newPassword,
	}); err != nil {
		t.Fatal(err)
	}

	user, err := svc.CurrentUser(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if user.MustChangePassword {
		t.Error("mustChangePassword survived the change")
	}
}

func TestAWrongCurrentPasswordChangesNothing(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	token := login(t, svc, email)

	err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: id, CurrentPassword: "not-the-password", NewPassword: newPassword,
		KeepRefreshToken: token,
	})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}

	// The failed attempt must not have revoked anything on its way out.
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: token}); err != nil {
		t.Errorf("a failed password change signed the user out: %v", err)
	}
	if _, err := svc.Login(ctx, auth.LoginInput{Email: email, Password: testPassword}); err != nil {
		t.Errorf("the original password stopped working: %v", err)
	}
}

func TestAGoogleOnlyAccountCannotChangeAPasswordItDoesNotHave(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool, googleOnly)

	err := svc.ChangePassword(context.Background(), auth.ChangePasswordInput{
		UserID: id, CurrentPassword: "", NewPassword: newPassword,
	})
	if !errors.Is(err, auth.ErrNoPasswordSet) {
		t.Fatalf("error = %v, want ErrNoPasswordSet", err)
	}
}

func TestAShortNewPasswordIsRejectedBeforeAnyHashingHappens(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)

	for _, tc := range []struct {
		name string
		pw   string
		want error
	}{
		{"seven characters", "1234567", auth.ErrPasswordTooShort},
		{"empty", "", auth.ErrPasswordTooShort},
		{"far too long", strings.Repeat("a", auth.MaxPasswordLength+1), auth.ErrPasswordTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ChangePassword(context.Background(), auth.ChangePasswordInput{
				UserID: id, CurrentPassword: "wrong", NewPassword: tc.pw,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEightCharactersIsAccepted(t *testing.T) {
	// The boundary itself, so an off-by-one in the check is visible.
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)

	if err := svc.ChangePassword(context.Background(), auth.ChangePasswordInput{
		UserID: id, CurrentPassword: testPassword, NewPassword: "12345678",
	}); err != nil {
		t.Fatalf("an 8-character password was rejected: %v", err)
	}
}

func TestAnUnknownKeepTokenRevokesEverySession(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	a := login(t, svc, email)
	b := login(t, svc, email)

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: id, CurrentPassword: testPassword, NewPassword: newPassword,
		KeepRefreshToken: "a-token-from-nowhere",
	}); err != nil {
		t.Fatal(err)
	}

	for name, token := range map[string]string{"first": a, "second": b} {
		if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: token}); err == nil {
			t.Errorf("the %s session survived a change with an unidentifiable caller", name)
		}
	}
}

func TestAnotherUsersRefreshTokenCannotSpareASession(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	victimID, victimEmail := makeUser(t, pool)
	_, otherEmail := makeUser(t, pool)
	ctx := context.Background()

	victimSession := login(t, svc, victimEmail)
	othersToken := login(t, svc, otherEmail)

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: victimID, CurrentPassword: testPassword, NewPassword: newPassword,
		KeepRefreshToken: othersToken,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: victimSession}); err == nil {
		t.Error("presenting another account's token spared a session it should not have")
	}
	// The other account is untouched: this endpoint changes one user's password.
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: othersToken}); err != nil {
		t.Errorf("another user's session was revoked: %v", err)
	}
}

func TestThePasswordChangeIsAudited(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)

	if err := svc.ChangePassword(context.Background(), auth.ChangePasswordInput{
		UserID: id, CurrentPassword: testPassword, NewPassword: newPassword,
		IP: "198.51.100.7", UserAgent: "go-test",
	}); err != nil {
		t.Fatal(err)
	}

	var action, entity string
	if err := pool.QueryRow(context.Background(),
		`SELECT action, entity FROM app.audit_log
		  WHERE actor_user_id = $1 ORDER BY id DESC LIMIT 1`, id).Scan(&action, &entity); err != nil {
		t.Fatalf("no audit row for the password change: %v", err)
	}
	if action != "user.password_changed" || entity != "user" {
		t.Errorf("audit row = %s/%s", action, entity)
	}
}

// A forced change does not ask for the current password.
//
// After a teacher-issued reset the password being replaced is one an admin
// generated and read aloud, so re-entering it proves nothing the access token
// has not already proved -- and demanding it strands the case G-07 exists for:
// a student whose password was reset, who signs in with Google and lands on
// /change-password having never held the temporary one.
func TestAForcedChangeDoesNotNeedTheCurrentPassword(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.users SET must_change_password = true WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID: id, CurrentPassword: "", NewPassword: newPassword,
	}); err != nil {
		t.Fatalf("a forced change was refused without the current password: %v", err)
	}

	user, err := svc.CurrentUser(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if user.MustChangePassword {
		t.Error("the flag survived the change that cleared it")
	}
}

// The exemption is scoped to that window and nothing wider: an ordinary change
// still has to prove the caller knows the password it is replacing, which is
// what protects a signed-in session left open on a shared machine.
func TestAnOrdinaryChangeStillNeedsTheCurrentPassword(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)

	err := svc.ChangePassword(context.Background(), auth.ChangePasswordInput{
		UserID: id, CurrentPassword: "", NewPassword: newPassword,
	})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}
