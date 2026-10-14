// Package model is what the organization module answers other modules: the sign-in policy identity and classes enforce, and the centre's name and contact lines that head every email.
package model

import (
	"slices"
	"strings"
)

// SignInPolicy is how people sign in, as the centre last saved it.
// AllowedGoogleDomains, lower-case, bounds the Google accounts that may sign
// in, link or join; empty allows every domain.
type SignInPolicy struct {
	Google               bool
	Password             bool
	ForcePasswordChange  bool
	JoinCodes            bool
	AllowedGoogleDomains []string
}

// AllowsGoogleEmail reports whether a Google account with email may be used
// under the domain list. It says nothing about whether Google sign-in is on.
func (p SignInPolicy) AllowsGoogleEmail(email string) bool {
	if len(p.AllowedGoogleDomains) == 0 {
		return true
	}
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return false
	}
	return slices.Contains(p.AllowedGoogleDomains, strings.ToLower(email[at+1:]))
}

// Branding is the centre's name and contact lines. Every field but Name may
// be empty.
type Branding struct {
	Name           string
	ShortName      string
	Website        string
	ContactEmail   string
	FrontDeskPhone string
	FrontDeskHours string
	Address        string
}
