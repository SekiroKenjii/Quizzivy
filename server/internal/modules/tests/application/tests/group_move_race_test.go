//go:build integration

package application_test

import (
	"errors"
	"quizzivy/internal/core/adapters"
	mediadomain "quizzivy/internal/modules/media/domain"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"slices"
	"sync"
	"testing"
	"time"
)

func (r *raceWorld) autosave(expected time.Time) domain.UpdateInput {
	test := r.test()
	sections := make([]domain.SectionInput, len(test.Sections))
	for i, section := range test.Sections {
		sections[i] = domain.SectionInput{ID: section.ID, Title: section.Title, QuestionIDs: section.QuestionIDs, Units: section.Units, SetUnits: true}
	}
	return domain.UpdateInput{ExpectedUpdatedAt: expected, SetSections: true, GroupOutline: true, Sections: sections}
}

func TestMoveIntoGroupSerialisesAgainstBankDelete(t *testing.T) {
	t.Run("the move holds the question's row first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		store, locked, open := newPausedQuestionLock(t, r.bank)
		in := r.copyIn(1)
		moved := r.spawn(func() error {
			_, err := r.groups(store, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		pid := r.await(locked, moved)
		deleted := r.spawn(func() error {
			return questionsrepo.NewPostgres(db.NewContext(r.pool)).SoftDelete(r.ctx, questionsdomain.WriteInput{ID: r.bank, ActorID: r.author, Now: time.Now()})
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the delete never waited on the move's lock on the bank question")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-deleted; err != nil {
			t.Fatalf("the delete must succeed once the draft no longer references the question: %v", err)
		}
		var deletedAt *time.Time
		if err := r.pool.QueryRow(r.ctx, `SELECT deleted_at FROM app.questions WHERE id=$1`, r.bank).Scan(&deletedAt); err != nil || deletedAt == nil {
			t.Fatalf("the bank question was not deleted: %v, %v", deletedAt, err)
		}
		stored := r.stored()
		if len(stored.Bundle.Group.Members) != 4 || stored.Bundle.Group.Members[1].QuestionID == r.bank {
			t.Fatalf("the group does not hold a copy: %v", memberIDs(stored))
		}
		for _, id := range memberIDs(stored) {
			var gone bool
			if err := r.pool.QueryRow(r.ctx, `SELECT deleted_at IS NOT NULL FROM app.questions WHERE id=$1`, id).Scan(&gone); err != nil || gone {
				t.Fatalf("member %s is deleted: %v", id, err)
			}
		}
	})

	t.Run("a deletion commits first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
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
		before := slices.Clone(r.order())
		in := r.copyIn(1)
		moved := r.spawn(func() error {
			_, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		if !r.blockedBy(pid) {
			t.Fatal("the move never waited on the other writer's lock on the bank question")
		}
		if err := tx.Commit(r.ctx); err != nil {
			t.Fatal(err)
		}

		if err := <-moved; !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("the move must find the question gone: %v", err)
		}
		if !slices.Equal(r.order(), before) || r.stored().Revision != r.group.Revision {
			t.Fatal("a refused move wrote to the group")
		}
	})
}

func TestMoveOutSerialisesAgainstGroupSave(t *testing.T) {
	t.Run("the move holds the group first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		store, locked, open := newPausedQuestionLock(t, r.members[1])
		in := r.moveOut(r.members[1], 0)
		moved := r.spawn(func() error {
			_, err := r.groups(store, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		pid := r.await(locked, moved)
		saved := r.spawn(func() error { return r.saveGroup(r.group) })
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the group save never waited on the move")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-saved; !errors.Is(err, domain.ErrStaleWrite) {
			t.Fatalf("the save at the old revision must lose: %v", err)
		}
		if got := r.order(); !slices.Equal(got, []string{r.members[0], r.members[2]}) {
			t.Fatalf("members %v", got)
		}
	})

	t.Run("either order has exactly one winner", func(t *testing.T) {
		for attempt := range 6 {
			r := newRaceWorld(t, raceOptions{})
			in := r.moveOut(r.members[1], 0)
			var wg sync.WaitGroup
			var moveErr, saveErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, moveErr = r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, in)
			}()
			go func() {
				defer wg.Done()
				saveErr = r.saveGroup(r.group)
			}()
			wg.Wait()

			if (moveErr == nil) == (saveErr == nil) {
				t.Fatalf("attempt %d: move=%v save=%v, want exactly one winner", attempt, moveErr, saveErr)
			}
			if loser := errors.Join(moveErr, saveErr); !errors.Is(loser, domain.ErrStaleWrite) {
				t.Fatalf("attempt %d: the loser must be stale: %v", attempt, loser)
			}
			wantMembers := 3
			if moveErr == nil {
				wantMembers = 2
			}
			if got := len(r.order()); got != wantMembers {
				t.Fatalf("attempt %d: %d members, want %d", attempt, got, wantMembers)
			}
		}
	})
}

func (r *raceWorld) saveGroup(observed domain.StoredGroup) error {
	bundle := observed.Bundle
	bundle.Group.Title = "Đã sửa song song"
	_, err := r.groups(adapters.GroupQuestions{}, r.media()).Update(r.ctx, domain.UpdateGroupInput{GroupMutation: domain.GroupMutation{
		ID: observed.Bundle.Group.ID, ExpectedRevision: observed.Revision, ExpectedTestUpdatedAt: *observed.TestUpdatedAt,
		ActorID: r.author, Now: time.Now(), Scope: r.scope, Grants: bothKeys}, Bundle: bundle})
	return err
}

func TestMoveSerialisesAgainstOutlineAutosave(t *testing.T) {
	t.Run("the move holds the test first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		store, locked, open := newPausedQuestionLock(t, r.members[0])
		in, autosave := r.moveOut(r.members[0], 0), r.autosave(r.test().UpdatedAt)
		moved := r.spawn(func() error {
			_, err := r.groups(store, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		pid := r.await(locked, moved)
		saved := r.spawn(func() error {
			_, err := r.tests(adapters.GroupQuestions{}).Update(r.ctx, domain.UpdateRequest{ID: r.testID, ActorID: r.author, Now: time.Now(), Scope: r.scope, Input: autosave})
			return err
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the autosave never waited on the move")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-saved; !errors.Is(err, domain.ErrStaleWrite) {
			t.Fatalf("the autosave at the old revision must lose: %v", err)
		}
		if got := r.units(); len(got) != 4 || got[0] != "q:"+r.members[0] {
			t.Fatalf("units %v", got)
		}
	})

	t.Run("the autosave holds the test first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		store, locked, open := newPausedQuestionLock(t, r.lead)
		in, autosave := r.moveOut(r.members[0], 0), r.autosave(r.test().UpdatedAt)
		saved := r.spawn(func() error {
			_, err := r.tests(store).Update(r.ctx, domain.UpdateRequest{ID: r.testID, ActorID: r.author, Now: time.Now(), Scope: r.scope, Input: autosave})
			return err
		})
		pid := r.await(locked, saved)
		moved := r.spawn(func() error {
			_, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the move never waited on the autosave")
		}
		open()

		if err := <-saved; err != nil {
			t.Fatalf("autosave: %v", err)
		}
		if err := <-moved; !errors.Is(err, domain.ErrStaleWrite) {
			t.Fatalf("the move at the old revision must lose: %v", err)
		}
		if got := r.order(); len(got) != 3 {
			t.Fatalf("members %v", got)
		}
	})
}

func TestMoveSerialisesAgainstPublish(t *testing.T) {
	version := func(r *raceWorld, number int) (units, questions []string) {
		section := `(SELECT s.id FROM app.test_version_sections s JOIN app.test_versions v ON v.id=s.test_version_id WHERE v.test_id=$1 AND v.version=$2)`
		units = r.sectionOrder(`SELECT CASE WHEN u.group_id IS NULL THEN 'q:'||q.source_question_id::text ELSE 'g' END
			FROM app.test_version_units u LEFT JOIN app.test_version_questions q ON q.id=u.question_id
			WHERE u.test_version_section_id=`+section+` ORDER BY u.ordinal`, r.testID, number)
		questions = r.sectionOrder(`SELECT source_question_id::text FROM app.test_version_questions WHERE test_version_section_id=`+section+` ORDER BY ordinal`, r.testID, number)
		return units, questions
	}
	publish := func(r *raceWorld, media repositories.MediaLocks) error {
		_, err := r.testsRepo(adapters.GroupQuestions{}, media).Publish(r.ctx, domain.PublishRequest{TestID: r.testID, ActorID: r.author, Scope: r.scope},
			time.Now(), func(domain.DraftContent) error { return nil })
		return err
	}

	t.Run("the move holds the test first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{})
		store, locked, open := newPausedQuestionLock(t, r.members[1])
		in := r.moveOut(r.members[1], 1)
		moved := r.spawn(func() error {
			_, err := r.groups(store, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		pid := r.await(locked, moved)
		published := r.spawn(func() error { return publish(r, r.media()) })
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the publish never waited on the move")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-published; err != nil {
			t.Fatalf("publish: %v", err)
		}
		units, questions := version(r, 1)
		if !slices.Equal(units, []string{"q:" + r.lead, "q:" + r.members[1], "q:" + r.bank, "g"}) ||
			!slices.Equal(questions, []string{r.lead, r.members[1], r.bank, r.members[0], r.members[2]}) {
			t.Fatalf("the version is not the state after the move:\nunits %v\nquestions %v", units, questions)
		}
	})

	t.Run("the publish holds the test first", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{leadImage: true})
		media, locked, open := newPausedGroupMedia(t)
		in := r.moveOut(r.members[1], 1)
		published := r.spawn(func() error { return publish(r, media) })
		pid := r.await(locked, published)
		moved := r.spawn(func() error {
			_, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, in)
			return err
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the move never waited on the publish")
		}
		open()

		if err := <-published; err != nil {
			t.Fatalf("publish: %v", err)
		}
		if err := <-moved; !errors.Is(err, domain.ErrStaleWrite) {
			t.Fatalf("the move at the old revision must lose: %v", err)
		}
		units, questions := version(r, 1)
		if !slices.Equal(units, []string{"q:" + r.lead, "q:" + r.bank, "g"}) ||
			!slices.Equal(questions, []string{r.lead, r.bank, r.members[0], r.members[1], r.members[2]}) {
			t.Fatalf("the version is not the state before the move:\nunits %v\nquestions %v", units, questions)
		}
		if got := r.order(); len(got) != 3 {
			t.Fatalf("a refused move changed the group: %v", got)
		}
	})
}

func TestMoveInContendsOnTheCopiedQuestionsAsset(t *testing.T) {
	t.Run("a media delete waits for the move's lock", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{bankImage: true})
		media, locked, open := newPausedGroupMedia(t)
		in := r.copyIn(0)
		moved := r.spawn(func() error {
			_, err := r.groups(adapters.GroupQuestions{}, media).MoveQuestion(r.ctx, in)
			return err
		})
		pid := r.await(locked, moved)
		deleted := r.spawn(func() error {
			_, err := r.media().SoftDelete(r.ctx, mediadomain.DeleteInput{ID: r.asset, ActorID: r.author, Now: time.Now()})
			return err
		})
		if !r.blockedBy(pid) {
			open()
			t.Fatal("the media delete never waited on the move's asset lock")
		}
		open()

		if err := <-moved; err != nil {
			t.Fatalf("move: %v", err)
		}
		if err := <-deleted; !errors.Is(err, mediadomain.ErrReferenced) {
			t.Fatalf("the delete must be refused as referenced: %v", err)
		}
		var kept, memberAsset *string
		if err := r.pool.QueryRow(r.ctx, `SELECT a.id::text, q.media_asset_id::text FROM app.questions q JOIN app.media_assets a ON a.id=q.media_asset_id
			WHERE q.id=$1 AND a.deleted_at IS NULL`, r.stored().Bundle.Group.Members[0].QuestionID).Scan(&kept, &memberAsset); err != nil || kept == nil || *kept != r.asset {
			t.Fatalf("the member points at a deleted asset: %v, %v", kept, err)
		}
	})

	t.Run("an asset deleted before the move", func(t *testing.T) {
		r := newRaceWorld(t, raceOptions{bankImage: true})
		if _, err := r.pool.Exec(r.ctx, `UPDATE app.media_assets SET deleted_at=now() WHERE id=$1`, r.asset); err != nil {
			t.Fatal(err)
		}
		before := slices.Clone(r.order())

		_, err := r.groups(adapters.GroupQuestions{}, r.media()).MoveQuestion(r.ctx, r.copyIn(0))

		if !errors.Is(err, mediadomain.ErrNotFound) {
			t.Fatalf("a deleted asset must fail the move as a missing one: %v", err)
		}
		if !slices.Equal(r.order(), before) {
			t.Fatal("a refused move wrote to the group")
		}
	})
}
