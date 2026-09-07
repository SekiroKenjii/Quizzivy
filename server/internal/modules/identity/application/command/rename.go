package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
	"strings"
	"unicode/utf8"
)

// Rename is an account changing its own display name -- the write behind the
// "Hồ sơ" card on both settings boards (S-10, S-17). The email is deliberately
// not here: it is the login, and only an admin moves it.
type Rename struct {
	UserID    string
	FullName  string
	IP        string
	UserAgent string
}

type RenameHandler struct {
	*support.Service
}

func (s RenameHandler) Handle(ctx context.Context, cmd Rename) (domain.User, error) {
	name := strings.TrimSpace(cmd.FullName)
	switch n := utf8.RuneCountInString(name); {
	case n < domain.MinFullNameLength:
		return domain.User{}, domain.ErrNameRequired
	case n > domain.MaxFullNameLength:
		return domain.User{}, domain.ErrNameTooLong
	}

	user, err := s.Users.FindUserByID(ctx, cmd.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
	}

	return s.Users.Rename(ctx, domain.RenameRecord{
		UserID:    user.ID,
		FullName:  name,
		Now:       s.Now(),
		IP:        opt.String(cmd.IP),
		UserAgent: opt.String(cmd.UserAgent),
	})
}
