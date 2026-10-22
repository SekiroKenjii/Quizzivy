//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/shared/access"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const guardRule = "media_assets_purged_binding"

func replaceAt(t *testing.T, w *replacementWorld, old string, kind domain.Kind, size, quota int64, now time.Time) (domain.ReplaceResult, error) {
	t.Helper()
	in := w.input(old, w.a, kind, size, quota)
	in.Asset.Now = now
	return w.repo.Replace(context.Background(), in)
}

func nameInTx(ctx context.Context, tx pgx.Tx, owner, asset string) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.questions(type,prompt,points,created_by,owner_id,media_asset_id,media_asset_kind,audio_allow_seek,audio_show_transcript_after) VALUES('short_answer','Hỏi',1,$1,$1,$2,'audio',false,false)`, owner, asset)
	return err
}

func freezeIn(ctx context.Context, tx pgx.Tx, author, asset string) error {
	var test, version, section string
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề đã xuất bản', 'published', 1, $1, $1) RETURNING id::text`, author).Scan(&test); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10.00, $2) RETURNING id::text`, test, author).Scan(&version); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần nghe') RETURNING id::text`, version).Scan(&section); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after) VALUES ($1, 0, 'short_answer', 'Nghe', 5.00, $2, 'audio', false, true)`, section, asset)
	return err
}

func guardViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23514" && pg.ConstraintName == guardRule
}

func TestAReplacedFileAPublishedVersionHoldsKeepsCountingTowardTheQuota(t *testing.T) {
	w := newReplacementWorld(t, nil)
	ctx := context.Background()
	old := w.asset(t, w.a, domain.KindAudio, 20)
	if err := publishVersionUsing(ctx, w.pool, w.a, old, true); err != nil {
		t.Fatal(err)
	}

	if _, err := replaceAt(t, w, old, domain.KindAudio, 30, 49, epoch); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("replacing into room for the new file alone: %v, want the quota refusal: the version still holds the old 20 bytes", err)
	}
	if w.text(t, `SELECT coalesce(replaced_by::text,'') FROM app.media_assets WHERE id=$1`, old) != "" || w.dated(t, old) != nil {
		t.Fatal("the refused replacement left the old file replaced or dated")
	}

	out, err := replaceAt(t, w, old, domain.KindAudio, 30, 50, epoch)
	if err != nil {
		t.Fatalf("replacing into room for both files: %v", err)
	}
	if len(out.Previous.UsedIn) != 1 || out.Previous.DeleteAt != nil || out.Previous.Asset.ID != old || out.Previous.Asset.Bytes != 20 {
		t.Errorf("the old file is answered as %+v, want it held by the one version with no removal time", out.Previous)
	}
	usage := w.usage(t, w.a)
	if usage.AudioBytes != 30 || usage.HeldBytes != 20 || usage.PendingBytes != 0 || usage.PendingDeleteAt != nil || usage.CountedBytes() != 50 {
		t.Errorf("usage is %+v, want 30 in the library and 20 held, none pending", usage)
	}
	if w.dated(t, old) != nil {
		t.Error("a file a version holds carries a removal date")
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := repositories.RequireQuota(ctx, tx, repositories.QuotaCheck{OwnerID: w.a, Add: 1, Quota: 50}); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Errorf("an upload into a library holding 50 of 50 counted: %v, want the quota refusal", err)
	}
}

func TestAReplacedFileNothingNamesIsDatedFromTheReplaceAndDoesNotCount(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindImage, 20)

	out, err := replaceAt(t, w, old, domain.KindImage, 30, 30, epoch)
	if err != nil {
		t.Fatalf("replacing into room for the new file alone: %v, want the old file out of the quota", err)
	}
	due := epoch.Add(domain.RetiredFileGrace)
	if out.Previous.DeleteAt == nil || !out.Previous.DeleteAt.Equal(due) || len(out.Previous.UsedIn) != 0 {
		t.Errorf("the old file is answered as %+v, want its removal at %v and no version", out.Previous, due)
	}
	if at := w.dated(t, old); at == nil || !at.Equal(epoch) {
		t.Errorf("the old file is dated %v, want %v", at, epoch)
	}
	usage := w.usage(t, w.a)
	if usage.ImageBytes != 30 || usage.HeldBytes != 0 || usage.PendingBytes != 20 || usage.PendingDeleteAt == nil || !usage.PendingDeleteAt.Equal(due) || usage.CountedBytes() != 30 {
		t.Errorf("usage is %+v, want 30 in the library, 20 pending for %v and none held", usage, due)
	}
}

func TestADeletedFileIsDatedFromTheDeleteUnlessAQuestionStillNamesIt(t *testing.T) {
	w := newReplacementWorld(t, nil)
	free := w.asset(t, w.a, domain.KindImage, 20)
	used := w.asset(t, w.a, domain.KindImage, 40)
	w.question(t, w.a, used, domain.KindImage, nil)
	due := epoch.Add(domain.RetiredFileGrace)

	gone, err := w.repo.SoftDelete(context.Background(), domain.DeleteInput{ID: free, ActorID: w.a, Now: epoch})
	if err != nil {
		t.Fatal(err)
	}
	if gone.Asset.ID != free || gone.DeleteAt == nil || !gone.DeleteAt.Equal(due) || len(gone.UsedIn) != 0 {
		t.Errorf("the delete answers %+v, want the file and its removal at %v", gone, due)
	}
	kept, err := w.repo.SoftDelete(context.Background(), domain.DeleteInput{ID: used, ActorID: w.a, Now: epoch})
	if err != nil {
		t.Fatal(err)
	}
	if kept.DeleteAt != nil || w.dated(t, used) != nil {
		t.Errorf("the delete of a file a live question names answers %v and dates it %v, want neither", kept.DeleteAt, w.dated(t, used))
	}
	usage := w.usage(t, w.a)
	if usage.HeldBytes != 40 || usage.PendingBytes != 20 || usage.PendingDeleteAt == nil || !usage.PendingDeleteAt.Equal(due) || usage.CountedBytes() != 40 {
		t.Errorf("usage is %+v, want 40 held, 20 pending for %v and nothing in the library", usage, due)
	}
}

func TestTheSweepDatesAFileWhoseLastNameWentAndCancelsAWaitSomethingNamesAgain(t *testing.T) {
	w := newReplacementWorld(t, nil)
	app := application.New(w.repo, newRetentionStore(), audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	foreign := w.question(t, w.b, old, domain.KindAudio, nil)
	out, err := replaceAt(t, w, old, domain.KindAudio, 30, 100, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if out.Previous.DeleteAt != nil || w.dated(t, old) != nil {
		t.Fatalf("the old file is answered with %v and dated %v while another owner's question names it", out.Previous.DeleteAt, w.dated(t, old))
	}
	at := epoch.Add(domain.RetiredFileGrace)
	sweepAt(t, app, at)
	if w.dated(t, old) != nil {
		t.Error("the sweep dated a file another owner's question names")
	}

	w.exec(t, `UPDATE app.questions SET deleted_at=$2 WHERE id=$1`, foreign, epoch)
	sweepAt(t, app, at)
	if got := w.dated(t, old); got == nil || !got.Equal(at) {
		t.Errorf("after the last name went the sweep dated the file %v, want %v", got, at)
	}

	w.exec(t, `UPDATE app.questions SET deleted_at=NULL WHERE id=$1`, foreign)
	if w.dated(t, old) == nil {
		t.Fatal("an update that leaves the binding as it was cleared the date")
	}
	sweepAt(t, app, at)
	if w.dated(t, old) != nil {
		t.Error("the sweep left a date on a file something names again")
	}
}

func TestTheSweepRemovesAFileSevenDaysAfterNothingNamesItAndNotBefore(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	app := application.New(w.repo, store, audioProbe{})
	gone := w.asset(t, w.a, domain.KindImage, 20)
	w.retire(t, gone, epoch)
	library := w.asset(t, w.a, domain.KindImage, 30)
	goneKey, libraryKey := w.key(t, gone), w.key(t, library)
	due := epoch.Add(domain.RetiredFileGrace)

	sweepAt(t, app, due.Add(-time.Second))
	if store.deleted(goneKey) != 0 || w.purged(t, gone) {
		t.Fatal("the sweep removed a file one second before its seven days were up")
	}
	swept := sweepAt(t, app, due)
	if swept.Removed < 1 || store.deleted(goneKey) != 1 || !w.purged(t, gone) {
		t.Fatalf("at seven days the sweep removed %d files and deleted the object %d times, want this file once", swept.Removed, store.deleted(goneKey))
	}
	if store.deleted(libraryKey) != 0 || w.purged(t, library) {
		t.Error("the sweep touched a file still in the library")
	}
	if at := w.text(t, `SELECT purged_at::text FROM app.media_assets WHERE id=$1`, gone); at == "" {
		t.Error("the removed file's row carries no time")
	}
	if audited := w.scalar(t, `SELECT count(*) FROM app.audit_log WHERE action='media.purged' AND entity_id=$1 AND actor_user_id IS NULL`, gone); audited != 1 {
		t.Errorf("the removal wrote %d audit rows, want 1 with no actor", audited)
	}
	sweepAt(t, app, due.Add(time.Hour))
	if store.deleted(goneKey) != 1 {
		t.Errorf("a later sweep deleted the object %d times in all, want 1", store.deleted(goneKey))
	}
	if usage := w.usage(t, w.a); usage.PendingBytes != 0 || usage.HeldBytes != 0 || usage.ImageBytes != 30 {
		t.Errorf("usage after the removal is %+v, want only the library's 30", usage)
	}
}

func TestAFileNamedAgainBeforeTheSweepTakesItIsNotRemoved(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.supersede(t, old, epoch)
	due := epoch.Add(domain.RetiredFileGrace)

	if err := publishVersionUsing(context.Background(), w.pool, w.a, old, true); err != nil {
		t.Fatalf("publishing a version of a file waiting for its removal: %v", err)
	}
	if w.dated(t, old) != nil {
		t.Error("freezing the file into a version left its removal date")
	}
	sweepAt(t, application.New(w.repo, store, audioProbe{}), due)
	if store.deleted(w.key(t, old)) != 0 || w.purged(t, old) {
		t.Fatal("the sweep removed a file a published version names")
	}
	if usage := w.usage(t, w.a); usage.HeldBytes != 20 || usage.PendingBytes != 0 {
		t.Errorf("usage is %+v, want the file held by its version", usage)
	}

	w.exec(t, `UPDATE app.media_assets SET unreferenced_at=$2 WHERE id=$1`, old, epoch)
	outcome, err := w.repo.Purge(context.Background(), domain.PurgeInput{ID: old, Due: due.Add(-domain.RetiredFileGrace), Now: due, Remove: func(context.Context, string) error {
		t.Error("the object was deleted although a version names the file")
		return nil
	}})
	if err != nil || outcome != domain.PurgeNamed {
		t.Fatalf("purging a dated file a version names: %v, %v, want PurgeNamed", outcome, err)
	}
	if w.dated(t, old) != nil || w.purged(t, old) {
		t.Error("the purge left a date on, or removed, a file a version names")
	}
}

func TestAPublishThatWaitsForTheSweepFindsTheFileGone(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	store.gate(t)
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.supersede(t, old, epoch)
	due := epoch.Add(domain.RetiredFileGrace)

	var wg sync.WaitGroup
	var swept command.Swept
	var publishErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		swept, _ = app.Commands.Sweep.Handle(context.Background(), command.Sweep{Now: due})
	}()
	select {
	case <-store.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the sweep never reached the object")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		publishErr = publishVersionUsing(context.Background(), w.pool, w.a, old, true)
	}()
	w.waitForLockWait(t, "purged_at IS NOT NULL FROM app.media_assets WHERE id = $1 FOR UPDATE")
	store.open()
	wg.Wait()

	if swept.Removed < 1 || !w.purged(t, old) {
		t.Fatalf("the sweep removed %d files and purged=%v, want this file removed", swept.Removed, w.purged(t, old))
	}
	if !errors.Is(publishErr, domain.ErrNotFound) {
		t.Errorf("the publish that waited for the sweep: %v, want ErrNotFound", publishErr)
	}
	if named := w.scalar(t, `SELECT count(*) FROM app.test_version_questions WHERE media_asset_id=$1`, old); named != 0 {
		t.Errorf("%d version questions name a file whose object is gone", named)
	}
}

func TestASweepThatFindsAPublishHoldingTheFileLeavesIt(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.supersede(t, old, epoch)
	key := w.key(t, old)
	due := epoch.Add(domain.RetiredFileGrace)
	ctx := context.Background()

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := repositories.LockForVersionUse(ctx, tx, old); err != nil {
		t.Fatal(err)
	}
	sweepAt(t, app, due)
	if store.deleted(key) != 0 || w.purged(t, old) {
		t.Fatal("the sweep removed a file a publish was holding")
	}
	if err := freezeIn(ctx, tx, w.a, old); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	sweepAt(t, app, due)
	if store.deleted(key) != 0 || w.purged(t, old) {
		t.Fatal("the sweep removed a file the publish then froze")
	}
	if usage := w.usage(t, w.a); usage.HeldBytes != 20 || usage.PendingBytes != 0 {
		t.Errorf("usage is %+v, want the file held by the version", usage)
	}
}

func TestAWriterThatNamesAFileWaitsForTheSweepAndIsRefusedOnceItIsGone(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	store.gate(t)
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.supersede(t, old, epoch)
	due := epoch.Add(domain.RetiredFileGrace)

	var wg sync.WaitGroup
	var writeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = app.Commands.Sweep.Handle(context.Background(), command.Sweep{Now: due})
	}()
	select {
	case <-store.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the sweep never reached the object")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, writeErr = w.pool.Exec(context.Background(), `INSERT INTO app.questions(type,prompt,points,created_by,owner_id,media_asset_id,media_asset_kind,audio_allow_seek,audio_show_transcript_after) VALUES('short_answer','Hỏi',1,$1,$1,$2,'audio',false,false)`, w.a, old)
	}()
	w.waitForLockWait(t, "INSERT INTO app.questions")
	store.open()
	wg.Wait()

	if !guardViolation(writeErr) {
		t.Errorf("the write that named a file the sweep was removing: %v, want the %s refusal", writeErr, guardRule)
	}
	if !w.purged(t, old) {
		t.Error("the sweep did not remove the file")
	}
	if named := w.scalar(t, `SELECT count(*) FROM app.questions WHERE media_asset_id=$1`, old); named != 0 {
		t.Errorf("%d questions name a file whose object is gone", named)
	}
}

func TestAWriterThatNamesAFileFirstKeepsItFromTheSweep(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.supersede(t, old, epoch)
	key := w.key(t, old)
	due := epoch.Add(domain.RetiredFileGrace)
	ctx := context.Background()

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameInTx(ctx, tx, w.a, old); err != nil {
		t.Fatal(err)
	}
	sweepAt(t, app, due)
	if store.deleted(key) != 0 || w.purged(t, old) {
		t.Fatal("the sweep removed a file a write was naming")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if w.dated(t, old) != nil {
		t.Error("naming the file left its removal date")
	}
	sweepAt(t, app, due)
	if store.deleted(key) != 0 || w.purged(t, old) {
		t.Fatal("the sweep removed a file a question names")
	}
}

func TestTwoSweepsDoNotRemoveAFileTwice(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	store.gate(t)
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.retire(t, old, epoch)
	key := w.key(t, old)
	due := epoch.Add(domain.RetiredFileGrace)

	done := make(chan command.Swept, 1)
	go func() {
		swept, _ := app.Commands.Sweep.Handle(context.Background(), command.Sweep{Now: due})
		done <- swept
	}()
	select {
	case <-store.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first sweep never reached the object")
	}
	patient, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	second, err := app.Commands.Sweep.Handle(patient, command.Sweep{Now: due})
	if err != nil || second.Failed != 0 {
		t.Fatalf("the second sweep: %+v, %v, want it to pass over the file the first holds", second, err)
	}
	if store.deleted(key) != 0 {
		t.Fatal("the second sweep deleted an object the first was deleting")
	}
	store.open()
	first := <-done
	if first.Removed < 1 || store.deleted(key) != 1 || !w.purged(t, old) {
		t.Errorf("the first sweep removed %d files and deleted the object %d times, want this file once", first.Removed, store.deleted(key))
	}
}

func TestAFailedObjectDeleteLeavesTheFileForTheNextSweep(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.retire(t, old, epoch)
	key := w.key(t, old)
	due := epoch.Add(domain.RetiredFileGrace)

	store.mu.Lock()
	store.fail = errors.New("storage unreachable")
	store.mu.Unlock()
	failed := sweepAt(t, app, due)
	if failed.Failed < 1 || failed.Cause == nil || w.purged(t, old) || store.deleted(key) != 0 {
		t.Fatalf("a failed delete: %+v purged=%v deleted=%d, want it counted, the file left and nothing deleted", failed, w.purged(t, old), store.deleted(key))
	}
	if at := w.dated(t, old); at == nil || !at.Equal(epoch) {
		t.Errorf("the failed delete left the file dated %v, want %v", at, epoch)
	}
	retried := sweepAt(t, app, due)
	if retried.Removed < 1 || !w.purged(t, old) || store.deleted(key) != 1 {
		t.Errorf("the retry removed %d files and deleted the object %d times, want this file once", retried.Removed, store.deleted(key))
	}
}

func TestAFileWhoseObjectIsGoneIsGoneToEveryReader(t *testing.T) {
	w := newReplacementWorld(t, nil)
	ctx := context.Background()
	scope := access.Scope{UserID: w.a}
	deleted := w.asset(t, w.a, domain.KindAudio, 20)
	replacedBy := w.asset(t, w.a, domain.KindAudio, 25)
	replaced := w.asset(t, w.a, domain.KindAudio, 30)
	w.exec(t, `UPDATE app.media_assets SET deleted_at=$2,unreferenced_at=$2,purged_at=$2 WHERE id=$1`, deleted, epoch)
	w.exec(t, `UPDATE app.media_assets SET replaced_by=$2,unreferenced_at=$3,purged_at=$3 WHERE id=$1`, replaced, replacedBy, epoch)

	for name, id := range map[string]string{"a deleted file": deleted, "a replaced file": replaced} {
		if _, err := w.repo.Get(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Get of %s: %v, want ErrNotFound", name, err)
		}
		if readable, err := w.repo.Readable(ctx, scope, []string{id}); err != nil || len(readable) != 0 {
			t.Errorf("Readable of %s: %v, %v, want nothing", name, readable, err)
		}
		if _, err := w.repo.SoftDelete(ctx, domain.DeleteInput{ID: id, ActorID: w.a, Now: epoch}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("SoftDelete of %s: %v, want ErrNotFound", name, err)
		}
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := repositories.LockForVersionUse(ctx, tx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("LockForVersionUse of %s: %v, want ErrNotFound", name, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if usage := w.usage(t, w.a); usage.AudioBytes != 25 || usage.HeldBytes != 0 || usage.PendingBytes != 0 || usage.PendingDeleteAt != nil {
		t.Errorf("usage is %+v, want only the library's 25", usage)
	}
}

func TestNothingCanNameAFileWhoseObjectIsGone(t *testing.T) {
	w := newReplacementWorld(t, nil)
	ctx := context.Background()
	gone := w.asset(t, w.a, domain.KindAudio, 20)
	goneImage := w.asset(t, w.a, domain.KindImage, 20)
	for _, id := range []string{gone, goneImage} {
		w.exec(t, `UPDATE app.media_assets SET deleted_at=$2,unreferenced_at=$2,purged_at=$2 WHERE id=$1`, id, epoch)
	}
	live := w.asset(t, w.a, domain.KindAudio, 20)
	liveQuestion := w.question(t, w.a, live, domain.KindAudio, nil)
	group := w.group(t, w.a, nil)
	stimulus := w.id(t, `INSERT INTO app.group_stimuli(group_id,ordinal,title,content) VALUES($1,0,'Ngữ liệu',$2) RETURNING id::text`, group, []byte(`{"format":"semantic_v1","blocks":[]}`))

	for name, write := range map[string]func(pgx.Tx) error{
		"a question": func(tx pgx.Tx) error { return nameInTx(ctx, tx, w.a, gone) },
		"a group's recording": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO app.group_recordings(group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit) VALUES($1,$2,2,false,false)`, group, gone)
			return err
		},
		"a group's material": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO app.group_stimulus_assets(stimulus_id,group_id,media_asset_id,media_asset_kind) VALUES($1,$2,$3,'image')`, stimulus, group, goneImage)
			return err
		},
		"a version's question": func(tx pgx.Tx) error { return freezeIn(ctx, tx, w.a, gone) },
		"a question's prose": func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO app.question_media_references(question_id,field,media_asset_id,media_asset_kind) VALUES($1,'prompt',$2,'image')`, liveQuestion, goneImage)
			return err
		},
		"a version's prose": func(tx pgx.Tx) error {
			if err := freezeIn(ctx, tx, w.a, live); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO app.test_version_question_media_references(test_version_question_id,field,media_asset_id,media_asset_kind) SELECT id,'explanation',$2,'image' FROM app.test_version_questions WHERE media_asset_id=$1`, live, goneImage)
			return err
		},
	} {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := write(tx); !guardViolation(err) {
			t.Errorf("naming the file from %s: %v, want the %s refusal", name, err, guardRule)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheGuardIsOnEveryTableThatNamesAFile(t *testing.T) {
	w := newReplacementWorld(t, nil)
	names := func(query string) []string {
		rows, err := w.pool.Query(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	triggers := names(`SELECT tgrelid::regclass::text FROM pg_trigger WHERE tgfoid='app.media_asset_binding_guard'::regproc AND NOT tgisinternal ORDER BY 1`)
	// Every table with a media_asset_id column names a file, so a new one is
	// held to the guard without this list being edited (QC-R6D-1).
	want := names(`SELECT 'app.' || c.table_name FROM information_schema.columns c JOIN information_schema.tables t USING (table_schema, table_name) WHERE c.table_schema = 'app' AND c.column_name = 'media_asset_id' AND t.table_type = 'BASE TABLE' ORDER BY 1`)
	if len(want) < 8 || !slices.Equal(triggers, want) {
		t.Errorf("the guard is on %v, want %v", triggers, want)
	}
}

func TestARetiredFilesFiguresBelongToItsOwnersLibraryAlone(t *testing.T) {
	w := newReplacementWorld(t, nil)
	mine := w.asset(t, w.a, domain.KindImage, 20)
	theirs := w.asset(t, w.b, domain.KindImage, 50)
	w.retire(t, mine, epoch)
	w.retire(t, theirs, epoch.Add(time.Hour))
	w.question(t, w.b, theirs, domain.KindImage, nil)

	if usage := w.usage(t, w.a); usage.PendingBytes != 20 || usage.HeldBytes != 0 || usage.PendingDeleteAt == nil || !usage.PendingDeleteAt.Equal(epoch.Add(domain.RetiredFileGrace)) {
		t.Errorf("the first owner's usage is %+v, want 20 pending", usage)
	}
	if usage := w.usage(t, w.b); usage.HeldBytes != 50 || usage.PendingBytes != 0 || usage.PendingDeleteAt != nil {
		t.Errorf("the second owner's usage is %+v, want 50 held", usage)
	}
	if usage, err := w.repo.Usage(context.Background(), access.Scope{}); err != nil || usage.HeldBytes != 0 || usage.PendingBytes != 0 {
		t.Errorf("a scope that reaches nothing sees %+v, %v", usage, err)
	}
	if usage, err := w.repo.Usage(context.Background(), access.Scope{All: true}); err != nil || usage.HeldBytes < 50 || usage.PendingBytes < 20 {
		t.Errorf("scope.all sees %+v, %v, want at least both owners' figures", usage, err)
	}
}

func TestASweepWhoseTimeEndsRightAfterTheObjectIsDeletedStillMarksTheRow(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.after = cancel
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.retire(t, old, epoch)
	key := w.key(t, old)

	swept, err := app.Commands.Sweep.Handle(ctx, command.Sweep{Now: epoch.Add(domain.RetiredFileGrace)})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("the storage delete did not end the sweep's time")
	}
	if store.deleted(key) != 1 || !w.purged(t, old) || len(swept.Unmarked) != 0 {
		t.Errorf("deleted %d times, purged=%v, unmarked=%v: want the row marked although the sweep's time ended with the delete", store.deleted(key), w.purged(t, old), swept.Unmarked)
	}
	if audited := w.scalar(t, `SELECT count(*) FROM app.audit_log WHERE action='media.purged' AND entity_id=$1`, old); audited != 1 {
		t.Errorf("the removal wrote %d audit rows, want 1", audited)
	}
}

func TestAnObjectDeletedWhoseRowCannotBeMarkedIsReportedAndMarkedByTheNextSweep(t *testing.T) {
	w := newReplacementWorld(t, nil)
	store := newRetentionStore()
	app := application.New(w.repo, store, audioProbe{})
	old := w.asset(t, w.a, domain.KindAudio, 20)
	w.retire(t, old, epoch)
	key := w.key(t, old)
	due := epoch.Add(domain.RetiredFileGrace)
	store.after = func() {
		if _, err := w.pool.Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=current_database() AND state='idle in transaction' AND query LIKE 'SELECT (EXISTS (SELECT 1 FROM app.questions r%'`); err != nil {
			t.Error(err)
		}
	}

	swept := sweepAt(t, app, due)
	if swept.Failed != 1 || len(swept.Unmarked) != 1 {
		t.Fatalf("the sweep counted %+v, want the one file failed and reported unmarked", swept)
	}
	lost := swept.Unmarked[0]
	if lost.ID != old || lost.Cause == nil {
		t.Errorf("the report is %+v, want the file's id and a cause", lost)
	}
	if store.deleted(key) != 1 || w.purged(t, old) {
		t.Fatalf("deleted %d times, purged=%v, want the object gone and the row not marked", store.deleted(key), w.purged(t, old))
	}

	store.mu.Lock()
	store.after = nil
	store.mu.Unlock()
	retried := sweepAt(t, app, due)
	if retried.Removed < 1 || len(retried.Unmarked) != 0 || !w.purged(t, old) {
		t.Errorf("the next sweep counted %+v purged=%v, want the row marked", retried, w.purged(t, old))
	}
	if store.deleted(key) != 2 {
		t.Errorf("the object was deleted %d times in all, want the second delete of a gone object to succeed", store.deleted(key))
	}
}
