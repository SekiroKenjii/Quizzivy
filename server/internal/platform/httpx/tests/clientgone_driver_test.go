//go:build integration

package httpx_test

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/httpx"
)

type stallingProxy struct {
	listener net.Listener
	upstream string
	stalled  atomic.Bool
	release  chan struct{}
	mu       sync.Mutex
	conns    []net.Conn
}

func newStallingProxy(t *testing.T, upstream string) *stallingProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &stallingProxy{listener: listener, upstream: upstream, release: make(chan struct{})}
	t.Cleanup(p.close)
	go p.accept()
	return p
}

func (p *stallingProxy) close() {
	close(p.release)
	_ = p.listener.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		_ = conn.Close()
	}
}

func (p *stallingProxy) accept() {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}
		server, err := net.Dial("tcp", p.upstream)
		if err != nil {
			_ = client.Close()
			return
		}
		p.mu.Lock()
		p.conns = append(p.conns, client, server)
		p.mu.Unlock()
		go func() { _, _ = io.Copy(client, server) }()
		go p.forward(server, client)
	}
}

func (p *stallingProxy) forward(server, client net.Conn) {
	buf := make([]byte, 32<<10)
	for {
		if p.stalled.Load() {
			<-p.release
			return
		}
		n, err := client.Read(buf)
		if n > 0 {
			if _, werr := server.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *stallingProxy) stall() { p.stalled.Store(true) }

func TestAClientCancelThatInterruptsADatabaseWriteIsStillTheClients(t *testing.T) {
	config, err := pgx.ParseConfig(db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	proxy := newStallingProxy(t, net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port))))
	host, port, err := net.SplitHostPort(proxy.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	config.Host, config.Port = host, uint16(portNumber)
	for _, fallback := range config.Fallbacks {
		fallback.Host, fallback.Port = host, uint16(portNumber)
	}
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closing, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = conn.Close(closing)
	})
	if _, err := conn.Exec(context.Background(), "SELECT length($1::text)", "warm"); err != nil {
		t.Fatal(err)
	}
	proxy.stall()

	payload := strings.Repeat("x", 64<<20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(300*time.Millisecond, cancel)
	_, err = conn.Exec(ctx, "SELECT length($1::text)", payload)

	if err == nil {
		t.Fatal("a query whose connection never drained returned no error")
	}
	t.Logf("the driver reported %T: %v", err, err)
	if !httpx.ClientGone(ctx, err) {
		t.Errorf("ClientGone = false for %v on a cancelled request: the API would answer 500 and log an error for the client's own cancellation", err)
	}
	if httpx.ClientGone(context.Background(), err) {
		t.Errorf("ClientGone = true for %v on a request that was never cancelled", err)
	}
}
