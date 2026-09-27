package model

import (
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// GoogleIdentity is what Google attests about the person who signed in; the
// subject is the stable key, the email may change.
type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type GoogleSignInResult struct {
	Session       Session
	EnrolledClass *classesdomain.EnrolledClass
}

// RefreshResult carries the new access token and the replacement refresh
// token. The refresh token goes into a Set-Cookie header and nowhere else.
type RefreshResult struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         domain.User
}

// Session is what a successful login produces: the tokens, the user, and the
// permissions the user's role holds, which the response shows the user.
type Session struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         domain.User
	Permissions  access.Set
}
