//go:build integration

package application_test

import (
	"errors"
	"quizzivy/internal/core/adapters"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"slices"
	"testing"
	"time"
)

func (r *raceWorld) copiedBank() string {
	r.t.Helper()
	result, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, r.copyIn(0))
	if err != nil || !result.Copied {
		r.t.Fatalf("copying the bank question into the group: copied=%v, %v", result.Copied, err)
	}
	return result.QuestionID
}

func (r *raceWorld) leave(member string, index int) domain.MoveQuestionInput {
	return r.moveOut(member, index)
}

func (r *raceWorld) restoring(store repositories.GroupQuestionStore, in domain.MoveQuestionInput) func() error {
	return func() error {
		result, err := r.groups(store, r.media()).MoveQuestion(r.ctx, in)
		if err == nil && !result.Restored {
			return errors.New("the copy did not go back as the original")
		}
		return err
	}
}

func (r *raceWorld) softDeleteBank() func() error {
	return func() error {
		return questionsrepo.NewPostgres(db.NewContext(r.pool)).SoftDelete(r.ctx, questionsdomain.WriteInput{ID: r.bank, ActorID: r.author, Now: time.Now()})
	}
}

func TestRestoreSerialisesAgainstBankDelete(t *testing.T) {
	t.Run("the move holds the original's row first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		copied := r.copiedBank()
		store, locked, open := newPausedQuestionLock(t, r.bank)
		moved := r.spawn(func() error {
			_, err := r.groups(store, r.media()).MoveQuestion(r.ctx, r.leave(copied, 1))
			return err
		})
		pid := r.await(locked, moved)
		deleted := r.spawn(r.softDeleteBank())
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the delete never waited on the move's lock on the original")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-deleted; !errors.Is(err, questionsdomain.ErrReferenced) {
			t.Fatalf("the delete must be refused, the draft referencing the original again: %v", err)
		}
		var deletedAt *time.Time
		if err := r.pool.QueryRow(r.ctx, `SELECT deleted_at FROM app.questions WHERE id=$1`, r.bank).Scan(&deletedAt); err != nil || deletedAt != nil {
			t.Fatalf("the original was deleted: %v, %v", deletedAt, err)
		}
		if got := r.units(); !slices.Equal(got, []string{"q:" + r.lead, "q:" + r.bank, "g:" + r.group.Bundle.Group.ID}) {
			t.Fatalf("units %v", got)
		}
		var remaining int
		if err := r.pool.QueryRow(r.ctx, `SELECT count(*) FROM app.questions WHERE id=$1`, copied).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("the copy is still there: %d, %v", remaining, err)
		}
	})

	t.Run("a deletion commits first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		copied := r.copiedBank()
		tx, err := r.pool.Begin(r.ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(r.ctx) })
		var pid int
		if err := tx.QueryRow(r.ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(r.ctx, `UPDATE app.questions SET deleted_at=now() WHERE id=$1`, r.bank); err != nil {
			t.Fatal(err)
		}
		in := r.leave(copied, 1)
		type outcome struct {
			result domain.MoveQuestionResult
			err    error
		}
		moved := make(chan outcome, 1)
		go func() {
			result, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, in)
			moved <- outcome{result, err}
		}()
		if !r.blockedBy(pid) {
			t.Fatal("the move never waited on the other writer's lock on the original")
		}
		if err := tx.Commit(r.ctx); err != nil {
			t.Fatal(err)
		}

		got := <-moved
		if got.err != nil {
			t.Fatalf("a deleted original must not fail the move: %v", got.err)
		}
		if got.result.Restored || got.result.EditedCopy || got.result.QuestionID != copied {
			t.Fatalf("restored=%v edited=%v id=%s, want the copy to leave under its own id", got.result.Restored, got.result.EditedCopy, got.result.QuestionID)
		}
		var standalone, forgotten bool
		if err := r.pool.QueryRow(r.ctx, `SELECT context_group_id IS NULL, copied_from_id IS NULL FROM app.questions WHERE id=$1`, copied).Scan(&standalone, &forgotten); err != nil || !standalone || !forgotten {
			t.Fatalf("the copy is a bank question: %v, forgotten origin: %v, %v", standalone, forgotten, err)
		}
		if got := r.units(); !slices.Equal(got, []string{"q:" + r.lead, "q:" + copied, "g:" + r.group.Bundle.Group.ID}) {
			t.Fatalf("units %v: a deleted question must not be placed", got)
		}
	})
}

func TestRestoreLocksQuestionsInIdOrderAgainstAnotherTestsAutosave(t *testing.T) {
	arrange := func(t *testing.T) (r *raceWorld, copied, legacy string, sharing domain.UpdateInput, sharingID string, others []string) {
		t.Helper()
		r = newRaceWorld(t, raceOptions{})
		time.Sleep(5 * time.Millisecond)
		first := r.bankQuestion("sau 1", false)
		time.Sleep(5 * time.Millisecond)
		second := r.bankQuestion("sau 2", false)
		others = []string{first, second}
		if r.bank > first || first > second {
			t.Fatalf("the questions are not in creation order: %s %s %s", r.bank, first, second)
		}
		if err := r.pool.QueryRow(r.ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,1,'Part 2') RETURNING id::text`, r.testID).Scan(&legacy); err != nil {
			t.Fatal(err)
		}
		for ordinal, id := range others {
			if _, err := r.pool.Exec(r.ctx, `INSERT INTO app.test_section_questions (test_section_id,ordinal,question_id) VALUES ($1,$2,$3)`, legacy, ordinal, id); err != nil {
				t.Fatal(err)
			}
		}
		copied = r.copiedBank()
		created, err := r.tests(adapters.GroupQuestions{}).Create(r.ctx, domain.CreateInput{Title: "Shares the questions", ActorID: r.author, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		sharingID = created.ID
		sharing = domain.UpdateInput{ExpectedUpdatedAt: created.UpdatedAt, SetSections: true,
			Sections: []domain.SectionInput{{Title: "Part 1", QuestionIDs: []string{r.bank, first, second}}}}
		return r, copied, legacy, sharing, sharingID, others
	}
	restore := func(r *raceWorld, member, legacy string) domain.MoveQuestionInput {
		in := r.leave(member, 0)
		in.To.SectionID = legacy
		return in
	}
	complete := func(t *testing.T, r *raceWorld, legacy string, others []string, moved, saved <-chan error) {
		t.Helper()
		if err := <-moved; err != nil {
			t.Fatalf("the restore: %v", err)
		}
		if err := <-saved; err != nil {
			t.Fatalf("the autosave of the test that shares the questions: %v", err)
		}
		got := r.sectionOrder(`SELECT question_id::text FROM app.test_section_units WHERE test_section_id=$1 ORDER BY ordinal`, legacy)
		if want := append([]string{r.bank}, others...); !slices.Equal(got, want) {
			t.Fatalf("the unit-less section holds %v, want the original first and its own questions after: %v", got, want)
		}
	}

	t.Run("an autosave of the other test holds the original first", func(t *testing.T) {
		r, copied, legacy, sharing, sharingID, others := arrange(t)
		store, locked, open := newPausedQuestionLock(t, r.bank)
		saved := r.spawn(func() error {
			_, err := r.tests(store).Update(r.ctx, domain.UpdateRequest{ID: sharingID, ActorID: r.author, Now: time.Now(), Scope: r.scope, Input: sharing})
			return err
		})
		pid := r.await(locked, saved)
		moved := r.spawn(r.restoring(adapters.GroupQuestions{}, restore(r, copied, legacy)))
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the restore never waited on the other test's lock on the original")
		}
		open()

		complete(t, r, legacy, others, moved, saved)
	})

	t.Run("the restore holds the original first", func(t *testing.T) {
		r, copied, legacy, sharing, sharingID, others := arrange(t)
		store, locked, open := newPausedQuestionLock(t, r.bank)
		moved := r.spawn(r.restoring(store, restore(r, copied, legacy)))
		pid := r.await(locked, moved)
		saved := r.spawn(func() error {
			_, err := r.tests(adapters.GroupQuestions{}).Update(r.ctx, domain.UpdateRequest{ID: sharingID, ActorID: r.author, Now: time.Now(), Scope: r.scope, Input: sharing})
			return err
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the other test's autosave never waited on the restore")
		}
		open()

		complete(t, r, legacy, others, moved, saved)
	})
}
