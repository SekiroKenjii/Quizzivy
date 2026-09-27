package maintenance_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/maintenance"
	"quizzivy/internal/platform/db"
)

func parse(t *testing.T, args ...string) maintenance.Command {
	t.Helper()
	command, err := maintenance.Parse(args)
	if err != nil {
		t.Fatalf("Parse(%q): %v", args, err)
	}
	return command
}

func runError(t *testing.T, command maintenance.Command) string {
	t.Helper()
	_, err := command.Run(context.Background(), nil)
	if err == nil {
		t.Fatalf("%s ran without refusing its flags", command.Name)
	}
	return err.Error()
}

func TestFlagsAfterTheCommandAreParsed(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		refuse string
	}{
		{[]string{"retain-integrity", "-apply", "-batch", "0"}, "batch must be between"},
		{[]string{"anonymize-student", "-apply", "-student", "not-a-uuid"}, "student ID must be a UUID"},
		{[]string{"window-schedule", "-start", "2026-10-01T16:00:00Z", "-end", "2026-10-01T15:00:00Z", "-apply"}, "must end after it starts"},
		{[]string{"window-schedule", "-start", "2026-10-01T15:00:00Z", "-end", "2026-10-02T15:00:00Z"}, "at most 12 hours"},
		{[]string{"window-cancel", "-window", "not-a-uuid", "-apply"}, "-window must be a window UUID"},
		{[]string{"window-end", "-window", "not-a-uuid"}, "-window must be a window UUID"},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			if got := runError(t, parse(t, tc.args...)); !strings.Contains(got, tc.refuse) {
				t.Errorf("error = %q, want it to mention %q", got, tc.refuse)
			}
		})
	}
}

func TestEveryCommandTakesATimeout(t *testing.T) {
	for _, name := range []string{"retain-integrity", "anonymize-student", "window-list", "window-cancel", "window-end"} {
		if got := parse(t, name, "-timeout", "7s").Timeout; got != 7*time.Second {
			t.Errorf("%s -timeout 7s = %v", name, got)
		}
		if got := parse(t, name).Timeout; got != time.Minute {
			t.Errorf("%s default timeout = %v, want a minute", name, got)
		}
	}
	command := parse(t, "window-schedule", "-start", "2026-10-01T15:00:00Z", "-end", "2026-10-01T16:00:00Z", "-timeout", "90s")
	if command.Timeout != 90*time.Second {
		t.Errorf("window-schedule -timeout 90s = %v", command.Timeout)
	}
}

func TestTheTimeoutReachesTheContext(t *testing.T) {
	command := parse(t, "window-list", "-timeout", "7s")
	var deadline time.Time
	var ok bool
	command.Run = func(ctx context.Context, _ db.Conn) (any, error) {
		deadline, ok = ctx.Deadline()
		return nil, nil
	}
	before := time.Now()
	if _, err := command.Execute(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the command ran without a deadline")
	}
	if left := deadline.Sub(before); left < 6*time.Second || left > 8*time.Second {
		t.Errorf("deadline is %v away, want about 7s", left)
	}
}

func TestTheOldFlagsFirstOrderStillParses(t *testing.T) {
	retain := parse(t, "-apply", "-batch", "0", "retain-integrity")
	if retain.Name != "retain-integrity" {
		t.Fatalf("name = %q", retain.Name)
	}
	if got := runError(t, retain); !strings.Contains(got, "batch must be between") {
		t.Errorf("-batch before the command was not parsed: %q", got)
	}
	anonymize := parse(t, "-student", "not-a-uuid", "anonymize-student")
	if got := runError(t, anonymize); !strings.Contains(got, "student ID must be a UUID") {
		t.Errorf("-student before the command was not parsed: %q", got)
	}
	if got := parse(t, "-timeout", "5s", "retain-integrity").Timeout; got != 5*time.Second {
		t.Errorf("-timeout before the command = %v", got)
	}
}

func TestUnknownCommandsAndFlagsPrintTheUsage(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"sweep-everything"},
		{"window-list", "-apply"},
		{"window-schedule", "-start"},
		{"retain-integrity", "extra"},
		{"-bogus", "retain-integrity"},
		{"-apply", "window-list"},
		{"window-list", "-timeout", "0s"},
		{"window-schedule", "-start", "tomorrow", "-end", "2026-10-01T16:00:00Z"},
	} {
		_, err := maintenance.Parse(args)
		if err == nil {
			t.Errorf("Parse(%q) accepted it", args)
			continue
		}
		if !strings.Contains(err.Error(), "usage: maintenance <command> [flags]") {
			t.Errorf("Parse(%q) = %q, want the usage line", args, err)
		}
	}
}
