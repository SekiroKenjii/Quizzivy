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
	// The reason to change a password is that someone else may know the old
	// one. If the sessions it authorised survive, the change accomplishes
	// nothing -- the intruder keeps refreshing indefinitely and never needs the
	// password again.
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

	// ...and the device that made the change stays signed in. Logging the user
	// out of the tab they are typing in is a bug, not extra safety.
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
	// §5.1's first-login flow: the teacher sets a temporary password and the
	// student is forced to change it. If the flag survives the change, the
	// student is trapped on that screen forever.
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
	// Length is checked first so a typo costs nothing, and so the endpoint is
	// not a way to make an authenticated caller burn Argon2id time at will.
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
				// Deliberately the WRONG current password: length must be
				// rejected before credentials are even consulted.
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
	// If we cannot identify the caller's session we cannot spare it, and
	// sparing the wrong one -- or sparing none by accident -- are not equally
	// bad. Signing everyone out is the safe direction.
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
	// KeepRefreshToken is attacker-influenced only in the sense that a caller
	// could present a token they hold from another account. Scoping the lookup
	// to the changing user is what stops that from meaning anything.
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
