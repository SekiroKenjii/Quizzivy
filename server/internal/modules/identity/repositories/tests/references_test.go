package repositories_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
)

func TestAUsersConstraintTheMapDoesNotNameIsOther(t *testing.T) {
	for constraint, want := range map[string]domain.Reference{
		"lesson_plans_author_id_fkey":      domain.ReferencedByOther,
		"":                                 domain.ReferencedByOther,
		"tests_owner_id_fkey":              domain.ReferencedByOwnedContent,
		"attempts_student_id_fkey":         domain.ReferencedByAttempts,
		"assignment_students_user_id_fkey": domain.ReferencedByAssignments,
	} {
		if got := repositories.UserReferencedBy(constraint); got != want {
			t.Errorf("%q answers %q, want %q", constraint, got, want)
		}
	}
}

func TestAUserWriteRefusalBecomesTheDomainsAnswer(t *testing.T) {
	lastAdmin := fmt.Errorf("delete: %w", &pgconn.PgError{Code: "23514", ConstraintName: "users_last_admin"})
	if !errors.Is(repositories.UserWriteError(lastAdmin), domain.ErrLastAdmin) {
		t.Error("the last-admin refusal is not ErrLastAdmin")
	}
	var refused *domain.ReferencedError
	owned := &pgconn.PgError{Code: "23001", ConstraintName: "questions_owner_id_fkey"}
	if err := repositories.UserWriteError(owned); !errors.As(err, &refused) || refused.By != domain.ReferencedByOwnedContent || !errors.Is(err, domain.ErrReferenced) {
		t.Errorf("an owner refusal answers %v", err)
	}
	other := errors.New("boom")
	if err := repositories.UserWriteError(other); err != other {
		t.Errorf("an unrelated error became %v", err)
	}
}
