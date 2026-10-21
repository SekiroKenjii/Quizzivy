//go:build integration

package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

func (w *workerWorld) sheetJob(t *testing.T, accounts int) domain.ImportJob {
	t.Helper()
	file, _ := w.students(accounts, "")
	job := w.submitRows(t, domain.ImportSheet, file)
	w.runOnce(t, w.runner())
	if done := w.job(t, job.ID); done.Status != domain.ImportDone || done.Created != accounts || w.sealed(t, job.ID) != accounts {
		t.Fatalf("job = %+v with %d sealed, want done with %d accounts and as many sealed passwords", done, w.sealed(t, job.ID), accounts)
	}
	return w.job(t, job.ID)
}

func (w *workerWorld) expireAt(t *testing.T, jobID string, at time.Time) {
	t.Helper()
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_jobs SET sheet_expires_at = $2 WHERE id = $1::uuid`, jobID, at); err != nil {
		t.Fatal(err)
	}
}

func (w *workerWorld) open(jobID string) domain.SecretOpener {
	return func(ordinal int, sealed []byte) (string, error) {
		password, err := w.sealer.Open(sealed, domain.SheetAAD(jobID, ordinal))
		return string(password), err
	}
}

func TestTwoRequestsForTheSameSheetAtOnceGiveItToOne(t *testing.T) {
	w := newWorkerWorld(t)
	job := w.sheetJob(t, 3)
	const askers = 6
	var (
		start     = make(chan struct{})
		group     sync.WaitGroup
		mu        sync.Mutex
		got, none int
		other     []error
	)
	for range askers {
		group.Go(func() {
			<-start
			sheet, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: w.req(), ID: job.ID})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && len(sheet) == 3:
				got++
			case errors.Is(err, domain.ErrSheetUnavailable):
				none++
			default:
				other = append(other, err)
			}
		})
	}
	close(start)
	group.Wait()

	if got != 1 || none != askers-1 || len(other) != 0 {
		t.Errorf("%d request(s) got the sheet, %d were refused, %v failed; want exactly one to get it and the rest refused", got, none, other)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user_import.sheet_downloaded' AND entity_id = $1::uuid`, job.ID); n != 1 {
		t.Errorf("%d download(s) audited, want 1", n)
	}
	if n := w.sealed(t, job.ID); n != 0 {
		t.Errorf("%d sealed password(s) survive the download", n)
	}
}

func TestARequestThatArrivesWhileAnotherHoldsTheSheetGetsNothing(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	job := w.sheetJob(t, 3)
	entered, release := make(chan struct{}, 2), make(chan struct{})
	gate := func() domain.SecretOpener {
		var once sync.Once
		return func(ordinal int, sealed []byte) (string, error) {
			once.Do(func() { entered <- struct{}{} })
			<-release
			return w.open(job.ID)(ordinal, sealed)
		}
	}
	type taken struct {
		sheet []domain.SheetRow
		err   error
	}
	ask := func() chan taken {
		out := make(chan taken, 1)
		go func() {
			sheet, err := w.queue.TakeSheet(ctx, w.req(), job.ID, time.Now(), gate())
			out <- taken{sheet, err}
		}()
		return out
	}

	first := ask()
	<-entered
	second := ask()
	select {
	case <-entered:
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	one, two := <-first, <-second

	if one.err != nil || len(one.sheet) != 3 {
		t.Errorf("the first request = %d row(s), %v, want the sheet", len(one.sheet), one.err)
	}
	if !errors.Is(two.err, domain.ErrSheetUnavailable) || len(two.sheet) != 0 {
		t.Errorf("the second request = %d row(s), %v, want ErrSheetUnavailable", len(two.sheet), two.err)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user_import.sheet_downloaded' AND entity_id = $1::uuid`, job.ID); n != 1 {
		t.Errorf("%d download(s) audited, want 1", n)
	}
}

func TestAJobThatEndedCannotBeWrittenToByTheAttemptThatEndedIt(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	file, _ := w.students(2, "")

	done := w.submitRows(t, domain.ImportEmailLink, file)
	w.runOnce(t, w.runner())

	file, _ = w.students(2, "")
	failed := w.submitRows(t, domain.ImportEmailLink, file)
	claimed, err := w.queue.Claim(ctx)
	if err != nil || claimed.ID != failed.ID {
		t.Fatalf("Claim = %+v, %v", claimed, err)
	}
	if err := w.queue.Fail(ctx, claimed.ID, claimed.Attempts, "X"); err != nil {
		t.Fatal(err)
	}

	for name, id := range map[string]string{"done": done.ID, "failed": claimed.ID} {
		ended := w.job(t, id)
		batch := domain.ImportBatch{JobID: id, Attempt: ended.Attempts, By: w.importer, Delivery: domain.ImportEmailLink, Now: time.Now()}
		if _, err := w.queue.Settle(ctx, batch, nil); !errors.Is(err, domain.ErrImportLeaseLost) {
			t.Errorf("%s: Settle under the attempt that ended it = %v, want ErrImportLeaseLost", name, err)
		}
		for verb, err := range map[string]error{
			"Fail":    w.queue.Fail(ctx, id, ended.Attempts, "Y"),
			"Retry":   w.queue.Retry(ctx, id, ended.Attempts),
			"Release": w.queue.Release(ctx, id, ended.Attempts),
		} {
			if !errors.Is(err, domain.ErrImportLeaseLost) {
				t.Errorf("%s: %s under the attempt that ended it = %v, want ErrImportLeaseLost", name, verb, err)
			}
		}
		if still := w.job(t, id); still.Status != ended.Status || still.ErrorCode != ended.ErrorCode || still.Attempts != ended.Attempts || still.Created != ended.Created {
			t.Errorf("%s: job = %+v, want it untouched, as %+v", name, still, ended)
		}
	}
}

func TestTheSweepErasesTheSealedPasswordsOfASheetNobodyTookInTime(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	base := time.Now().Add(-300 * 24 * time.Hour).Truncate(time.Second)

	lapsed := w.sheetJob(t, 3)
	w.expireAt(t, lapsed.ID, base)
	waiting := w.sheetJob(t, 3)
	w.expireAt(t, waiting.ID, base.Add(2*time.Hour))
	running := w.sheetJob(t, 3)
	if _, err := w.admin.Exec(ctx, `UPDATE app.user_import_jobs SET sheet_expires_at = NULL WHERE id = $1::uuid`, running.ID); err != nil {
		t.Fatal(err)
	}
	taken := w.sheetJob(t, 3)
	if _, err := w.app.Commands.DownloadUserImportSheet.Handle(ctx, command.DownloadUserImportSheet{Request: w.req(), ID: taken.ID}); err != nil {
		t.Fatal(err)
	}
	w.expireAt(t, taken.ID, base)
	file, _ := w.students(2, "")
	byEmail := w.submitRows(t, domain.ImportEmailLink, file)
	w.runOnce(t, w.runner())
	before := w.job(t, lapsed.ID)

	erased, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 100)
	if err != nil || erased != 3 {
		t.Fatalf("EraseExpiredSheets = %d, %v, want the 3 passwords of the lapsed sheet", erased, err)
	}

	if n := w.sealed(t, lapsed.ID); n != 0 {
		t.Errorf("%d sealed password(s) of the lapsed sheet remain", n)
	}
	if after := w.job(t, lapsed.ID); after.Status != before.Status || after.Created != before.Created || after.Skipped != before.Skipped ||
		after.SheetDownloadedAt != nil || after.SheetExpiresAt == nil || !after.SheetExpiresAt.Equal(*before.SheetExpiresAt) {
		t.Errorf("job = %+v, want it as it was, %+v", after, before)
	}
	if got := w.outcomes(t, lapsed.ID); got["created"] != 3 || len(got) != 1 {
		t.Errorf("outcomes = %v, want the 3 created rows kept", got)
	}
	if n := w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND user_id IS NOT NULL AND email <> '' AND full_name <> ''`, lapsed.ID); n != 3 {
		t.Errorf("%d row(s) keep their account, name and address, want 3", n)
	}
	for name, id := range map[string]string{"one not yet due": waiting.ID, "one with no deadline": running.ID} {
		if n := w.sealed(t, id); n != 3 {
			t.Errorf("%s: %d sealed password(s), want the 3 left alone", name, n)
		}
	}
	if w.sealed(t, byEmail.ID) != 0 || w.sealed(t, taken.ID) != 0 {
		t.Error("a job with nothing sealed gained a password")
	}

	if again, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 100); err != nil || again != 0 {
		t.Errorf("a second sweep = %d, %v, want nothing more to erase", again, err)
	}
	if next, err := w.queue.EraseExpiredSheets(ctx, base.Add(2*time.Hour), 100); err != nil || next != 3 || w.sealed(t, waiting.ID) != 0 {
		t.Errorf("a sweep at the deadline = %d, %v with %d left, want the 3 passwords due at that instant gone", next, err, w.sealed(t, waiting.ID))
	}
	if n := w.sealed(t, running.ID); n != 3 {
		t.Errorf("a job with no deadline has %d sealed password(s), want 3", n)
	}
}

func TestTheSweepWorksThroughBatchesAndReportsWhenItHasCaughtUp(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	base := time.Now().Add(-290 * 24 * time.Hour).Truncate(time.Second)
	big := w.sheetJob(t, 5)
	w.expireAt(t, big.ID, base)

	var got []int
	for range 5 {
		n, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 2)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, n)
		if n < 2 {
			break
		}
	}

	if len(got) != 3 || got[0] != 2 || got[1] != 2 || got[2] != 1 || w.sealed(t, big.ID) != 0 {
		t.Errorf("batches = %v with %d left, want 2, 2, 1 and none left", got, w.sealed(t, big.ID))
	}
}

func TestASweepLeavesASheetBeingTakenAloneAndTheDownloadStillGetsItsPasswords(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	base := time.Now().Add(-280 * 24 * time.Hour).Truncate(time.Second)
	job := w.sheetJob(t, 3)
	w.expireAt(t, job.ID, base)

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	slow := func(ordinal int, sealed []byte) (string, error) {
		once.Do(func() { close(entered) })
		<-release
		return w.open(job.ID)(ordinal, sealed)
	}
	type taken struct {
		sheet []domain.SheetRow
		err   error
	}
	result := make(chan taken, 1)
	go func() {
		sheet, err := w.queue.TakeSheet(ctx, w.req(), job.ID, base.Add(-time.Hour), slow)
		result <- taken{sheet, err}
	}()
	<-entered

	skipped, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 100)
	close(release)
	got := <-result

	if err != nil || skipped != 0 {
		t.Errorf("a sweep during a download = %d, %v, want it to skip the job it cannot lock", skipped, err)
	}
	if got.err != nil || len(got.sheet) != 3 {
		t.Fatalf("the download = %d row(s), %v, want its 3 passwords", len(got.sheet), got.err)
	}
	if n := w.sealed(t, job.ID); n != 0 {
		t.Errorf("%d sealed password(s) survive the download", n)
	}
	if later, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 100); err != nil || later != 0 {
		t.Errorf("the next sweep = %d, %v, want nothing left for it", later, err)
	}
}

func TestASheetWhoseSealedPasswordsAreGoneIsNotHandedOutEmptyAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	base := time.Now().Add(-270 * 24 * time.Hour).Truncate(time.Second)
	job := w.sheetJob(t, 3)
	w.expireAt(t, job.ID, base)
	if erased, err := w.queue.EraseExpiredSheets(ctx, base.Add(time.Hour), 100); err != nil || erased != 3 {
		t.Fatalf("EraseExpiredSheets = %d, %v", erased, err)
	}

	sheet, err := w.queue.TakeSheet(ctx, w.req(), job.ID, base.Add(-time.Hour), w.open(job.ID))

	if !errors.Is(err, domain.ErrSheetUnavailable) || len(sheet) != 0 {
		t.Errorf("TakeSheet by a clock that is behind the sweep's = %d row(s), %v, want ErrSheetUnavailable", len(sheet), err)
	}
	if after := w.job(t, job.ID); after.SheetDownloadedAt != nil {
		t.Errorf("a download was recorded for a sheet with nothing in it: %v", after.SheetDownloadedAt)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user_import.sheet_downloaded' AND entity_id = $1::uuid`, job.ID); n != 0 {
		t.Errorf("%d download(s) audited for a sheet with nothing in it", n)
	}
}

func TestTheSweepCommandNeedsALimit(t *testing.T) {
	w := newWorkerWorld(t)
	if _, err := w.app.Commands.EraseExpiredImportSheets.Handle(context.Background(), command.EraseExpiredImportSheets{}); err == nil {
		t.Error("a sweep with no limit was accepted")
	}
}

func TestAnAddressASheetCouldNotCarryIsRefusedAtPreviewAndAtCreationForEitherDelivery(t *testing.T) {
	for name, delivery := range map[string]domain.ImportDelivery{"sheet": domain.ImportSheet, "email link": domain.ImportEmailLink} {
		t.Run(name, func(t *testing.T) {
			w := newWorkerWorld(t)
			good, plus := w.email("tot"), "+"+w.email("cong")
			file := csvOf("full_name,email", "Nguyễn Văn Tốt,"+good, "Trần Thị Cộng,"+plus)

			preview, err := w.preview(w.req(), file, domain.DefaultStudent, delivery)
			if err != nil {
				t.Fatal(err)
			}
			if preview.NewAccounts != 1 || preview.InvalidRows != 1 || len(preview.Issues) != 1 {
				t.Fatalf("preview = new %d, invalid %d, issues %+v, want 1, 1 and the refused cell", preview.NewAccounts, preview.InvalidRows, preview.Issues)
			}
			wantIssue(t, preview.Issues, 3, domain.FieldEmail, domain.CodeFormulaLike)

			created, err := w.submit(w.req(), uuid.NewString(), file, domain.DefaultStudent, delivery)
			if err != nil {
				t.Fatal(err)
			}
			if created.Job.Total != 1 || created.Job.Invalid != 1 {
				t.Errorf("job = %+v, want the one good row stored and the other counted invalid", created.Job)
			}
			rows := w.storedRows(t, created.Job.ID)
			if len(rows) != 1 || rows[0].Email != good {
				t.Errorf("stored rows = %+v, want only %s", rows, good)
			}
		})
	}
}

func TestTheSheetNamesTheClassOnlyOfTheStudentsWhoWerePlacedInIt(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	name := "Lớp đóng giữa chừng " + w.marker
	class := w.classNamed(t, name, false)
	file, _ := w.students(3*domain.ImportBatchSize, name)
	job := w.submitRows(t, domain.ImportSheet, file)
	run := w.runner()
	run.Queue = &afterFirstSettle{UserImportQueue: w.queue, then: func() {
		if _, err := w.admin.Exec(ctx, `UPDATE app.classes SET archived_at = now() WHERE id = $1::uuid`, class); err != nil {
			t.Error(err)
		}
	}}

	w.runOnce(t, run)

	if done := w.job(t, job.ID); done.Status != domain.ImportDone || done.Created != 3*domain.ImportBatchSize {
		t.Fatalf("job = %+v, want done with every account created", done)
	}
	if n := w.count(t, `SELECT count(*) FROM app.class_members WHERE class_id = $1::uuid`, class); n != domain.ImportBatchSize {
		t.Errorf("%d student(s) placed, want only the first batch of %d", n, domain.ImportBatchSize)
	}
	sheet, err := w.app.Commands.DownloadUserImportSheet.Handle(ctx, command.DownloadUserImportSheet{Request: w.req(), ID: job.ID})
	if err != nil || len(sheet) != 3*domain.ImportBatchSize {
		t.Fatalf("the sheet = %d row(s), %v, want a line for every account", len(sheet), err)
	}
	named, blank := 0, 0
	for i, row := range sheet {
		switch {
		case i < domain.ImportBatchSize && row.Class == name:
			named++
		case i >= domain.ImportBatchSize && row.Class == "":
			blank++
		default:
			t.Errorf("line %d names the class %q", i, row.Class)
		}
	}
	if named != domain.ImportBatchSize || blank != 2*domain.ImportBatchSize {
		t.Errorf("%d line(s) name the class and %d leave it blank, want %d and %d", named, blank, domain.ImportBatchSize, 2*domain.ImportBatchSize)
	}
	if n := w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND class_id IS NOT NULL`, job.ID); n != domain.ImportBatchSize {
		t.Errorf("%d stored row(s) still name the class, want only the %d placed", n, domain.ImportBatchSize)
	}
}
