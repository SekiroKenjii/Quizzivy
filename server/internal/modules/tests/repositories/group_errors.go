package repositories

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"quizzivy/internal/modules/tests/domain"
)

func groupWriteError(err error) error {
	var constraint *pgconn.PgError
	if !errors.As(err, &constraint) {
		return err
	}
	switch constraint.Code {
	case "23505":
		return domain.ErrGroupConflict
	case "23503", "23514":
		return &domain.GroupError{Rule: "group_reference"}
	default:
		return err
	}
}
