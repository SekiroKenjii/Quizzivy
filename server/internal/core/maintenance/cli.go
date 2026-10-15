package maintenance

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

// Usage is the line printed when a command or a flag is not understood.
const Usage = "usage: maintenance <command> [flags]\n" +
	"commands: retention-sweep, retain-integrity, anonymize-user (alias anonymize-student), window-schedule, window-list, window-cancel, window-end,\n" +
	"  rekey-join-codes, ai-credits <status|set-allowance|add-credits|approve|decline|pause|resume|set-rules>, ai-reservations-sweep\n" +
	"every command takes -timeout <duration> (default 1m), retention-sweep -budget <duration> (default 1m) instead; writes are dry runs until -apply"

// Command is one parsed invocation: what to run and how long it may take.
// ReportOnError asks the caller to print the report Run returned even when it also returned an error, for a command whose batches commit one by one and so leave a partial result worth reading.
type Command struct {
	Name          string
	Timeout       time.Duration
	ReportOnError bool
	Run           func(ctx context.Context, conn db.Conn) (any, error)
}

// Execute runs the command with its timeout applied to ctx.
func (c Command) Execute(ctx context.Context, conn db.Conn) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	return c.Run(ctx, conn)
}

const (
	retentionSweep   = "retention-sweep"
	retainIntegrity  = "retain-integrity"
	anonymizeUser    = "anonymize-user"
	anonymizeStudent = "anonymize-student"
)

type runner func(ctx context.Context, conn db.Conn) (any, error)

type spec func(fs *flag.FlagSet) func() (runner, error)

var commands = map[string]spec{
	retainIntegrity: func(fs *flag.FlagSet) func() (runner, error) {
		apply := fs.Bool("apply", false, "delete the batch; omitted means dry run")
		batch := fs.Int("batch", 1000, "maximum integrity events in one batch (1..10000)")
		return func() (runner, error) {
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return retainConfigured(ctx, conn, *apply, *batch)
			}, nil
		}
	},
	anonymizeUser:    anonymizeSpec,
	anonymizeStudent: anonymizeSpec,
	"window-schedule": func(fs *flag.FlagSet) func() (runner, error) {
		apply := fs.Bool("apply", false, "schedule; omitted means dry run")
		start := fs.String("start", "", "start of the window, RFC 3339")
		end := fs.String("end", "", "end of the window, RFC 3339")
		return func() (runner, error) {
			startsAt, err := time.Parse(time.RFC3339, *start)
			if err != nil {
				return nil, fmt.Errorf("-start must be an RFC 3339 time: %w", err)
			}
			endsAt, err := time.Parse(time.RFC3339, *end)
			if err != nil {
				return nil, fmt.Errorf("-end must be an RFC 3339 time: %w", err)
			}
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return ScheduleWindow(ctx, conn, startsAt, endsAt, *apply)
			}, nil
		}
	},
	"window-list": func(*flag.FlagSet) func() (runner, error) {
		return func() (runner, error) {
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return ListWindows(ctx, conn)
			}, nil
		}
	},
	"window-cancel": func(fs *flag.FlagSet) func() (runner, error) {
		apply := fs.Bool("apply", false, "cancel; omitted means dry run")
		window := fs.String("window", "", "window UUID")
		return func() (runner, error) {
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return CancelWindow(ctx, conn, *window, *apply)
			}, nil
		}
	},
	"rekey-join-codes": func(fs *flag.FlagSet) func() (runner, error) {
		apply := fs.Bool("apply", false, "re-key; omitted means dry run")
		batch := fs.Int("batch", 500, "join codes re-keyed in one transaction (1..10000)")
		return func() (runner, error) {
			current, previous, err := config.JoinCodeKeys()
			if err != nil {
				return nil, err
			}
			if previous == nil {
				return nil, errors.New("JOIN_CODE_KEY_PREVIOUS is required: rekey-join-codes moves codes from the previous key to JOIN_CODE_KEY")
			}
			keys, err := classesdomain.NewJoinCodeKeys(current, previous)
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return RekeyJoinCodes(ctx, conn, keys, *apply, *batch)
			}, nil
		}
	},
	aiSweep: sweepSpec,
	"window-end": func(fs *flag.FlagSet) func() (runner, error) {
		apply := fs.Bool("apply", false, "end now; omitted means dry run")
		window := fs.String("window", "", "window UUID")
		return func() (runner, error) {
			return func(ctx context.Context, conn db.Conn) (any, error) {
				return EndWindow(ctx, conn, *window, *apply)
			}, nil
		}
	},
}

var legacyCommands = map[string]bool{retainIntegrity: true, anonymizeStudent: true}

func retainConfigured(ctx context.Context, conn db.Conn, apply bool, batch int) (RetentionReport, error) {
	if err := checkBatch(batch); err != nil {
		return RetentionReport{}, err
	}
	retention, err := ReadRetention(ctx, conn)
	if err != nil {
		return RetentionReport{}, err
	}
	return RetainEvents(ctx, conn, retention.KeepFocusEventsMonths, apply, batch)
}

func anonymizeSpec(fs *flag.FlagSet) func() (runner, error) {
	apply := fs.Bool("apply", false, "anonymize; omitted means dry run")
	user := fs.String("user", "", "user UUID")
	student := fs.String("student", "", "user UUID, the older name of -user")
	return func() (runner, error) {
		id, err := accountFlag(*user, *student)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, conn db.Conn) (any, error) {
			return AnonymizeUser(ctx, conn, id, *apply)
		}, nil
	}
}

func accountFlag(user, student string) (string, error) {
	if user != "" && student != "" && user != student {
		return "", errors.New("-user and -student name different accounts; use -user")
	}
	return cmp.Or(user, student), nil
}

func parseRetentionSweep(args []string) (Command, error) {
	fs := newFlagSet(retentionSweep)
	apply := fs.Bool("apply", false, "write; omitted means dry run")
	batch := fs.Int("batch", 500, "rows in one batch: integrity events, attempts or accounts (1..10000)")
	budget := fs.Duration("budget", time.Minute, "how long the sweep may run; it is also the command's timeout")
	if err := fs.Parse(args); err != nil {
		return Command{}, fmt.Errorf("%s: %w\n%s", retentionSweep, err, Usage)
	}
	if fs.NArg() > 0 {
		return Command{}, fmt.Errorf("%s: unexpected argument %q\n%s", retentionSweep, fs.Arg(0), Usage)
	}
	if *budget <= 0 {
		return Command{}, fmt.Errorf("%s: -budget must be positive\n%s", retentionSweep, Usage)
	}
	opts := SweepOptions{Apply: *apply, Batch: *batch, Budget: *budget}
	return Command{Name: retentionSweep, Timeout: *budget, ReportOnError: true, Run: func(ctx context.Context, conn db.Conn) (any, error) {
		report, err := RetentionSweep(ctx, conn, opts)
		if err != nil && report.DatabaseRole == "" {
			return nil, err
		}
		return report, err
	}}, nil
}

// Parse reads `maintenance <command> [flags]`. Each command owns its flags,
// so a flag written after the command is parsed. The older order,
// `maintenance [-apply] [-batch N] [-student UUID] <command>`, is still
// accepted for retain-integrity and anonymize-student.
func Parse(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, errors.New(Usage)
	}
	if strings.HasPrefix(args[0], "-") {
		return parseLegacy(args)
	}
	name := args[0]
	if name == aiCredits {
		return parseAICredits(args[1:])
	}
	if name == retentionSweep {
		return parseRetentionSweep(args[1:])
	}
	build, ok := commands[name]
	if !ok {
		return Command{}, fmt.Errorf("unknown command %q\n%s", name, Usage)
	}
	fs := newFlagSet(name)
	timeout := fs.Duration("timeout", time.Minute, "how long the command may run")
	finish := build(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return Command{}, fmt.Errorf("%s: %w\n%s", name, err, Usage)
	}
	if fs.NArg() > 0 {
		return Command{}, fmt.Errorf("%s: unexpected argument %q\n%s", name, fs.Arg(0), Usage)
	}
	return command(name, *timeout, finish)
}

func parseLegacy(args []string) (Command, error) {
	fs := newFlagSet("maintenance")
	apply := fs.Bool("apply", false, "apply changes; omitted means dry run")
	batch := fs.Int("batch", 1000, "maximum integrity events in one batch (1..10000)")
	student := fs.String("student", "", "student UUID for anonymize-student")
	timeout := fs.Duration("timeout", time.Minute, "how long the command may run")
	if err := fs.Parse(args); err != nil {
		return Command{}, fmt.Errorf("%w\n%s", err, Usage)
	}
	name := fs.Arg(0)
	if !legacyCommands[name] {
		return Command{}, fmt.Errorf("unknown command %q\n%s", name, Usage)
	}
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return Command{}, fmt.Errorf("%s: %w\n%s", name, err, Usage)
	}
	if fs.NArg() > 0 {
		return Command{}, fmt.Errorf("%s: unexpected argument %q\n%s", name, fs.Arg(0), Usage)
	}
	var run runner = func(ctx context.Context, conn db.Conn) (any, error) {
		return retainConfigured(ctx, conn, *apply, *batch)
	}
	if name == anonymizeStudent {
		run = func(ctx context.Context, conn db.Conn) (any, error) {
			return AnonymizeUser(ctx, conn, *student, *apply)
		}
	}
	return command(name, *timeout, func() (runner, error) { return run, nil })
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func command(name string, timeout time.Duration, finish func() (runner, error)) (Command, error) {
	if timeout <= 0 {
		return Command{}, fmt.Errorf("%s: -timeout must be positive\n%s", name, Usage)
	}
	run, err := finish()
	if err != nil {
		return Command{}, fmt.Errorf("%s: %w\n%s", name, err, Usage)
	}
	return Command{Name: name, Timeout: timeout, Run: run}, nil
}
