package httpx

import (
	"context"
	"errors"
	"net/http"
)

// StatusClientClosedRequest is nginx's 499: the status of a request whose
// client went away before the server answered, as a browser does when it
// aborts a fetch on navigation. It is not a server error, and nothing that
// counts 5xx counts it.
const StatusClientClosedRequest = 499

// ClientGone reports whether err is the client ending the request: err carries
// context.Canceled and ctx, the request's own context, is cancelled. A
// cancellation of a context the server derived while the request is live, a
// deadline of any context, and any other failure on a request that happens to
// be cancelled are the server's problem and answer as one.
func ClientGone(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled)
}

// WriteClientGone answers a request whose client has gone with
// StatusClientClosedRequest and no body: nobody reads it, and the envelope has
// no code for a failure that is not the server's.
func WriteClientGone(w http.ResponseWriter) {
	w.WriteHeader(StatusClientClosedRequest)
}
