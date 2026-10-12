package maintenance_test

import (
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/maintenance"
)

func TestTheAICommandsAreInTheUsage(t *testing.T) {
	_, err := maintenance.Parse([]string{"sweep-everything"})
	if err == nil || !strings.Contains(err.Error(), "ai-credits") || !strings.Contains(err.Error(), "ai-reservations-sweep") {
		t.Errorf("the usage line is %v, want it to name the AI commands", err)
	}
}

func TestEveryAICreditsSubcommandParsesItsFlagsAndTakesATimeout(t *testing.T) {
	const user, request = "0198f5f0-0000-7000-8000-000000000001", "0198f5f0-0000-7000-8000-0000000000aa"
	for _, args := range [][]string{
		{"ai-credits", "status"},
		{"ai-credits", "set-allowance", "-user", user, "-credits", "30000", "-operator", "thuong"},
		{"ai-credits", "set-allowance", "-user", user, "-default", "-operator", "thuong"},
		{"ai-credits", "pause", "-user", user, "-operator", "thuong"},
		{"ai-credits", "resume", "-user", user, "-operator", "thuong", "-apply"},
		{"ai-credits", "add-credits", "-credits", "50000", "-operator", "thuong"},
		{"ai-credits", "approve", "-request", request, "-operator", "thuong"},
		{"ai-credits", "decline", "-request", request, "-operator", "thuong"},
		{"ai-credits", "set-rules", "-warn-percent", "90", "-operator", "thuong"},
		{"ai-reservations-sweep"},
		{"ai-reservations-sweep", "-apply"},
	} {
		command := parse(t, args...)
		if command.Timeout != time.Minute {
			t.Errorf("%v: default timeout %v", args, command.Timeout)
		}
		if got := parse(t, append(args, "-timeout", "9s")...).Timeout; got != 9*time.Second {
			t.Errorf("%v: -timeout 9s = %v", args, got)
		}
	}
	if got := parse(t, "ai-credits", "status").Name; got != "ai-credits status" {
		t.Errorf("name %q", got)
	}
}

func TestAnAICreditsCommandIsRefusedBeforeItTouchesTheDatabase(t *testing.T) {
	const user = "0198f5f0-0000-7000-8000-000000000001"
	for name, c := range map[string]struct {
		args []string
		want string
	}{
		"no subcommand":              {[]string{"ai-credits"}, "name a subcommand"},
		"an unknown subcommand":      {[]string{"ai-credits", "refill"}, `unknown subcommand "refill"`},
		"a write with no operator":   {[]string{"ai-credits", "pause", "-user", user}, "-operator is required"},
		"an operator of spaces":      {[]string{"ai-credits", "pause", "-user", user, "-operator", "   "}, "-operator is required"},
		"an operator too long":       {[]string{"ai-credits", "pause", "-user", user, "-operator", strings.Repeat("x", 101)}, "-operator is required"},
		"set-allowance with neither": {[]string{"ai-credits", "set-allowance", "-user", user, "-operator", "t"}, "exactly one of -credits N and -default"},
		"set-allowance with both":    {[]string{"ai-credits", "set-allowance", "-user", user, "-credits", "5000", "-default", "-operator", "t"}, "exactly one of -credits N and -default"},
		"status with an operator":    {[]string{"ai-credits", "status", "-operator", "t"}, "flag provided but not defined"},
		"status with apply":          {[]string{"ai-credits", "status", "-apply"}, "flag provided but not defined"},
		"a stray argument":           {[]string{"ai-credits", "status", "extra"}, `unexpected argument "extra"`},
		"a zero timeout":             {[]string{"ai-credits", "status", "-timeout", "0s"}, "-timeout must be positive"},
		"a sweep with an operator":   {[]string{"ai-reservations-sweep", "-operator", "t"}, "flag provided but not defined"},
		"a bogus flag":               {[]string{"ai-credits", "add-credits", "-bogus", "1"}, "flag provided but not defined"},
	} {
		_, err := maintenance.Parse(c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, c.want)
			continue
		}
		if !strings.Contains(err.Error(), "usage: maintenance <command> [flags]") {
			t.Errorf("%s: %q does not end with the usage line", name, err)
		}
	}
}
