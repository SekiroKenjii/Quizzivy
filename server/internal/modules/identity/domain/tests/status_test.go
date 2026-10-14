package domain_test

import (
	"testing"
	"time"

	"quizzivy/internal/modules/identity/domain"
)

func TestAccountStatusFollowsDG54sOrder(t *testing.T) {
	for _, c := range []struct {
		name                           string
		disabled, mustChange, signedIn bool
		want                           domain.AccountStatus
	}{
		{"disabled beats everything", true, true, false, domain.AccountDisabled},
		{"disabled, signed in", true, false, true, domain.AccountDisabled},
		{"must change beats never signed in", false, true, false, domain.AccountMustChangePassword},
		{"must change, signed in", false, true, true, domain.AccountMustChangePassword},
		{"never signed in", false, false, false, domain.AccountNeverSignedIn},
		{"active", false, false, true, domain.AccountActive},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.StatusOf(c.disabled, c.mustChange, c.signedIn); got != c.want {
				t.Errorf("StatusOf = %s, want %s", got, c.want)
			}
		})
	}
}

func TestUsersAndStudentsAreReadAsTheSameStatus(t *testing.T) {
	now := time.Now()
	user := domain.User{MustChangePassword: true}
	student := domain.Student{MustChangePassword: true}
	if user.Status() != domain.AccountMustChangePassword || student.Status() != user.Status() {
		t.Errorf("user %s, student %s", user.Status(), student.Status())
	}
	user, student = domain.User{LastSignInAt: &now}, domain.Student{LastSignInAt: &now, DisabledAt: &now}
	if user.Status() != domain.AccountActive || student.Status() != domain.AccountDisabled {
		t.Errorf("user %s, student %s", user.Status(), student.Status())
	}
}
