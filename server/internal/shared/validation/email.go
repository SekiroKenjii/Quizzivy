package validation

import (
	"errors"
	"fmt"
	"net/mail"
)

// ErrEmail is wrapped by every refusal from ParseEmail.
var ErrEmail = errors.New("not an email address the API can store and print back")

// ParseEmail returns the bare address of value, the form a contract email
// field stores. It refuses what net/mail refuses, and an address that net/mail
// accepts once but not again: a quoted local part such as "a b"@example.com
// parses to the bare a b@example.com, which no later parse accepts, so a row
// holding it could not be written into a response. The contract's email
// format validator and any importer that reads addresses from cells call it,
// so both admit the same set.
func ParseEmail(value string) (string, error) {
	parsed, err := mail.ParseAddress(value)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrEmail, err)
	}
	again, err := mail.ParseAddress(parsed.Address)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrEmail, err)
	}
	if again.Address != parsed.Address {
		return "", ErrEmail
	}
	return parsed.Address, nil
}
