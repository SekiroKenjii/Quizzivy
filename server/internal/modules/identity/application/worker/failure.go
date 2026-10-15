package worker

import (
	"context"
	"errors"
	"net"

	"quizzivy/internal/modules/identity/domain"
)

func failureClass(err error) string {
	var database interface{ SQLState() string }
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, domain.ErrRoleChanged):
		return "role changed"
	case errors.Is(err, errNothingLeft):
		return "nothing left"
	case errors.As(err, &database):
		return "database " + database.SQLState()
	case errors.As(err, &network):
		return "network"
	}
	return "error"
}
