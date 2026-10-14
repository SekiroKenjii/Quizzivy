//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type raceOptions struct {
	leadImage bool
	bankImage bool
}

type raceWorld struct {
	t       *testing.T
	ctx     context.Context
	pool    *pgxpool.Pool
	author  string
	scope   access.Scope
	testID  string
	section string
	lead    string
	bank    string
	asset   string
	group   domain.StoredGroup
	members []string
}

var raceCleanup = []string{
	`DELETE FROM app.test_section_units WHERE test_section_id IN
		(SELECT s.id FROM app.test_sections s JOIN app.tests t ON t.id=s.test_id WHERE t.created_by=$1)`,
	`DELETE FROM app.questions WHERE context_group_id IN (SELECT id FROM app.question_groups WHERE created_by=$1)`,
	`DELETE FROM app.question_groups WHERE created_by=$1`,
	`DELETE FROM app.test_versions WHERE published_by=$1`,
	`DELETE FROM app.audit_log WHERE actor_user_id=$1`,
	`DELETE FROM app.tests WHERE created_by=$1`,
	`DELETE FROM app.questions WHERE created_by=$1`,
	`DELETE FROM app.media_assets WHERE uploaded_by=$1`,
	`DELETE FROM app.users WHERE id=$1`,
}

func newRaceWorld(t *testing.T, opts raceOptions) *raceWorld {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	r := &raceWorld{t: t, ctx: ctx, pool: newPool(t)}
	t.Cleanup(func() {
		if r.author == "" {
			return
		}
		tx, err := r.pool.Begin(context.Background())
		if err != nil {
			t.Errorf("cleanup: %v", err)
			return
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		for _, statement := range raceCleanup {
			if _, err := tx.Exec(context.Background(), statement, r.author); err != nil {
				t.Errorf("cleanup %q: %v", statement, err)
				return
			}
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Errorf("cleanup commit: %v", err)
		}
	})
	r.author = r.insertAuthor()
	r.scope = access.Scope{UserID: r.author}
	if opts.leadImage || opts.bankImage {
		r.asset = r.insertImage()
	}
	r.lead, r.bank = r.bankQuestion("lead", opts.leadImage), r.bankQuestion("bank", opts.bankImage)
	tests := r.tests(adapters.GroupQuestions{})
	created, err := tests.Create(r.ctx, domain.CreateInput{Title: "Race fixture", ActorID: r.author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	r.testID = created.ID
	saved, err := tests.Update(r.ctx, domain.UpdateRequest{ID: r.testID, ActorID: r.author, Now: time.Now(), Scope: r.scope, Input: domain.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt, SetSections: true, Sections: []domain.SectionInput{{Title: "Part 1", QuestionIDs: []string{r.lead, r.bank}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	r.section = saved.Sections[0].ID
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(t), Title: "Nhóm tranh chấp", Stimuli: []domain.GroupStimulus{}, Recordings: []domain.GroupRecording{}}}
	for _, prompt := range []string{"m0", "m1", "m2"} {
		id := groupIdentity(t)
		bundle.Group.Members = append(bundle.Group.Members, domain.GroupMember{QuestionID: id, OptionOrder: "shuffle"})
		bundle.Questions = append(bundle.Questions, domain.GroupQuestion{ID: id, Input: plainInput(prompt)})
		r.members = append(r.members, id)
	}
	r.group, err = r.groups(adapters.GroupQuestions{}, r.media()).Create(r.ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &r.section,
		ExpectedTestUpdatedAt: r.test().UpdatedAt, ActorID: r.author, Now: time.Now(), Scope: r.scope, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *raceWorld) insertAuthor() string {
	r.t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		r.t.Fatal(err)
	}
	var id string
	if err := r.pool.QueryRow(r.ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`,
		"move-race-"+hex.EncodeToString(nonce)+"@example.com").Scan(&id); err != nil {
		r.t.Fatal(err)
	}
	return id
}

func (r *raceWorld) insertImage() string {
	r.t.Helper()
	var id string
	if err := r.pool.QueryRow(r.ctx, `INSERT INTO app.media_assets
		(kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by,owner_id)
		VALUES ('image',$1,'image/png',100,NULL,'fixture',$2,$3,$3) RETURNING id::text`, groupIdentity(r.t), make([]byte, 32), r.author).Scan(&id); err != nil {
		r.t.Fatal(err)
	}
	return id
}

func (r *raceWorld) bankQuestion(prompt string, image bool) string {
	r.t.Helper()
	in := plainInput(prompt)
	write := questionsdomain.WriteInput{ActorID: r.author, Now: time.Now()}
	if image {
		kind := "image"
		in.MediaAssetID, in.MediaAlt = &r.asset, ptr("Hình minh hoạ")
		write.MediaAssetKind = &kind
	}
	write.Input = in
	created, err := questionsrepo.NewPostgres(db.NewContext(r.pool)).Create(r.ctx, write)
	if err != nil {
		r.t.Fatal(err)
	}
	return created.ID
}

func (r *raceWorld) media() *mediarepo.Postgres { return mediarepo.NewPostgres(db.NewContext(r.pool)) }

func (r *raceWorld) groups(q repositories.GroupQuestionStore, m repositories.MediaLocks) *repositories.GroupsPostgres {
	return repositories.NewGroupsPostgres(db.NewContext(r.pool), q, m)
}

func (r *raceWorld) tests(q repositories.GroupQuestionStore) *repositories.Postgres {
	return r.testsRepo(q, r.media())
}

func (r *raceWorld) testsRepo(q repositories.GroupQuestionStore, m repositories.MediaLocks) *repositories.Postgres {
	return repositories.NewPostgres(db.NewContext(r.pool), q, m).WithGroupQuestions(q)
}

func (r *raceWorld) test() domain.Test {
	r.t.Helper()
	test, err := r.tests(adapters.GroupQuestions{}).Get(r.ctx, r.scope, r.testID)
	if err != nil {
		r.t.Fatal(err)
	}
	return test
}

func (r *raceWorld) stored() domain.StoredGroup {
	r.t.Helper()
	stored, err := r.groups(adapters.GroupQuestions{}, r.media()).Get(r.ctx, r.scope, r.group.Bundle.Group.ID)
	if err != nil {
		r.t.Fatal(err)
	}
	return stored
}

func (r *raceWorld) input(question string, group *string, index int, touched ...domain.GroupRevision) domain.MoveQuestionInput {
	r.t.Helper()
	return domain.MoveQuestionInput{TestID: r.testID, QuestionID: question, ExpectedTestUpdatedAt: r.test().UpdatedAt,
		To: domain.MoveTarget{SectionID: r.section, GroupID: group, Index: index}, ExpectedGroups: touched,
		ActorID: r.author, Now: time.Now(), Scope: r.scope, Grants: bothKeys}
}

func (r *raceWorld) copyIn(index int) domain.MoveQuestionInput {
	id := r.group.Bundle.Group.ID
	return r.input(r.bank, ptr(id), index, domain.GroupRevision{GroupID: id, Revision: r.stored().Revision})
}

func (r *raceWorld) moveOut(member string, index int) domain.MoveQuestionInput {
	id := r.group.Bundle.Group.ID
	return r.input(member, nil, index, domain.GroupRevision{GroupID: id, Revision: r.stored().Revision})
}

func (r *raceWorld) sectionOrder(query string, args ...any) []string {
	r.t.Helper()
	rows, err := r.pool.Query(r.ctx, query, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

func (r *raceWorld) units() []string {
	return r.sectionOrder(`SELECT CASE WHEN group_id IS NULL THEN 'q:'||question_id::text ELSE 'g:'||group_id::text END
		FROM app.test_section_units WHERE test_section_id=$1 ORDER BY ordinal`, r.section)
}

func (r *raceWorld) order() []string {
	return r.sectionOrder(`SELECT id::text FROM app.questions WHERE context_group_id=$1 ORDER BY context_ordinal`, r.group.Bundle.Group.ID)
}

type pausedQuestionLock struct {
	adapters.GroupQuestions
	target  string
	locked  chan int
	release <-chan struct{}
	once    *sync.Once
}

func newPausedQuestionLock(t *testing.T, target string) (pausedQuestionLock, <-chan int, func()) {
	t.Helper()
	locked, release, once := make(chan int, 1), make(chan struct{}), new(sync.Once)
	open := func() { once.Do(func() { close(release) }) }
	t.Cleanup(open)
	return pausedQuestionLock{target: target, locked: locked, release: release, once: new(sync.Once)}, locked, open
}

func (p pausedQuestionLock) LockForDraftUse(ctx context.Context, tx pgx.Tx, id string) error {
	if err := p.GroupQuestions.LockForDraftUse(ctx, tx, id); err != nil || id != p.target {
		return err
	}
	pause := false
	p.once.Do(func() { pause = true })
	if !pause {
		return nil
	}
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		return err
	}
	p.locked <- pid
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *raceWorld) await(locked <-chan int, done <-chan error) int {
	r.t.Helper()
	select {
	case pid := <-locked:
		return pid
	case err := <-done:
		r.t.Fatalf("a writer ended before the lock it should hold: %v", err)
	case <-time.After(10 * time.Second):
		r.t.Fatal("the writer never reached its lock")
	}
	return 0
}

func (r *raceWorld) blockedBy(pid int) bool {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := r.pool.QueryRow(r.ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
			r.t.Fatal(err)
		}
		if blocked {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func (r *raceWorld) spawn(f func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- f() }()
	return done
}

func newPausedGroupMedia(t *testing.T) (pausedGroupMedia, <-chan int, func()) {
	t.Helper()
	locked, release, once := make(chan int, 1), make(chan struct{}), new(sync.Once)
	open := func() { once.Do(func() { close(release) }) }
	t.Cleanup(open)
	return pausedGroupMedia{locked: locked, release: release}, locked, open
}
