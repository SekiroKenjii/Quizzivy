// Package email is the app's outgoing mail: the outbox a message is written to in the transaction that decides to send it, the templates it is rendered from when it is sent, and the senders that deliver it, Resend in production and a directory of JSON files elsewhere.
package email

import (
	"context"
	"errors"
	"strings"
)

// Message is one rendered email, ready for a Sender. IdempotencyKey is the
// outbox row's id, so a resend after a crash is delivered once.
type Message struct {
	To             string
	Subject        string
	HTML           string
	Text           string
	ReplyTo        string
	IdempotencyKey string
	Tags           map[string]string
}

// Sender delivers a Message and answers the provider's id for it. Every error
// it returns is a *SendError.
type Sender interface {
	Send(ctx context.Context, m Message) (providerID string, err error)
}

// The two kinds of send failure. A transient one is tried again later; a
// permanent one never is.
var (
	ErrTransient = errors.New("email: the send failed and may succeed later")
	ErrPermanent = errors.New("email: the send was refused")
)

// SendError is a failed send. Kind is ErrTransient or ErrPermanent and Unwrap
// exposes it. Code is a short identifier fit for the outbox and the logs, such
// as "http_429" or "network"; it never carries an address, a subject or a key.
type SendError struct {
	Kind error
	Code string
}

func (e *SendError) Error() string { return e.Kind.Error() + ": " + e.Code }

func (e *SendError) Unwrap() error { return e.Kind }

// Domain is the part of address after its last "@", the only part of a
// recipient the logs carry.
func Domain(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(address[at+1:])
}
