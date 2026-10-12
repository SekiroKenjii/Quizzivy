package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"

	"quizzivy/internal/platform/httpx"
)

type recordingWriter struct {
	http.ResponseWriter
	headers  []int
	writeErr error
}

func (w *recordingWriter) WriteHeader(code int) {
	w.headers = append(w.headers, code)
	w.ResponseWriter.WriteHeader(code)
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseWriter.Write(p)
}

func lostPeer(op string, errno error) error {
	return &net.OpError{Op: op, Net: "tcp", Err: os.NewSyscallError(op, errno)}
}

func loggedStatus(t *testing.T, logs *bytes.Buffer) int {
	t.Helper()
	var line struct {
		Msg    string `json:"msg"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &line); err != nil || line.Msg != "http" {
		t.Fatalf("log = %q (%v), want one access line", logs.String(), err)
	}
	return line.Status
}

func serveLogged(w http.ResponseWriter, handler http.HandlerFunc) *bytes.Buffer {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	httpx.Logging(logger)(handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/teacher/questions", nil))
	return logs
}

func TestClientGoneIsAFailedWriteOfTheResponseToALostPeer(t *testing.T) {
	same := func(written error) error { return fmt.Errorf("encode: %w", written) }
	for _, c := range []struct {
		name     string
		writeErr error
		failure  func(written error) error
		want     bool
	}{
		{"a write to a peer that reset the connection", lostPeer("write", syscall.ECONNRESET), same, true},
		{"a write to a peer that closed the connection", lostPeer("write", syscall.EPIPE), same, true},
		{"a write to a connection that was closed", &net.OpError{Op: "write", Net: "tcp", Err: net.ErrClosed}, same, true},
		{"the failure itself, unwrapped", lostPeer("write", syscall.ECONNRESET), func(written error) error { return written }, true},
		{"a write that timed out", &net.OpError{Op: "write", Net: "tcp", Err: os.ErrDeadlineExceeded}, same, false},
		{"a write that was cut short", io.ErrShortWrite, same, false},
		{"a reset the database reported while the response's write failed another way", io.ErrShortWrite,
			func(error) error { return lostPeer("read", syscall.ECONNRESET) }, false},
		{"a reset the database reported while the response's write was failing too", lostPeer("write", syscall.ECONNRESET),
			func(error) error { return lostPeer("read", syscall.ECONNRESET) }, false},
		{"a reset the database reported when the response's write did not fail", nil,
			func(error) error { return lostPeer("read", syscall.ECONNRESET) }, false},
	} {
		var got bool
		serveLogged(&recordingWriter{ResponseWriter: httptest.NewRecorder(), writeErr: c.writeErr}, func(w http.ResponseWriter, r *http.Request) {
			var written error
			if c.writeErr != nil {
				_, written = w.Write([]byte("{}"))
			}
			got = httpx.ClientGone(r.Context(), c.failure(written))
		})
		if got != c.want {
			t.Errorf("%s: ClientGone = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestClientGoneNeedsTheLoggerToHaveSeenTheWrite(t *testing.T) {
	if httpx.ClientGone(context.Background(), lostPeer("write", syscall.ECONNRESET)) {
		t.Error("a reset with no recorded write of the response was taken for the client's")
	}
}

func TestALoggedRequestAnsweredAfterItsResponseBeganReads499AndSendsOneStatus(t *testing.T) {
	const gone = httpx.StatusClientClosedRequest
	for _, c := range []struct {
		name   string
		serve  func(w http.ResponseWriter)
		sent   []int
		logged int
	}{
		{"the body began without a status", func(w http.ResponseWriter) {
			_, _ = w.Write([]byte("{}"))
			httpx.WriteClientGone(w)
		}, nil, gone},
		{"the status went out, then the body, then the client was gone", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			httpx.WriteClientGone(w)
		}, []int{http.StatusOK}, gone},
		{"nothing was written yet", func(w http.ResponseWriter) {
			httpx.WriteClientGone(w)
		}, []int{gone}, gone},
		{"an informational status came first", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusEarlyHints)
			w.WriteHeader(http.StatusOK)
		}, []int{http.StatusEarlyHints, http.StatusOK}, http.StatusOK},
	} {
		writer := &recordingWriter{ResponseWriter: httptest.NewRecorder()}
		logs := serveLogged(writer, func(w http.ResponseWriter, _ *http.Request) { c.serve(w) })

		if fmt.Sprint(writer.headers) != fmt.Sprint(c.sent) {
			t.Errorf("%s: the connection was sent statuses %v, want %v", c.name, writer.headers, c.sent)
		}
		if got := loggedStatus(t, logs); got != c.logged {
			t.Errorf("%s: the access line reads %d, want %d", c.name, got, c.logged)
		}
	}
}
