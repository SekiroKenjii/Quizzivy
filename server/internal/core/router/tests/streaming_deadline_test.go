package router_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/cqrs"
)

type upload struct {
	read int
	err  error
}

func TestAStreamedUploadMayTakeLongerThanTheServersReadTimeoutThroughTheWholeChain(t *testing.T) {
	const serverReadTimeout = 400 * time.Millisecond
	const pause = 1200 * time.Millisecond
	first, second := bytes.Repeat([]byte("a"), 4096), bytes.Repeat([]byte("b"), 4096)
	got := make(chan upload, 1)
	setAvatar := func(_ context.Context, cmd command.SetAvatar) (domain.User, error) {
		raw, err := io.ReadAll(cmd.Body)
		got <- upload{read: len(raw), err: err}
		return domain.User{ID: teacherUser, Email: "lan@example.com", FullName: "Lan"}, err
	}
	app := &application.Application{Commands: application.Commands{SetAvatar: cqrs.HandlerFunc[command.SetAvatar, domain.User](setAvatar)}}
	issuer := testIssuer(t)
	h, _ := debugLoggedRouter(t, router.Deps{
		DB:      fakeDB{},
		Tokens:  issuer,
		Modules: router.Modules{Identity: identityhttp.NewIdentity(app, time.Hour, true, nil)},
	})
	token, err := issuer.Issue(teacherUser, 0)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(h)
	server.Config.ReadTimeout = serverReadTimeout
	server.Start()
	t.Cleanup(server.Close)

	head := []byte("--qv\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\nContent-Type: image/png\r\n\r\n")
	tail := []byte("\r\n--qv--\r\n")
	length := len(head) + len(first) + len(second) + len(tail)
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "PUT /me/avatar HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nContent-Type: multipart/form-data; boundary=qv\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
		server.Listener.Addr(), token, length); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(append([]byte{}, head...), first...)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(pause)
	if _, err := conn.Write(append(append([]byte{}, second...), tail...)); err != nil {
		t.Fatal(err)
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no answer: %v", err)
	}
	_ = response.Body.Close()
	select {
	case read := <-got:
		if read.err != nil || read.read != len(first)+len(second) {
			t.Errorf("the handler read %d bytes with error %v, want all %d: the body outlived the server's %s read timeout and must not be cut", read.read, read.err, len(first)+len(second), serverReadTimeout)
		}
	case <-time.After(5 * time.Second):
		t.Error("the upload never reached the handler")
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}
}
