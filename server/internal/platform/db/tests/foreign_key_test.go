package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/platform/db"
)

func TestAForeignKeyRefusalNamesItsConstraint(t *testing.T) {
	for _, code := range []string{"23503", "23001"} {
		constraint, ok := db.ForeignKeyViolation(fmt.Errorf("delete: %w", &pgconn.PgError{Code: code, ConstraintName: "tests_owner_id_fkey"}))
		if !ok || constraint != "tests_owner_id_fkey" {
			t.Errorf("%s: (%q, %v), want the constraint", code, constraint, ok)
		}
	}
	for name, err := range map[string]error{
		"a duplicate":     &pgconn.PgError{Code: "23505", ConstraintName: "users_email_lower_key"},
		"a check":         &pgconn.PgError{Code: "23514", ConstraintName: "users_last_admin"},
		"a plain error":   errors.New("boom"),
		"no error at all": nil,
	} {
		if _, ok := db.ForeignKeyViolation(err); ok {
			t.Errorf("%s reads as a foreign-key refusal", name)
		}
	}
}

func TestACheckViolationMatchesOnlyItsConstraint(t *testing.T) {
	err := fmt.Errorf("update: %w", &pgconn.PgError{Code: "23514", ConstraintName: "users_last_admin"})
	if !db.IsCheckViolation(err, "users_last_admin") {
		t.Error("the last-admin refusal is not recognised")
	}
	if db.IsCheckViolation(err, "users_email_check") {
		t.Error("another check's name matches")
	}
	if db.IsCheckViolation(&pgconn.PgError{Code: "23503", ConstraintName: "users_last_admin"}, "users_last_admin") {
		t.Error("a foreign-key refusal reads as a check")
	}
}
