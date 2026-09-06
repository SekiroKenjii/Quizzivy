package support

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
)

type ChangePasswordInput struct {
	UserID           string
	CurrentPassword  string
	NewPassword      string
	KeepRefreshToken string
	IP               string
	UserAgent        string
}

type GoogleSignInInput struct {
	Code         string
	CodeVerifier string
	RedirectURI  string
	JoinCode     string
	UserAgent    string
	IP           string
}

type LinkGoogleInput struct {
	UserID       string
	Code         string
	CodeVerifier string
	RedirectURI  string
	IP           string
	UserAgent    string
}

type RefreshInput struct {
	Token     string
	UserAgent string
	IP        string
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

func NewRefreshToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

// NewTemporaryPassword returns a fresh temporary password and its hash.
func TemporaryPassword(ctx context.Context) (password, hash string, err error) {
	password, err = domain.Passwords.Temporary()
	if err != nil {
		return "", "", err
	}
	hash, err = domain.Passwords.Hash(ctx, password)
	if err != nil {
		return "", "", err
	}
	return password, hash, nil
}
