package httpx

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// StatusClientClosedRequest is nginx's 499: the status of a request whose
// client went away before the server answered, as a browser does when it
// aborts a fetch on navigation. It is not a server error, and nothing that
// counts 5xx counts it.
const StatusClientClosedRequest = 499

// ClientGone reports whether err is the client ending the request: ctx, the
// request's own context, is cancelled, and err is a context.Canceled or a
// network timeout. The second is how a database driver reports a cancel that
// reaches it while it writes: it expires the connection's deadline to
// interrupt the write, and the error that returns names the timeout, not the
// cancel. A context.DeadlineExceeded, a cancellation of a context the server
// derived while the request is live, a timeout on a live request and any other
// failure are the server's problem and answer as one.
func ClientGone(ctx context.Context, err error) bool {
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	var network net.Error
	return errors.As(err, &network) && network.Timeout() && !errors.Is(err, context.DeadlineExceeded)
}

// WriteClientGone answers a request whose client has gone with
// StatusClientClosedRequest and no body: nobody reads it, and the envelope has
// no code for a failure that is not the server's.
func WriteClientGone(w http.ResponseWriter) {
	w.WriteHeader(StatusClientClosedRequest)
}
