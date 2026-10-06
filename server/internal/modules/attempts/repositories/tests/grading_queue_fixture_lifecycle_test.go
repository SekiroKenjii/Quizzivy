//go:build integration

package repositories_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"os/exec"
	"quizzivy/internal/modules/attempts/domain"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGradingQueueFixtureLifecycleChild(t *testing.T) {
	mode := os.Getenv("QUEUE_LIFECYCLE_CHILD")
	if mode == "" {
		return
	}
	f := newQueueFixture(t)
	var exists bool
	if err := f.tx.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM app.users WHERE id=$1)`, f.teacher).Scan(&exists); err != nil || !exists {
		t.Fatalf("lifecycle fixture user prerequisite=%t,%v", exists, err)
	}
	if mode == "partial_setup" || mode == "unrelated_sql" {
		id := f.id("tests")
		f.exec(`INSERT INTO app.tests(id,title,created_by,owner_id) VALUES($1,'Partial fixture',$2,$2)`, id, f.teacher)
		var value string
		err := f.tx.QueryRow(f.ctx, `SELECT queue_fixture_missing_column FROM app.tests WHERE id=$1`, id).Scan(&value)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42703" {
			t.Fatalf("lifecycle expected actual SQL42703 prerequisite=%v", err)
		}
		if mode == "unrelated_sql" {
			t.Fatalf("QUEUE_UNRELATED_SQL code=%s: %v", pg.Code, err)
		}
		queueLifecycleStage(t, f, mode)
		t.Fatal("QUEUE_INTENDED_PARTIAL_SETUP SQLSTATE42703")
	}
	p := f.paper(f.teacher, "Lifecycle", "short_answer")
	at := f.attempt(p, f.students[0], "submitted", queueTime(1), 1)
	f.answer(at, p.questions[0], true, nil)
	out := f.read(domain.GradingQueueQuery{Scope: f.own()})
	if out.AnswersRemaining != 1 || len(out.Items) != 1 || out.Items[0].AttemptID != at {
		t.Fatalf("lifecycle public queue prerequisite=%+v", out)
	}
	switch mode {
	case "normal":
		queueLifecycleStage(t, f, mode)
	case "fatal":
		queueLifecycleStage(t, f, mode)
		t.Fatal("QUEUE_INTENDED_FATAL_AFTER_PUBLIC_READ")
	case "cancel":
		f.cancel()
		_, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own()})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lifecycle actual canceled public read=%v", err)
		}
		queueLifecycleStage(t, f, mode)
		t.Fatal("QUEUE_INTENDED_CANCEL_AFTER_PUBLIC_READ")
	default:
		t.Fatalf("unknown lifecycle mode=%q", mode)
	}
}

func queueLifecycleStage(t *testing.T, f *queueFixture, mode string) {
	t.Helper()
	raw, err := json.Marshal(f.owned)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("QUEUE_REACHED_STAGE %s %s\n", mode, raw)
}

func TestGradingQueueFixtureLifecycle(t *testing.T) {
	for _, mode := range []string{"normal", "fatal", "partial_setup", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			output, err := runQueueLifecycle(t, mode)
			if verifyQueueLifecycle(output, err, mode) != nil {
				t.Fatalf("strict lifecycle proof rejected: %v\n%s", verifyQueueLifecycle(output, err, mode), output)
			}
		})
	}
}

func TestGradingQueueFixtureVerifierRejectsUnrelatedSQLFailure(t *testing.T) {
	output, err := runQueueLifecycle(t, "unrelated_sql")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("negative did not fail through actual SQL: %v\n%s", err, output)
	}
	if !strings.Contains(output, "QUEUE_UNRELATED_SQL code=42703") || !strings.Contains(output, "QUEUE_ROLLBACK_VERIFIED ") || !strings.Contains(output, "QUEUE_POOLS_CLOSED") {
		t.Fatalf("negative lacked actual SQL failure and successful cleanup:\n%s", output)
	}
	if err := verifyQueueLifecycle(output, err, "fatal"); err == nil {
		t.Fatalf("unrelated actual SQL failure falsely counted as intended Fatal:\n%s", output)
	}
}

func runQueueLifecycle(t *testing.T, mode string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGradingQueueFixtureLifecycleChild$", "-test.v", "-test.timeout=35s")
	cmd.Env = append(os.Environ(), "QUEUE_LIFECYCLE_CHILD="+mode)
	raw, err := cmd.CombinedOutput()
	output := string(raw)
	t.Logf("ACTUAL_QUEUE_CHILD mode=%s\n%s", mode, output)
	if ctx.Err() != nil {
		t.Fatalf("lifecycle child deadline: %v\n%s", ctx.Err(), output)
	}
	return output, err
}

func verifyQueueLifecycle(output string, childErr error, mode string) error {
	if strings.Contains(output, "panic:") || strings.Contains(output, "test timed out") || strings.Contains(output, "QUEUE_UNRELATED_SQL") {
		return errors.New("unexpected child failure")
	}
	if mode == "normal" {
		if childErr != nil {
			return fmt.Errorf("normal child: %w", childErr)
		}
	} else {
		var exit *exec.ExitError
		if !errors.As(childErr, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("expected exact exit1 for %s: %v", mode, childErr)
		}
	}
	signature := map[string]string{"fatal": "QUEUE_INTENDED_FATAL_AFTER_PUBLIC_READ", "partial_setup": "QUEUE_INTENDED_PARTIAL_SETUP SQLSTATE42703", "cancel": "QUEUE_INTENDED_CANCEL_AFTER_PUBLIC_READ"}[mode]
	if mode != "normal" && (signature == "" || strings.Count(output, signature) != 1) {
		return errors.New("missing unique intended branch failure")
	}
	stage, err := queueLifecycleOwnership(output, "QUEUE_REACHED_STAGE "+mode+" ")
	if err != nil {
		return err
	}
	cleanup, err := queueLifecycleOwnership(output, "QUEUE_ROLLBACK_VERIFIED ")
	if err != nil {
		return err
	}
	if len(stage["users"]) != 5 || !reflect.DeepEqual(stage, cleanup) || strings.Count(output, "QUEUE_POOLS_CLOSED") != 1 {
		return errors.New("actual reached identities/rollback/pool close proof mismatch")
	}
	if mode == "partial_setup" {
		if len(stage["tests"]) != 1 || len(stage["attempts"]) != 0 {
			return errors.New("partial setup did not reach the expected actual row boundary")
		}
	} else if len(stage["attempts"]) != 1 || len(stage["test_version_questions"]) != 1 {
		return errors.New("public read prerequisites did not reach actual rows")
	}
	return nil
}

func queueLifecycleOwnership(output, prefix string) (map[string][]string, error) {
	var out map[string][]string
	matches := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		start := strings.Index(line, prefix)
		if start < 0 {
			continue
		}
		matches++
		if err := json.Unmarshal([]byte(line[start+len(prefix):]), &out); err != nil {
			return nil, fmt.Errorf("malformed ownership proof: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if matches != 1 || out == nil {
		return nil, fmt.Errorf("expected one %q ownership proof, got %d", prefix, matches)
	}
	return out, nil
}
