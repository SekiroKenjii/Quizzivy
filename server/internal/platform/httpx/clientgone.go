package httpx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
)

// StatusClientClosedRequest is nginx's 499: the status of a request whose
// client went away before the server answered, as a browser does when it
// aborts a fetch on navigation. It is not a server error, and nothing that
// counts 5xx counts it.
const StatusClientClosedRequest = 499

// ClientGone reports whether err is the client ending the request, in one of
// two ways. Either ctx, the request's own context, is cancelled and err is a
// context.Canceled or a network timeout (the second is how a database driver
// reports a cancel that reaches it while it writes: it expires the
// connection's deadline to interrupt the write, and the error that returns
// names the timeout, not the cancel); or err is the failure of writing the
// response and the connection is what failed, a broken pipe, a reset by the
// peer or a closed connection. A write meets a lost peer before the server has
// seen the close, so the context may still be live; only the error the
// response writer returned counts, as Logging recorded it, because the same
// error from the database connection is not the client's. A
// context.DeadlineExceeded, a cancellation of a context the server derived
// while the request is live, a timeout on a live request, a write that timed
// out and any other failure are the server's problem and answer as one.
func ClientGone(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if lostResponse(ctx, err) {
		return true
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	var network net.Error
	return errors.As(err, &network) && network.Timeout() && !errors.Is(err, context.DeadlineExceeded)
}

func lostResponse(ctx context.Context, err error) bool {
	rec, ok := ctx.Value(recorderKey{}).(*statusRecorder)
	return ok && rec.writeErr != nil && errors.Is(err, rec.writeErr) && connectionLost(rec.writeErr)
}

func connectionLost(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, net.ErrClosed)
}

// WriteClientGone answers a request whose client has gone with
// StatusClientClosedRequest and no body: nobody reads it, and the envelope has
// no code for a failure that is not the server's.
func WriteClientGone(w http.ResponseWriter) {
	w.WriteHeader(StatusClientClosedRequest)
}
