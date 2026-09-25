//go:build integration

package repositories_test

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"os/exec"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"
)

func TestKilledWorkerCanBeReclaimedWithoutLosingSources(t *testing.T) {
	if os.Getenv("WORD_QUEUE_CHILD") == "1" {
		claimAndWait(t)
		return
	}
	h := setup(t)
	version := uuid.NewString()
	scheduled := h.schedule(t, version, 3)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestKilledWorkerCanBeReclaimedWithoutLosingSources$")
	child.Env = append(os.Environ(), "WORD_QUEUE_CHILD=1", "WORD_QUEUE_PIPELINE="+version)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if child.Process != nil {
			_ = child.Process.Kill()
		}
	})
	ready := bufio.NewScanner(stdout)
	if !ready.Scan() || ready.Text() != "claimed" {
		_ = child.Wait()
		t.Fatal("child did not acquire its durable claim")
	}
	claimed, err := h.repo.Run(context.Background(), scheduled.ImportID, scheduled.ID)
	if err != nil || claimed.Status != "running" {
		t.Fatalf("claim was not committed: %v", err)
	}
	readCtx, stop := context.WithTimeout(context.Background(), time.Second)
	parent, err := h.repo.Get(readCtx, scheduled.ImportID)
	stop()
	if err != nil || len(parent.Sources) != 1 {
		t.Fatalf("processing held a database transaction or lost input: %v", err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("child was not killed")
	}
	h.expire(t, claimed.ID)
	replacement := h.claim(t, policy(version))
	if replacement.ClaimToken <= claimed.ClaimToken || replacement.SourceRevision != claimed.SourceRevision || replacement.ID != claimed.ID {
		t.Fatal("restart did not preserve work identity")
	}
	h.rejectLateWrites(t, claimed.Claim())
}

func claimAndWait(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("TEST_APP_DATABASE_URL")
	if dsn == "" {
		dsn = db.TestDSN(t)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := repositories.NewPostgres(db.NewContext(pool))
	p := policy(os.Getenv("WORD_QUEUE_PIPELINE"))
	if _, err := repo.Claim(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stdout, "claimed"); err != nil {
		t.Fatal(err)
	}
	<-context.Background().Done()
}

var _ domain.Queue = (*repositories.Postgres)(nil)
