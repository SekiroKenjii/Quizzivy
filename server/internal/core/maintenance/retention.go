package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/retention"
)

// Retention is the organization's retention settings as the sweep reads them. A nil month count means keep for ever.
type Retention struct {
	KeepAnswersMonths            *int `json:"keepAnswersMonths"`
	KeepFocusEventsMonths        int  `json:"keepFocusEventsMonths"`
	AnonymiseDisabledAfterMonths *int `json:"anonymiseDisabledAfterMonths"`
}

// ReadRetention reads the organization's retention settings.
func ReadRetention(ctx context.Context, conn db.Querier) (Retention, error) {
	var r Retention
	err := conn.QueryRow(ctx, `SELECT keep_answers_months, keep_focus_events_months, anonymise_disabled_after_months
		 FROM app.organization_settings`).Scan(&r.KeepAnswersMonths, &r.KeepFocusEventsMonths, &r.AnonymiseDisabledAfterMonths)
	if err != nil {
		return r, fmt.Errorf("read retention settings: %w", err)
	}
	return r, nil
}

// SweepOptions are the flags of retention-sweep: whether to write, how many rows a batch holds and how long the run may take.
type SweepOptions struct {
	Apply  bool
	Batch  int
	Budget time.Duration
}

// AnswersStep is the answers step of a sweep. With Apply false the counts are what the sweep would remove; with it true, what it removed. OverdueInProgress is the attempts still in progress past their deadline on an assignment that closed before the cutoff, which the sweep never touches: nothing but a teacher's monitor closes them.
type AnswersStep struct {
	Enabled           bool       `json:"enabled"`
	Cutoff            *time.Time `json:"cutoff,omitempty"`
	Attempts          int64      `json:"attempts"`
	Answers           int64      `json:"answers"`
	Batches           int        `json:"batches"`
	OverdueInProgress int64      `json:"overdueInProgress"`
}

// EventsStep is the focus-loss events step of a sweep, counted the way AnswersStep is.
type EventsStep struct {
	Cutoff  time.Time `json:"cutoff"`
	Events  int64     `json:"events"`
	Batches int       `json:"batches"`
}

// AccountsStep is the accounts step of a sweep. The three counts split the long-disabled accounts, so an owner who also has an attempt in progress is counted once, as an owner, and a dry run counts what an apply would do. Anonymise is the number of long-disabled accounts the sweep would anonymise, or with Apply true did; SkippedOwners are those it leaves because they own classes or content, SkippedInProgress those it leaves for now because an attempt is in progress, and Notices the in-app notifications deleted because they carried a name. Failed is the accounts a run could not anonymise even alone, after the batch holding them failed: the step ends with an error and the next run tries them again.
type AccountsStep struct {
	Enabled           bool       `json:"enabled"`
	Cutoff            *time.Time `json:"cutoff,omitempty"`
	Anonymise         int64      `json:"anonymise"`
	SkippedOwners     int64      `json:"skippedOwners"`
	SkippedInProgress int64      `json:"skippedInProgress"`
	Notices           int64      `json:"notices"`
	Batches           int        `json:"batches"`
	Failed            int64      `json:"failedAccounts"`
}

// StepFailure names a step of the sweep that failed and the error it ended with.
type StepFailure struct {
	Step  string `json:"step"`
	Error string `json:"error"`
}

// SweepReport is what retention-sweep printed: the settings it read, one step each for answers, focus-loss events and accounts, and whether it finished. Complete is false when the budget ended a step or a step failed, and the next run goes on.
type SweepReport struct {
	Applied      bool          `json:"applied"`
	DatabaseRole string        `json:"databaseRole"`
	Retention    Retention     `json:"retention"`
	Answers      AnswersStep   `json:"answers"`
	FocusEvents  EventsStep    `json:"focusEvents"`
	Accounts     AccountsStep  `json:"accounts"`
	Complete     bool          `json:"complete"`
	Failed       []StepFailure `json:"failed,omitempty"`
}

var errBudget = errors.New("the budget ended")

type sweeper struct {
	conn     db.Conn
	opts     SweepOptions
	deadline time.Time
	slowest  time.Duration
}

func (s *sweeper) mayStart() bool {
	return time.Until(s.deadline) > 2*s.slowest
}

func (s *sweeper) timed(ctx context.Context, batch func(context.Context) (bool, error)) (bool, error) {
	if !s.mayStart() {
		return false, errBudget
	}
	started := time.Now()
	more, err := batch(ctx)
	if elapsed := time.Since(started); elapsed > s.slowest {
		s.slowest = elapsed
	}
	if err != nil && ctx.Err() != nil {
		return false, errBudget
	}
	return more, err
}

// RetentionSweep applies the organization's retention settings in bounded batches until each step is done or the budget ends. A dry run, which runs in a read-only transaction, counts what each step would do.
// A batch commits on its own, so a run the budget ends, or a step that fails, loses nothing and the next run goes on. A step that fails does not stop the others: it is named in Failed and RetentionSweep returns the report with an error.
func RetentionSweep(ctx context.Context, conn db.Conn, opts SweepOptions) (SweepReport, error) {
	report := SweepReport{Applied: opts.Apply}
	if err := checkBatch(opts.Batch); err != nil {
		return report, err
	}
	if opts.Budget <= 0 {
		return report, errors.New("budget must be positive")
	}
	s := &sweeper{conn: conn, opts: opts, deadline: time.Now().Add(opts.Budget)}
	var err error
	if report.Retention, err = ReadRetention(ctx, conn); err != nil {
		return report, err
	}
	if err := conn.QueryRow(ctx, `SELECT current_user::text`).Scan(&report.DatabaseRole); err != nil {
		return report, err
	}
	report.Complete = true
	steps := []struct {
		name string
		run  func() error
	}{
		{"answers", func() error { return s.answers(ctx, report.Retention.KeepAnswersMonths, &report.Answers) }},
		{"focusEvents", func() error { return s.events(ctx, report.Retention.KeepFocusEventsMonths, &report.FocusEvents) }},
		{"accounts", func() error { return s.accounts(ctx, report.Retention.AnonymiseDisabledAfterMonths, &report.Accounts) }},
	}
	var failures []error
	for _, step := range steps {
		err := step.run()
		switch {
		case errors.Is(err, errBudget):
			report.Complete = false
		case err != nil:
			report.Complete = false
			report.Failed = append(report.Failed, StepFailure{Step: step.name, Error: err.Error()})
			failures = append(failures, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	if len(failures) > 0 {
		return report, fmt.Errorf("retention-sweep: %d of %d steps failed: %w", len(failures), len(steps), errors.Join(failures...))
	}
	return report, nil
}

const nilUUID = "00000000-0000-0000-0000-000000000000"

var eligibleAttempts = retention.EndedBefore("$1")

const removedAnswer = `'` + retention.RemovedAnswer + `'::jsonb`

var overdueInProgress = `SELECT count(*) FROM app.attempts at ` + retention.AttemptJoins + `
	WHERE ` + retention.OverdueInProgressBefore("$1")

func (s *sweeper) answers(ctx context.Context, months *int, step *AnswersStep) error {
	if months == nil {
		return nil
	}
	cutoff, err := cutoffBefore(ctx, s.conn, *months)
	if err != nil {
		return err
	}
	step.Enabled, step.Cutoff = true, &cutoff
	if err := s.readOnly(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, overdueInProgress, cutoff).Scan(&step.OverdueInProgress)
	}); err != nil {
		return err
	}
	if !s.opts.Apply {
		return s.readOnly(ctx, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, retention.CountAnswers, cutoff).Scan(&step.Attempts, &step.Answers)
		})
	}
	after := nilUUID
	for {
		var picked int
		var last *string
		var attempts, answers int64
		more, err := s.timed(ctx, func(ctx context.Context) (bool, error) {
			err := s.conn.QueryRow(ctx, removeAnswers, cutoff, s.opts.Batch, after).Scan(&picked, &last, &attempts, &answers)
			return picked == s.opts.Batch, err
		})
		if err != nil {
			return err
		}
		step.Batches++
		step.Attempts += attempts
		step.Answers += answers
		if !more {
			return nil
		}
		after = *last
	}
}

var removeAnswers = `
	WITH picked AS (
	  SELECT at.id
	    FROM app.attempts at ` + retention.AttemptJoins + `
	   WHERE at.id > $3::uuid AND ` + eligibleAttempts + `
	     AND EXISTS (SELECT 1 FROM app.attempt_answers x WHERE x.attempt_id = at.id AND x.payload <> ` + removedAnswer + `)
	   ORDER BY at.id
	   LIMIT $2
	), due AS (
	  SELECT x.attempt_id, x.question_id
	    FROM app.attempt_answers x
	   WHERE x.attempt_id IN (SELECT id FROM picked) AND x.payload <> ` + removedAnswer + `
	   ORDER BY x.attempt_id, x.question_id
	     FOR UPDATE OF x SKIP LOCKED
	), removed AS (
	  UPDATE app.attempt_answers x SET payload = ` + removedAnswer + `
	    FROM due
	   WHERE x.attempt_id = due.attempt_id AND x.question_id = due.question_id
	  RETURNING x.attempt_id
	), logged AS (
	  INSERT INTO app.audit_log (action, entity, diff)
	  SELECT 'retention.answers_removed', 'attempt_answers',
	         jsonb_build_object('cutoff', $1::timestamptz, 'attempts', count(DISTINCT attempt_id),
	           'answers', count(*), 'databaseRole', current_user)
	    FROM removed HAVING count(*) > 0
	)
	SELECT (SELECT count(*) FROM picked),
	       (SELECT id::text FROM picked ORDER BY id DESC LIMIT 1),
	       (SELECT count(DISTINCT attempt_id) FROM removed),
	       (SELECT count(*) FROM removed)`

func (s *sweeper) events(ctx context.Context, months int, step *EventsStep) error {
	cutoff, err := cutoffBefore(ctx, s.conn, months)
	if err != nil {
		return err
	}
	step.Cutoff = cutoff
	if !s.opts.Apply {
		return s.readOnly(ctx, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, retention.CountEvents, cutoff).Scan(&step.Events)
		})
	}
	after := fromTheStart
	for {
		var removed int64
		more, err := s.timed(ctx, func(ctx context.Context) (bool, error) {
			var err error
			removed, after, err = deleteEvents(ctx, s.conn, cutoff, s.opts.Batch, after)
			return removed == int64(s.opts.Batch), err
		})
		if err != nil {
			return err
		}
		step.Batches++
		step.Events += removed
		if !more {
			return nil
		}
	}
}

const pickAccounts = `
	SELECT u.id::text FROM app.users u
	 WHERE u.id > $3::uuid AND u.disabled_at < $1 AND ` + retention.NotAnonymous + `
	   AND NOT ` + retention.OwnsContent + ` AND NOT ` + retention.HasAttemptInProgress + `
	 ORDER BY u.id LIMIT $2
	   FOR UPDATE OF u SKIP LOCKED`

func (s *sweeper) accounts(ctx context.Context, months *int, step *AccountsStep) error {
	if months == nil {
		return nil
	}
	cutoff, err := cutoffBefore(ctx, s.conn, *months)
	if err != nil {
		return err
	}
	step.Enabled, step.Cutoff = true, &cutoff
	count := func(ctx context.Context, q db.Querier, counted *AccountsStep) error {
		return q.QueryRow(ctx, retention.CountAccounts, cutoff).Scan(&counted.Anonymise, &counted.SkippedOwners, &counted.SkippedInProgress)
	}
	if !s.opts.Apply {
		return s.readOnly(ctx, func(tx pgx.Tx) error { return count(ctx, tx, step) })
	}
	after := nilUUID
	var failures []error
	for {
		var done accountsBatch
		more, err := s.timed(ctx, func(ctx context.Context) (bool, error) {
			var err error
			done, err = s.anonymiseBatch(ctx, cutoff, after)
			return done.picked == s.opts.Batch, err
		})
		if err != nil {
			return err
		}
		step.Batches++
		step.Anonymise += done.users
		step.Notices += done.notices
		step.Failed += int64(len(done.failed))
		failures = append(failures, done.failed...)
		if !more {
			break
		}
		after = done.last
	}
	var left AccountsStep
	if err := count(ctx, s.conn, &left); err != nil {
		return err
	}
	step.SkippedOwners, step.SkippedInProgress = left.SkippedOwners, left.SkippedInProgress
	if len(failures) > 0 {
		return fmt.Errorf("%d accounts could not be anonymised, the first with: %w", len(failures), failures[0])
	}
	return nil
}

type accountsBatch struct {
	picked  int
	last    string
	users   int64
	notices int64
	failed  []error
}

func tryAnonymise(ctx context.Context, tx pgx.Tx, ids []string, cutoff time.Time) (anonymised, error) {
	attempt, err := tx.Begin(ctx)
	if err != nil {
		return anonymised{}, err
	}
	done, err := anonymise(ctx, attempt, ids, "retention", &cutoff)
	if err != nil {
		_ = attempt.Rollback(ctx)
		return anonymised{}, err
	}
	return done, attempt.Commit(ctx)
}

func (s *sweeper) anonymiseBatch(ctx context.Context, cutoff time.Time, after string) (accountsBatch, error) {
	var batch accountsBatch
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return batch, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, pickAccounts, cutoff, s.opts.Batch, after)
	if err != nil {
		return batch, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return batch, err
	}
	if len(ids) == 0 {
		return batch, nil
	}
	batch.picked, batch.last = len(ids), ids[len(ids)-1]
	done, err := tryAnonymise(ctx, tx, ids, cutoff)
	switch {
	case err == nil:
		batch.users, batch.notices = done.users, done.notices
	case ctx.Err() != nil:
		return batch, err
	case len(ids) == 1:
		batch.failed = append(batch.failed, err)
	default:
		for _, id := range ids {
			one, err := tryAnonymise(ctx, tx, []string{id}, cutoff)
			if err != nil {
				if ctx.Err() != nil {
					return batch, err
				}
				batch.failed = append(batch.failed, err)
				continue
			}
			batch.users += one.users
			batch.notices += one.notices
		}
	}
	return batch, tx.Commit(ctx)
}

func (s *sweeper) readOnly(ctx context.Context, read func(pgx.Tx) error) error {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
		return err
	}
	return read(tx)
}
