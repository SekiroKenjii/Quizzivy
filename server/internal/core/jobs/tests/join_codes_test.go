package jobs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
	classesapp "quizzivy/internal/modules/classes/application"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/cqrs"
)

type rotationRun struct {
	calls  atomic.Int32
	budget time.Duration
}

func (r *rotationRun) app(outcome func(classescommand.RotateLegacyJoinCodes) (classesdomain.LegacyRotation, error)) *classesapp.Application {
	return &classesapp.Application{Commands: classesapp.Commands{
		RotateLegacyJoinCodes: cqrs.HandlerFunc[classescommand.RotateLegacyJoinCodes, classesdomain.LegacyRotation](
			func(ctx context.Context, cmd classescommand.RotateLegacyJoinCodes) (classesdomain.LegacyRotation, error) {
				r.calls.Add(1)
				if deadline, bounded := ctx.Deadline(); bounded {
					r.budget = time.Until(deadline)
				}
				return outcome(cmd)
			}),
	}}
}

func runTheRotation(t *testing.T, app *classesapp.Application) []map[string]any {
	t.Helper()
	var logs bytes.Buffer
	done := make(chan struct{})
	go func() {
		jobs.RotateLegacyJoinCodes(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)), app)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the rotation did not return although the process is still running: it must run once, never on a timer")
	}
	var lines []map[string]any
	for line := range strings.Lines(logs.String()) {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		lines = append(lines, entry)
	}
	return lines
}

func logged(lines []map[string]any, msg string) []map[string]any {
	var found []map[string]any
	for _, line := range lines {
		if line["msg"] == msg {
			found = append(found, line)
		}
	}
	return found
}

func TestTheLegacyRotationRunsOnceUnderTwoMinutesAndLogsItsCounts(t *testing.T) {
	var run rotationRun
	lines := runTheRotation(t, run.app(func(cmd classescommand.RotateLegacyJoinCodes) (classesdomain.LegacyRotation, error) {
		cmd.Found(7)
		cmd.Problem(errors.New("rotate the legacy join code of class 0195a000-0000-7000-8000-000000000001: the database is away"))
		return classesdomain.LegacyRotation{Found: 7, Rotated: 5, Failed: 1, Teachers: 2, NotifyFailed: 3}, nil
	}))

	if n := run.calls.Load(); n != 1 {
		t.Errorf("the command ran %d times, want once", n)
	}
	if run.budget <= 110*time.Second || run.budget > 2*time.Minute {
		t.Errorf("the run had a budget of %s, want about two minutes", run.budget)
	}

	var order []string
	for _, line := range lines {
		order = append(order, line["msg"].(string))
		for key := range line {
			if !slices.Contains([]string{"time", "level", "msg", "count", "found", "rotated", "failed", "teachers", "notify_failed", "err"}, key) {
				t.Errorf("the line %v carries %q: the job logs counts and errors, never a code or a hint", line, key)
			}
		}
	}
	if !slices.Equal(order, []string{"legacy_join_codes_found", "legacy_join_codes_problem", "legacy_join_codes_rotated"}) {
		t.Fatalf("the job logged %v", order)
	}
	if found := lines[0]; found["level"] != "INFO" || found["count"] != float64(7) {
		t.Errorf("found: %v, want the count 7 at info", found)
	}
	if problem := lines[1]; problem["level"] != "WARN" || !strings.Contains(problem["err"].(string), "0195a000-0000-7000-8000-000000000001") {
		t.Errorf("problem: %v, want a warning naming the class", problem)
	}
	rotated := lines[2]
	for key, want := range map[string]float64{"found": 7, "rotated": 5, "failed": 1, "teachers": 2, "notify_failed": 3} {
		if rotated[key] != want {
			t.Errorf("rotated: %s = %v, want %v", key, rotated[key], want)
		}
	}
	if rotated["level"] != "INFO" {
		t.Errorf("rotated: level %v, want INFO", rotated["level"])
	}
}

func TestALegacyRotationThatFindsNothingLogsNothing(t *testing.T) {
	var run rotationRun
	lines := runTheRotation(t, run.app(func(cmd classescommand.RotateLegacyJoinCodes) (classesdomain.LegacyRotation, error) {
		cmd.Found(0)
		return classesdomain.LegacyRotation{}, nil
	}))

	if n := run.calls.Load(); n != 1 {
		t.Errorf("the command ran %d times, want once", n)
	}
	if len(lines) != 0 {
		t.Errorf("a run that found nothing logged %v", lines)
	}
}

func TestAFailedLegacyRotationIsAWarningAndTheJobReturns(t *testing.T) {
	var run rotationRun
	lines := runTheRotation(t, run.app(func(classescommand.RotateLegacyJoinCodes) (classesdomain.LegacyRotation, error) {
		return classesdomain.LegacyRotation{Found: 4, Rotated: 4}, errors.New("the database is away")
	}))

	if n := run.calls.Load(); n != 1 {
		t.Errorf("the command ran %d times, want once: a failed run waits for the next start-up", n)
	}
	failed := logged(lines, "legacy_join_codes_failed")
	if len(failed) != 1 || failed[0]["level"] != "WARN" || failed[0]["err"] != "the database is away" {
		t.Errorf("the job logged %v, want one warning with the error", lines)
	}
	if len(logged(lines, "legacy_join_codes_rotated")) != 0 {
		t.Errorf("a failed run logged counts: %v", lines)
	}
}
