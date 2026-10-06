//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGradingQueueFrozenRichKeysMediaAndSharedContext(t *testing.T) {
	f := newQueueFixture(t)
	p := f.paper(f.teacher, "Published", "short_answer", "multiple_choice", "fill_blank")
	rich := `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Frozen rich text","marks":[]}]}]}`
	bank := f.id("questions")
	f.exec(`INSERT INTO app.questions(id,type,prompt,points,created_by,owner_id,sample_answer,prompt_content) VALUES($1,'short_answer','Bank original',1,$2,$2,'Bank original sample',$3::jsonb)`, bank, f.teacher, rich)
	media := f.id("media_assets")
	f.exec(`INSERT INTO app.media_assets(id,kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by,owner_id) VALUES($1,'audio',$2,'audio/mpeg',128,1000,'queue.mp3',sha256('queue media'::bytea),$3,$3)`, media, "queue-fixture/"+media, f.teacher)
	f.exec(`UPDATE app.test_version_questions SET source_question_id=$2,prompt_content=$3::jsonb,explanation_content=$3::jsonb,media_asset_id=$4,media_asset_kind='audio',audio_max_plays=2,audio_allow_seek=false,audio_show_transcript_after=false,transcript='Frozen question transcript' WHERE id=$1`, p.questions[0], bank, rich, media)
	option := f.id("test_version_options")
	f.exec(`INSERT INTO app.test_version_options(id,test_version_question_id,ordinal,text,is_correct,content) VALUES($1,$2,0,'Frozen correct option',true,$3::jsonb)`, option, p.questions[1], rich)
	wrongOption := f.id("test_version_options")
	f.exec(`INSERT INTO app.test_version_options(id,test_version_question_id,ordinal,text,is_correct) VALUES($1,$2,1,'Frozen wrong option',false)`, wrongOption, p.questions[1])
	blank := f.id("test_version_blanks")
	f.exec(`INSERT INTO app.test_version_blanks(id,test_version_question_id,ordinal,gap_id,case_sensitive) VALUES($1,$2,1,'blank_one',true)`, blank, p.questions[2])
	for _, value := range []string{"Frozen key", "Frozen alternate"} {
		f.exec(`INSERT INTO app.test_version_blank_answers(id,test_version_blank_id,answer) VALUES($1,$2,$3)`, f.id("test_version_blank_answers"), blank, value)
	}
	otherBlank := f.id("test_version_blanks")
	f.exec(`INSERT INTO app.test_version_blanks(id,test_version_question_id,ordinal,gap_id,case_sensitive) VALUES($1,$2,2,'blank_two',false)`, otherBlank, p.questions[2])
	f.exec(`INSERT INTO app.test_version_blank_answers(id,test_version_blank_id,answer) VALUES($1,$2,'Other blank key')`, f.id("test_version_blank_answers"), otherBlank)
	group := f.id("test_version_groups")
	f.exec(`INSERT INTO app.test_version_groups(id,test_version_section_id,title,instructions) VALUES($1,$2,'Frozen group',$3::jsonb)`, group, p.section, rich)
	f.exec(`INSERT INTO app.test_version_units(id,test_version_section_id,ordinal,group_id) VALUES($1,$2,0,$3)`, f.id("test_version_units"), p.section, group)
	f.exec(`INSERT INTO app.test_version_group_members(group_id,test_version_section_id,question_id,ordinal,option_order) VALUES($1,$2,$3,0,'fixed')`, group, p.section, p.questions[0])
	material := f.id("test_version_group_stimuli")
	f.exec(`INSERT INTO app.test_version_group_stimuli(id,group_id,ordinal,title,content) VALUES($1,$2,0,'Frozen passage',$3::jsonb)`, material, group, rich)
	recording := f.id("test_version_group_recordings")
	f.exec(`INSERT INTO app.test_version_group_recordings(id,group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit,transcript) VALUES($1,$2,$3,3,false,false,'Frozen shared transcript')`, recording, group, media)
	f.exec(`INSERT INTO app.test_version_group_assets(stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id) VALUES($1,$2,$3,'audio',$4)`, material, group, media, recording)
	at := f.attempt(p, f.students[0], "submitted", queueTime(1), 1)
	for _, q := range p.questions {
		f.answer(at, q, true, nil)
	}
	groups := testsrepo.NewPostgres(db.NewContext(f.tx), nil, nil)
	app := application.New(nil, f.repo, nil).WithGroupContexts(cqrs.HandlerFunc[testsquery.GroupContexts, []testsdomain.PreviewGroup](func(ctx context.Context, q testsquery.GroupContexts) ([]testsdomain.PreviewGroup, error) {
		return groups.GroupContexts(ctx, q.VersionID)
	}))
	before, err := app.Queries.GradingQueue.Handle(f.ctx, query.GradingQueue{Scope: f.own()})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE app.questions SET prompt='Changed bank',sample_answer='Changed bank sample',prompt_content=NULL WHERE id=$1`, bank)
	f.exec(`UPDATE app.tests SET title='Live assignment title' WHERE id=$1`, p.test)
	f.exec(`UPDATE app.users SET full_name='Live student name' WHERE id=$1`, f.students[0])
	after, err := app.Queries.GradingQueue.Handle(f.ctx, query.GradingQueue{Scope: f.own()})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 3 || after.AnswersRemaining != 3 || after.StudentsWaiting != 1 {
		t.Fatalf("frozen queue counts=%+v", after)
	}
	for i, item := range after.Items {
		if !reflect.DeepEqual(item.Question, before.Items[i].Question) || !reflect.DeepEqual(item.SharedContext, before.Items[i].SharedContext) || item.AssignmentTitle != "Live assignment title" || item.StudentName != "Live student name" {
			t.Fatalf("frozen/live boundary item=%+v before=%+v", item, before.Items[i])
		}
	}
	q := after.Items[0].Question
	if q.SampleAnswer == nil || *q.SampleAnswer != "Frozen teacher sample" || q.Media == nil || q.Media.ID != media || q.Audio == nil || q.Audio.MaxPlays == nil || *q.Audio.MaxPlays != 2 || q.Transcript == nil || *q.Transcript != "Frozen question transcript" || !json.Valid(q.PromptContent) || !json.Valid(q.ExplanationContent) {
		t.Fatalf("frozen question media/rich key=%+v", q)
	}
	options := after.Items[1].Question.Options
	if len(options) != 2 || options[1].ID != wrongOption || options[1].IsCorrect || options[0].ID != option || !options[0].IsCorrect || options[0].Text != "Frozen correct option" || !json.Valid(options[0].Content) {
		t.Fatalf("frozen option identity/key=%+v", options)
	}
	blanks := after.Items[2].Question.Blanks
	if len(blanks) != 2 || blanks[1].ID != otherBlank || blanks[1].GapID == nil || *blanks[1].GapID != "blank_two" || blanks[1].CaseSensitive || !reflect.DeepEqual(blanks[1].Accepted, []string{"Other blank key"}) || blanks[0].ID != blank || blanks[0].GapID == nil || *blanks[0].GapID != "blank_one" || !blanks[0].CaseSensitive || len(blanks[0].Accepted) != 2 {
		t.Fatalf("frozen blank identity/keys=%+v", blanks)
	}
	keys := map[string]bool{}
	for _, key := range blanks[0].Accepted {
		keys[key] = true
	}
	if !keys["Frozen key"] || !keys["Frozen alternate"] {
		t.Fatalf("frozen accepted values=%v", blanks[0].Accepted)
	}
	shared := after.Items[0].SharedContext
	if shared == nil || len(shared.Groups) != 1 || shared.Groups[0].ID != group || len(shared.Groups[0].Stimuli) != 1 || shared.Groups[0].Stimuli[0].ID != material || shared.Transcripts[recording] != "Frozen shared transcript" || shared.AudioPlays != nil {
		t.Fatalf("frozen represented context=%+v", shared)
	}
	delivery, err := repositories.NewPostgres(db.NewContext(f.tx), nil).Questions(f.ctx, p.version)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Frozen teacher sample", "Frozen question transcript", "Frozen key", "Frozen alternate", "Other blank key"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("student delivery leaked %q", secret)
		}
	}
}

func TestGradingQueueConcurrentGradeAndFreshRefetch(t *testing.T) {
	f := newCommittedQueueFixture(t)
	p := f.paper(f.teacher, "Committed race", "short_answer", "short_answer")
	at := f.attempt(p, f.students[0], "submitted", queueTime(1), 1)
	for _, q := range p.questions {
		f.answer(at, q, true, nil)
	}
	f.commitFixture()
	app := application.New(nil, f.repo, nil)
	if _, err := app.Commands.Finish.Handle(f.ctx, command.Finish{Scope: f.own(), AttemptID: at}); !errors.Is(err, domain.ErrGradingIncomplete) {
		t.Fatalf("early Finish=%v", err)
	}
	blocker, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := f.pool.Acquire(f.ctx)
	if err != nil {
		if rollback := blocker.Rollback(context.Background()); rollback != nil {
			t.Errorf("blocker rollback: %v", rollback)
		}
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan error, 1)
	started := false
	joined := false
	released := false
	t.Cleanup(func() {
		cancel()
		clean, cancelClean := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelClean()
		if !released {
			if err := blocker.Rollback(clean); err != nil {
				t.Errorf("race blocker rollback: %v", err)
			}
		}
		if started && !joined {
			select {
			case <-done:
			case <-clean.Done():
				t.Errorf("grade worker did not join: %v", clean.Err())
			}
		}
		writer.Release()
	})
	if _, err := blocker.Exec(f.ctx, `SELECT id FROM app.attempts WHERE id=$1 FOR UPDATE`, at); err != nil {
		t.Fatal(err)
	}
	var blockerPID int
	if err := blocker.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	writerApp := application.New(nil, repositories.NewReviews(db.NewContext(writer)), nil)
	started = true
	go func() {
		_, err := writerApp.Commands.Grade.Handle(ctx, command.Grade{Scope: f.own(), AttemptID: at, GraderID: f.teacher, Items: []domain.GradeItem{{QuestionID: p.questions[0], Points: 0}}})
		done <- err
	}()
	waitQueueBlock(t, f, writer.Conn().PgConn().PID(), blockerPID)
	before := f.read(domain.GradingQueueQuery{Scope: f.own()})
	if before.AnswersRemaining != 2 || len(before.Items) != 2 {
		t.Fatalf("pre-grade statement snapshot=%+v", before)
	}
	if err := blocker.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	released = true
	select {
	case err := <-done:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-f.ctx.Done():
		t.Fatal("grade did not finish")
	}
	fresh, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	after, readErr := repositories.NewReviews(db.NewContext(fresh)).GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own()})
	rollbackErr := fresh.Rollback(f.ctx)
	if readErr != nil || rollbackErr != nil {
		t.Fatalf("fresh queue read=%v rollback=%v", readErr, rollbackErr)
	}
	if after.AnswersRemaining != 1 || len(after.Items) != 1 || after.Items[0].Question.ID != p.questions[1] {
		t.Fatalf("public zero Grade did not remove exactly one answer: %+v", after)
	}
	review, err := f.repo.Get(f.ctx, f.own(), at)
	if err != nil {
		t.Fatal(err)
	}
	if review.Attempt.Status != domain.Submitted || review.Answers[p.questions[0]].ManualScore == nil || *review.Answers[p.questions[0]].ManualScore != 0 {
		t.Fatalf("Grade implicitly finished or lost zero: %+v", review)
	}
	if _, err := app.Commands.Finish.Handle(f.ctx, command.Finish{Scope: f.own(), AttemptID: at}); !errors.Is(err, domain.ErrGradingIncomplete) {
		t.Fatalf("incomplete Finish after one grade=%v", err)
	}
	captured := make(chan struct{})
	resume := make(chan struct{})
	snapshotDone := make(chan queueSnapshotResult, 1)
	reader, err := f.pool.Acquire(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshotCtx, cancelSnapshot := context.WithCancel(f.ctx)
	resumed := false
	snapshotJoined := false
	t.Cleanup(func() {
		cancelSnapshot()
		if !resumed {
			close(resume)
		}
		if !snapshotJoined {
			select {
			case <-snapshotDone:
			case <-time.After(5 * time.Second):
				t.Error("snapshot worker did not join")
			}
		}
		reader.Release()
	})
	snapshotRepo := repositories.NewReviews(db.NewContext(queueCaptureConn{Conn: reader, captured: captured, resume: resume, once: &sync.Once{}}))
	go func() {
		out, err := snapshotRepo.GradingQueue(snapshotCtx, domain.GradingQueueQuery{Scope: f.own()})
		snapshotDone <- queueSnapshotResult{out: out, err: err}
	}()
	select {
	case <-captured:
		t.Log("QUEUE_SNAPSHOT_CAPTURED after actual public statement Scan")
	case <-f.ctx.Done():
		t.Fatal("public queue statement was not captured")
	}
	if _, err := app.Commands.Grade.Handle(f.ctx, command.Grade{Scope: f.own(), AttemptID: at, GraderID: f.teacher, Items: []domain.GradeItem{{QuestionID: p.questions[1], Points: 1}}}); err != nil {
		t.Fatal(err)
	}
	close(resume)
	resumed = true
	select {
	case snapshot := <-snapshotDone:
		snapshotJoined = true
		if snapshot.err != nil || snapshot.out.AnswersRemaining != 1 || snapshot.out.StudentsWaiting != 1 || len(snapshot.out.Items) != 1 || snapshot.out.Items[0].Question.ID != p.questions[1] || snapshot.out.Groups[0].Remaining != 1 {
			t.Fatalf("statement membership/count coherence across actual Grade: %+v,%v", snapshot.out, snapshot.err)
		}
	case <-f.ctx.Done():
		t.Fatal("snapshot reader did not join")
	}
	empty := f.read(domain.GradingQueueQuery{Scope: f.own()})
	if empty.AnswersRemaining != 0 || len(empty.Groups) != 0 || len(empty.Items) != 0 {
		t.Fatalf("all manually graded refetch=%+v", empty)
	}
	final, err := app.Commands.Finish.Handle(f.ctx, command.Finish{Scope: f.own(), AttemptID: at})
	if err != nil || final.Status != domain.Graded {
		t.Fatalf("explicit Finish=%+v,%v", final, err)
	}
}

func waitQueueBlock(t *testing.T, f *queueFixture, writer uint32, blocker int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err := f.pool.QueryRow(ctx, `SELECT $2::integer=ANY(pg_blocking_pids($1::integer))`, writer, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			t.Logf("QUEUE_GRADE_BLOCK writer=%d blocker=%d", writer, blocker)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("public Grade did not reach the actual attempt lock")
		case <-tick.C:
		}
	}
}

type queueSnapshotResult struct {
	out domain.GradingQueue
	err error
}
type queueCaptureConn struct {
	db.Conn
	captured chan struct{}
	resume   <-chan struct{}
	once     *sync.Once
}

// QueryRow captures the real statement before the queue reader resumes.
func (c queueCaptureConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return queueCaptureRow{Row: c.Conn.QueryRow(ctx, sql, args...), ctx: ctx, captured: c.captured, resume: c.resume, once: c.once}
}

type queueCaptureRow struct {
	pgx.Row
	ctx      context.Context
	captured chan struct{}
	resume   <-chan struct{}
	once     *sync.Once
}

// Scan pauses after the real row has supplied the statement snapshot.
func (r queueCaptureRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	r.once.Do(func() { close(r.captured) })
	select {
	case <-r.resume:
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}
