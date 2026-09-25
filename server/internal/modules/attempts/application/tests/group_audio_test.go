//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func sharedAudioAttempt(t *testing.T, pool *pgxpool.Pool) (*application.Application, world, domain.Session, string, string) {
	t.Helper()
	w := seedWorld(t, pool, openAssignment())
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	first, asset := attemptGroupFixture(t, tx, w)
	var section, second, material, recording string
	if err := tx.QueryRow(ctx, `SELECT test_version_section_id::text FROM app.test_version_groups WHERE id=$1`, first).Scan(&section); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_groups(test_version_section_id,title) VALUES($1,'Same file, another allowance') RETURNING id::text`, section).Scan(&second); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO app.test_version_group_members(group_id,test_version_section_id,question_id,ordinal,option_order) VALUES($1,$2,$3,0,'shuffle')`, []any{second, section, w.essay}},
		{`UPDATE app.test_version_units SET question_id=NULL,group_id=$1 WHERE question_id=$2`, []any{second, w.essay}},
	} {
		if _, err := tx.Exec(ctx, step.sql, step.args...); err != nil {
			t.Fatal(err)
		}
	}
	content := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"` + asset + `","label":"Shared audio"}]}`)
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_stimuli(group_id,ordinal,title,content) VALUES($1,0,'Listen',$2) RETURNING id::text`, second, content).Scan(&material); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_recordings(group_id,media_asset_id,max_plays) VALUES($1,$2,2) RETURNING id::text`, second, asset).Scan(&recording); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_group_assets(stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id) VALUES($1,$2,$3,'audio',$4)`, material, second, asset, recording); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	dbx := db.NewContext(pool)
	papers := testsapp.New(testsrepo.NewPostgres(dbx, nil, nil))
	svc := application.New(repositories.NewTimelines(dbx), nil, repositories.NewPostgres(dbx)).WithGroupContexts(papers.Queries.GroupContexts)
	session, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatal(err)
	}
	return svc, w, session, session.Groups[0].Recordings[0].ID, recording
}

func TestSharedPlaysDeduplicateConcurrentRetriesAndKeepIndependentScopes(t *testing.T) {
	pool := newPool(t)
	svc, w, session, first, second := sharedAudioAttempt(t, pool)
	ctx := context.Background()
	input := domain.GroupPlayInput{AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID, RecordingID: first, PlayID: uuid.NewString()}
	concurrently := func(sameID bool) {
		t.Helper()
		const n = 10
		errs := make([]error, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range n {
			in := input
			if !sameID {
				in.PlayID = uuid.NewString()
			}
			wg.Go(func() {
				<-start
				_, errs[i] = svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: in})
			})
		}
		close(start)
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	concurrently(true)
	if got := count(t, pool, `SELECT plays FROM app.attempt_group_audio_plays WHERE attempt_id=$1 AND recording_id=$2`, input.AttemptID, first); got != 1 {
		t.Fatalf("retry counted %d plays", got)
	}
	concurrently(false)
	retry, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input})
	if err != nil || retry.Plays != 11 || retry.MaxPlays == nil || *retry.MaxPlays != 2 {
		t.Fatalf("retries/over-limit: %+v, %v", retry, err)
	}
	input.RecordingID = second
	if _, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input}); !errors.Is(err, domain.ErrPlayIDConflict) {
		t.Fatalf("gesture reused on another recording: %v", err)
	}
	input.PlayID = uuid.NewString()
	other, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input})
	if err != nil || other.Plays != 1 {
		t.Fatalf("same asset shared the counter across groups: %+v, %v", other, err)
	}
	loaded, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: input.AttemptID, StudentID: w.student})
	if err != nil || loaded.GroupAudioPlays[first] != 11 || loaded.GroupAudioPlays[second] != 1 {
		t.Fatalf("reload lost counts: %+v, %v", loaded.GroupAudioPlays, err)
	}
	timeline, err := svc.Queries.Timeline.Handle(ctx, query.Timeline{AttemptID: input.AttemptID})
	if err != nil || timeline.Summary.AudioReplays != 9 {
		t.Fatalf("timeline omitted shared replays: %+v, %v", timeline.Summary, err)
	}
	monitor, err := svc.Queries.Monitor.Handle(ctx, query.Monitor{AssignmentID: w.assignment})
	if err != nil || len(monitor.Rows) != 1 || !monitor.Rows[0].AudioOverLimit {
		t.Fatalf("monitor omitted shared replays: %+v, %v", monitor.Rows, err)
	}
	resumed, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil || resumed.GroupAudioPlays[first] != 11 || resumed.GroupAudioPlays[second] != 1 {
		t.Fatalf("takeover reset counts: %+v, %v", resumed.GroupAudioPlays, err)
	}
	if _, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input}); !errors.Is(err, domain.ErrSessionSuperseded) {
		t.Fatalf("old session wrote: %v", err)
	}
	input.SessionID = resumed.SessionID
	if _, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`SELECT count(*) FROM app.attempt_group_audio_receipts WHERE attempt_id=$1`,
		`SELECT count(*) FROM app.attempt_events WHERE attempt_id=$1 AND kind='audio_play' AND meta->>'scope'='group'`,
	} {
		if got := count(t, pool, sql, input.AttemptID); got != 12 {
			t.Fatalf("expected 12 exactly-once receipts/events, got %d", got)
		}
	}
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: input.AttemptID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET max_attempts=2 WHERE id=$1`, w.assignment); err != nil {
		t.Fatal(err)
	}
	next, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil || next.Attempt.ID == session.Attempt.ID || len(next.GroupAudioPlays) != 0 {
		t.Fatalf("new attempt inherited counts: %+v, %v", next.GroupAudioPlays, err)
	}
	input.AttemptID = next.Attempt.ID
	input.SessionID = next.SessionID
	nextPlay, err := svc.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: input})
	if err != nil || nextPlay.Plays != 1 {
		t.Fatalf("attempt-scoped play ID: %+v, %v", nextPlay, err)
	}
}

func TestSharedPlaysRejectForeignAndUnwritableRequestsWithoutReceipts(t *testing.T) {
	pool := newPool(t)
	svc, w, session, recording, _ := sharedAudioAttempt(t, pool)
	_, _, _, foreign, _ := sharedAudioAttempt(t, pool)
	input := domain.GroupPlayInput{AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID, RecordingID: recording, PlayID: uuid.NewString()}
	for _, test := range []struct {
		name   string
		change func(*domain.GroupPlayInput)
		want   error
	}{
		{"other student", func(in *domain.GroupPlayInput) { in.StudentID = w.outsider }, domain.ErrForbidden},
		{"other version", func(in *domain.GroupPlayInput) { in.RecordingID = foreign }, domain.ErrForbidden},
		{"unknown recording", func(in *domain.GroupPlayInput) { in.RecordingID = uuid.NewString() }, domain.ErrForbidden},
		{"wrong session", func(in *domain.GroupPlayInput) { in.SessionID = uuid.NewString() }, domain.ErrSessionSuperseded},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := input
			test.change(&in)
			if _, err := svc.Commands.RecordGroupPlay.Handle(context.Background(), command.RecordGroupPlay{Input: in}); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	repo := repositories.NewPostgres(db.NewContext(pool))
	if _, err := repo.RecordGroupPlay(context.Background(), input, session.Attempt.DeadlineAt.Add(time.Second)); !errors.Is(err, domain.ErrDeadlinePassed) {
		t.Fatalf("expired attempt wrote: %v", err)
	}
	if _, err := svc.Commands.Submit.Handle(context.Background(), command.Submit{AttemptID: input.AttemptID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.RecordGroupPlay.Handle(context.Background(), command.RecordGroupPlay{Input: input}); !errors.Is(err, domain.ErrAttemptClosed) {
		t.Fatalf("closed attempt wrote: %v", err)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_group_audio_plays WHERE attempt_id=$1`, input.AttemptID); got != 0 {
		t.Fatalf("refused requests wrote %d counters", got)
	}
}

type failingAudioEvents struct{ *pgxpool.Pool }
type failingAudioEventTx struct{ pgx.Tx }

var errAudioEventWrite = errors.New("injected event write failure")

func (s failingAudioEvents) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failingAudioEventTx{tx}, nil
}
func (tx failingAudioEventTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO app.attempt_events") {
		return pgconn.CommandTag{}, errAudioEventWrite
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestSharedAudioEventFailureRollsBackCounterAndReceipt(t *testing.T) {
	pool := newPool(t)
	svc, w, session, recording, _ := sharedAudioAttempt(t, pool)
	input := domain.GroupPlayInput{AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID, RecordingID: recording, PlayID: uuid.NewString()}
	repo := repositories.NewPostgres(db.NewContext(failingAudioEvents{pool}))
	if _, err := repo.RecordGroupPlay(context.Background(), input, time.Now()); !errors.Is(err, errAudioEventWrite) {
		t.Fatalf("write failure: %v", err)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_group_audio_plays WHERE attempt_id=$1`, input.AttemptID); got != 0 {
		t.Fatalf("counter survived rollback: %d", got)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_group_audio_receipts WHERE attempt_id=$1`, input.AttemptID); got != 0 {
		t.Fatalf("receipt survived rollback: %d", got)
	}
	got, err := svc.Commands.RecordGroupPlay.Handle(context.Background(), command.RecordGroupPlay{Input: input})
	if err != nil || got.Plays != 1 {
		t.Fatalf("retry after rollback: %+v, %v", got, err)
	}
}
