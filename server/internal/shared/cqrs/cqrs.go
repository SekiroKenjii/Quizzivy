package cqrs

import "context"

// CommandHandler runs one command — a request to change the system — and
// answers with what the change produced.
type CommandHandler[C any, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// QueryHandler answers one query without changing anything.
type QueryHandler[Q any, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}

// HandlerFunc adapts a function to either handler interface.
type HandlerFunc[I any, R any] func(ctx context.Context, in I) (R, error)

func (f HandlerFunc[I, R]) Handle(ctx context.Context, in I) (R, error) { return f(ctx, in) }

// Nothing is the result of a command that only succeeds or fails.
type Nothing struct{}

// Err keeps only the error of a handler call whose result the caller has no use for.
func Err[R any](_ R, err error) error {
	return err
}
