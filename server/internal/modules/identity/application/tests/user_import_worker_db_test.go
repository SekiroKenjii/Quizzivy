//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	accessapp "quizzivy/internal/modules/access/application"
	accessrepo "quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/application/worker"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/platform/secrets"
	"quizzivy/internal/shared/access"
)

type workerWorld struct {
	*importsWorld
	importer   string
	principals *accessapp.Application
	queue      *repositories.UserImports
	sealer     *secrets.Sealer
	passwords  atomic.Int32
}

func newWorkerWorld(t *testing.T) *workerWorld {
	t.Helper()
	w := &workerWorld{importsWorld: newImportsWorld(t), sealer: sheetSealer(t)}
	w.importer = w.insertUser(t, "người nhập", "admin")
	w.principals = accessapp.New(accessrepo.NewPostgres(w.dbx))
	w.queue = repositories.NewUserImports(w.dbx).WithLinks(w.links)
	return w
}

func (w *workerWorld) cheapPasswords(context.Context) (string, string, error) {
	n := w.passwords.Add(1)
	return fmt.Sprintf("mat-khau-tam-%02d", n), fmt.Sprintf("$test$hash-%d", n), nil
}

func (w *workerWorld) runner() worker.UserImports {
	return worker.UserImports{
		Queue: w.queue, Roles: w.principals.Queries.AssignableRole, Principals: w.principals,
		Policy: switchablePolicy(func() organizationmodel.SignInPolicy { return w.policy }),
		Links:  w.links, Sealer: w.sealer, Passwords: w.cheapPasswords,
	}
}

func (w *workerWorld) req() domain.WriteRequest {
	r := w.everything()
	r.ActorID = w.importer
	return r
}

func (w *workerWorld) dropJobsOnCleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := w.admin.Exec(context.Background(), `DELETE FROM app.user_import_jobs WHERE created_by = $1::uuid`, w.importer); err != nil {
			t.Errorf("cleanup of the import jobs: %v", err)
		}
	})
}

func (w *workerWorld) job(t *testing.T, id string) domain.ImportJob {
	t.Helper()
	job, err := w.queue.Find(context.Background(), w.importer, id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func (w *workerWorld) submitRows(t *testing.T, delivery domain.ImportDelivery, file []byte) domain.ImportJob {
	t.Helper()
	got, err := w.submit(w.req(), uuid.NewString(), file, domain.DefaultStudent, delivery)
	if err != nil {
		t.Fatal(err)
	}
	return got.Job
}

func (w *workerWorld) students(n int, class string) ([]byte, []string) {
	lines := []string{"full_name,email,role,class"}
	emails := make([]string, n)
	for i := range n {
		emails[i] = w.email(fmt.Sprintf("hv%02d", i))
		lines = append(lines, fmt.Sprintf("Học viên %02d,%s,,%s", i, emails[i], class))
	}
	return csvOf(lines...), emails
}

func (w *workerWorld) runOnce(t *testing.T, run worker.UserImports) {
	t.Helper()
	worked, err := run.RunOne(context.Background())
	if err != nil || !worked {
		t.Fatalf("RunOne = %v, %v, want a job run", worked, err)
	}
}

func (w *workerWorld) nothingToRun(t *testing.T, run worker.UserImports) {
	t.Helper()
	if worked, err := run.RunOne(context.Background()); err != nil || worked {
		t.Fatalf("RunOne = %v, %v, want an empty queue", worked, err)
	}
}

func (w *workerWorld) madeByTheImporter(t *testing.T) int {
	t.Helper()
	return w.count(t, `SELECT count(*) FROM app.users WHERE created_by = $1::uuid`, w.importer)
}

func (w *workerWorld) outcomes(t *testing.T, jobID string) map[string]int {
	t.Helper()
	rows, err := w.admin.Query(context.Background(), `
		SELECT outcome, count(*) FROM app.user_import_rows WHERE job_id = $1::uuid GROUP BY outcome`, jobID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var outcome string
		var n int
		if err := rows.Scan(&outcome, &n); err != nil {
			t.Fatal(err)
		}
		out[outcome] = n
	}
	return out
}

func (w *workerWorld) wantFailed(t *testing.T, jobID, code string) {
	t.Helper()
	job := w.job(t, jobID)
	if job.Status != domain.ImportFailed || job.ErrorCode != code || job.CompletedAt == nil {
		t.Errorf("job = %s / %q (completed %v), want failed with %s", job.Status, job.ErrorCode, job.CompletedAt, code)
	}
}

func (w *workerWorld) sealed(t *testing.T, jobID string) int {
	t.Helper()
	return w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND secret_ciphertext IS NOT NULL`, jobID)
}

func TestTheWorkerCreatesTheAccountsOfAnEmailJobInBatchesAndFinishesIt(t *testing.T) {
	w := newWorkerWorld(t)
	class := w.classNamed(t, "Lớp 10A1 "+w.marker, false)
	taken, teacher := w.taken(t, "co-san"), w.email("gv")
	file, students := w.students(57, "Lớp 10A1 "+w.marker)
	file = append(file, csvOf("Giáo viên Mới,"+teacher+",teacher,", "Có sẵn,"+taken+",,")...)
	job := w.submitRows(t, domain.ImportEmailLink, file)
	if job.Total != 59 || job.Skipped != 1 {
		t.Fatalf("job = %+v, want 59 stored and 1 skipped", job)
	}
	before := w.accounts(t)

	w.runOnce(t, w.runner())

	done := w.job(t, job.ID)
	if done.Status != domain.ImportDone || done.Created != 58 || done.Skipped != 1 || done.Processed() != done.Total || done.CompletedAt == nil || done.StartedAt == nil || done.Attempts != 1 {
		t.Errorf("job = %+v, want done with 58 created, 1 skipped, one attempt", done)
	}
	if state, ok := done.Sheet(time.Now()); ok || state.Available {
		t.Errorf("an email job offers a sheet %+v", state)
	}
	if got := w.accounts(t) - before; got != 58 {
		t.Errorf("%d account(s) created, want 58", got)
	}
	if got := w.madeByTheImporter(t); got != 58 {
		t.Errorf("%d account(s) carry the importer as creator, want 58", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.class_members WHERE class_id = $1::uuid AND added_by = $2::uuid AND joined_via = 'admin'`, class, w.importer); got != 57 {
		t.Errorf("%d student(s) enrolled by the importer, want 57", got)
	}
	var role string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT r.builtin_key FROM app.users u JOIN app.roles r ON r.id = u.role_id WHERE u.email = $1`, teacher).Scan(&role); err != nil || role != "teacher" {
		t.Errorf("the teacher row made a %q account (%v), want teacher", role, err)
	}
	if got := w.count(t, `SELECT count(*) FROM app.users WHERE created_by = $1::uuid AND password_hash IS NOT NULL`, w.importer); got != 0 {
		t.Errorf("%d email-link account(s) hold a password", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.email_outbox WHERE to_address = ANY($1::text[])`, append(students, teacher)); got != 58 {
		t.Errorf("%d message(s) queued, want one for each of the 58 accounts", got)
	}
	if got := w.count(t, `
		SELECT count(*) FROM app.audit_log
		 WHERE action = 'user.created' AND actor_user_id = $1::uuid AND diff->>'via' = 'import' AND diff->>'jobId' = $2`, w.importer, job.ID); got != 58 {
		t.Errorf("%d user.created entries name the job, want 58", got)
	}
	if got := w.outcomes(t, job.ID); got["created"] != 58 || got["skipped_exists"] != 1 || got["pending"] != 0 {
		t.Errorf("outcomes = %v, want 58 created and 1 skipped", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND outcome = 'created' AND user_id IS NULL`, job.ID); got != 0 {
		t.Errorf("%d created row(s) name no account", got)
	}
	w.nothingToRun(t, w.runner())
}

func TestTheWorkerHandsASheetJobItsPasswordsOnceAndOnlyToItsImporter(t *testing.T) {
	w := newWorkerWorld(t)
	run := w.runner()
	run.Passwords = nil
	file := csvOf("full_name,email,role,class", "Nguyễn Văn An,"+w.email("an")+",,", "Trần Thị Bình,"+w.email("binh")+",teacher,")
	job := w.submitRows(t, domain.ImportSheet, file)

	w.runOnce(t, run)

	done := w.job(t, job.ID)
	if done.Status != domain.ImportDone || done.Created != 2 || done.SheetExpiresAt == nil || done.CompletedAt == nil {
		t.Fatalf("job = %+v, want done with two accounts and a sheet deadline", done)
	}
	if gap := done.SheetExpiresAt.Sub(*done.CompletedAt); gap < 23*time.Hour+59*time.Minute || gap > 24*time.Hour+time.Minute {
		t.Errorf("the sheet expires %s after completion, want 24 hours", gap)
	}
	if state, ok := done.Sheet(time.Now()); !ok || !state.Available {
		t.Errorf("sheet = %+v, want available", state)
	}
	if got := w.sealed(t, job.ID); got != 2 {
		t.Fatalf("%d sealed password(s) kept, want 2", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.users WHERE created_by = $1::uuid AND password_hash IS NOT NULL AND must_change_password`, w.importer); got != 2 {
		t.Errorf("%d account(s) hold a password they must change, want 2", got)
	}

	stranger := w.req()
	stranger.ActorID = w.actor
	if _, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: stranger, ID: job.ID}); !errors.Is(err, domain.ErrUserImportNotFound) {
		t.Errorf("another user's download = %v, want ErrUserImportNotFound", err)
	}
	if got := w.sealed(t, job.ID); got != 2 {
		t.Fatalf("a stranger's request left %d sealed password(s), want 2", got)
	}

	sheet, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: w.req(), ID: job.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet) != 2 || sheet[0].FullName != "Nguyễn Văn An" || sheet[0].Role == "" || sheet[1].FullName != "Trần Thị Bình" {
		t.Fatalf("sheet = %+v, want the two accounts in file order", sheet)
	}
	for _, row := range sheet {
		var hash string
		if err := w.admin.QueryRow(context.Background(), `SELECT password_hash FROM app.users WHERE email = $1`, row.Email).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if ok, err := domain.Passwords.Verify(context.Background(), row.Password, hash); err != nil || !ok {
			t.Errorf("the sheet's password for %s does not open the account (%v)", row.Email, err)
		}
	}
	if got := w.sealed(t, job.ID); got != 0 {
		t.Errorf("%d sealed password(s) survive the download", got)
	}
	taken := w.job(t, job.ID)
	if taken.SheetDownloadedAt == nil {
		t.Error("the download was not recorded")
	}
	if state, _ := taken.Sheet(time.Now()); state.Available || state.DownloadedAt == nil {
		t.Errorf("sheet after the download = %+v, want unavailable and dated", state)
	}
	if _, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: w.req(), ID: job.ID}); !errors.Is(err, domain.ErrSheetUnavailable) {
		t.Errorf("second download = %v, want ErrSheetUnavailable", err)
	}

	var diff []byte
	if err := w.admin.QueryRow(context.Background(), `
		SELECT diff FROM app.audit_log
		 WHERE action = 'user_import.sheet_downloaded' AND entity_id = $1::uuid AND actor_user_id = $2::uuid`, job.ID, w.importer).Scan(&diff); err != nil {
		t.Fatalf("the download was not audited: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(diff, &fields); err != nil || fmt.Sprint(fields) != "map[rows:2]" {
		t.Errorf("audit diff = %s (%v), want only the row count", diff, err)
	}
	for _, row := range sheet {
		if strings.Contains(string(diff), row.Password) {
			t.Error("the audit entry holds a password")
		}
	}
}

func TestASheetThatIsNotReadyOrHasExpiredOrBelongsToAnEmailJobIsNotHandedOut(t *testing.T) {
	w := newWorkerWorld(t)
	ask := func(id string) error {
		_, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: w.req(), ID: id})
		return err
	}
	file, _ := w.students(2, "")

	queued := w.submitRows(t, domain.ImportSheet, file)
	if err := ask(queued.ID); !errors.Is(err, domain.ErrSheetUnavailable) {
		t.Errorf("a queued job's sheet = %v, want ErrSheetUnavailable", err)
	}
	w.runOnce(t, w.runner())
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_jobs SET sheet_expires_at = now() - interval '1 minute' WHERE id = $1::uuid`, queued.ID); err != nil {
		t.Fatal(err)
	}
	if err := ask(queued.ID); !errors.Is(err, domain.ErrSheetUnavailable) {
		t.Errorf("an expired sheet = %v, want ErrSheetUnavailable", err)
	}
	if got := w.sealed(t, queued.ID); got != 2 {
		t.Errorf("an expired sheet's refusal left %d sealed password(s), want 2 for the housekeeping to erase", got)
	}

	file, _ = w.students(1, "")
	byEmail := w.submitRows(t, domain.ImportEmailLink, file)
	w.runOnce(t, w.runner())
	if err := ask(byEmail.ID); !errors.Is(err, domain.ErrSheetUnavailable) {
		t.Errorf("an email job's sheet = %v, want ErrSheetUnavailable", err)
	}
	if err := ask(uuid.NewString()); !errors.Is(err, domain.ErrUserImportNotFound) {
		t.Errorf("an unknown job's sheet = %v, want ErrUserImportNotFound", err)
	}
}

func TestASheetKeepsThePasswordWhenThePolicyDoesNotForceAChange(t *testing.T) {
	w := newWorkerWorld(t)
	w.policy.ForcePasswordChange = false
	file, _ := w.students(2, "")
	w.submitRows(t, domain.ImportSheet, file)

	w.runOnce(t, w.runner())

	if got := w.count(t, `SELECT count(*) FROM app.users WHERE created_by = $1::uuid AND password_hash IS NOT NULL AND NOT must_change_password`, w.importer); got != 2 {
		t.Errorf("%d account(s) keep their password, want 2", got)
	}
}

type dyingQueue struct {
	domain.UserImportQueue
	settles, allowed int
}

func (d *dyingQueue) Settle(ctx context.Context, batch domain.ImportBatch, issue domain.LinkIssue) (domain.ImportProgress, error) {
	if d.settles >= d.allowed {
		return domain.ImportProgress{}, errors.New("the worker died")
	}
	d.settles++
	return d.UserImportQueue.Settle(ctx, batch, issue)
}

func (d *dyingQueue) Retry(context.Context, string, int) error   { return nil }
func (d *dyingQueue) Release(context.Context, string, int) error { return nil }
func (d *dyingQueue) Fail(context.Context, string, int, string) error {
	return nil
}

func (w *workerWorld) crashAfter(t *testing.T, batches int) worker.UserImports {
	t.Helper()
	run := w.runner()
	run.Queue = &dyingQueue{UserImportQueue: w.queue, allowed: batches}
	return run
}

func (w *workerWorld) lapse(t *testing.T, jobID string) {
	t.Helper()
	if _, err := w.admin.Exec(context.Background(), `
		UPDATE app.user_import_jobs SET started_at = now() - interval '31 minutes' WHERE id = $1::uuid`, jobID); err != nil {
		t.Fatal(err)
	}
}

func TestAJobLeftRunningByADeadWorkerResumesAfterItsLeaseWithoutDuplicates(t *testing.T) {
	w := newWorkerWorld(t)
	file, emails := w.students(60, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)

	if worked, err := w.crashAfter(t, 1).RunOne(context.Background()); worked || err == nil {
		t.Fatalf("the dying worker returned %v, %v, want its failure", worked, err)
	}
	half := w.job(t, job.ID)
	if half.Status != domain.ImportRunning || half.Created != domain.ImportBatchSize || half.Attempts != 1 {
		t.Fatalf("job = %+v, want running with the first batch of %d made on attempt 1", half, domain.ImportBatchSize)
	}
	w.nothingToRun(t, w.runner())
	if again := w.job(t, job.ID); again.Created != domain.ImportBatchSize || again.Attempts != 1 {
		t.Fatalf("a job within its lease was taken again: %+v", again)
	}

	w.lapse(t, job.ID)
	w.runOnce(t, w.runner())

	done := w.job(t, job.ID)
	if done.Status != domain.ImportDone || done.Created != 60 || done.Attempts != 2 {
		t.Errorf("job = %+v, want done with 60 created on the second attempt", done)
	}
	if got := w.count(t, `SELECT count(*) FROM app.users WHERE email = ANY($1::text[])`, emails); got != 60 {
		t.Errorf("%d account(s) exist for the 60 addresses", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user.created' AND diff->>'jobId' = $1`, job.ID); got != 60 {
		t.Errorf("%d user.created entries, want one for each account", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.email_outbox WHERE to_address = ANY($1::text[])`, emails); got != 60 {
		t.Errorf("%d message(s) queued, want exactly one for each account", got)
	}
}

func TestAJobClaimedThreeTimesIsFailedAndItsSealedPasswordsAreErased(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(30, "")
	job := w.submitRows(t, domain.ImportSheet, file)
	if worked, err := w.crashAfter(t, 1).RunOne(context.Background()); worked || err == nil {
		t.Fatalf("the dying worker returned %v, %v", worked, err)
	}
	if w.sealed(t, job.ID) != domain.ImportBatchSize {
		t.Fatalf("%d sealed password(s) after the first batch, want %d", w.sealed(t, job.ID), domain.ImportBatchSize)
	}
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_jobs SET attempts = 3 WHERE id = $1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
	w.lapse(t, job.ID)

	w.nothingToRun(t, w.runner())

	w.wantFailed(t, job.ID, domain.ImportFailedInterrupted)
	if got := w.sealed(t, job.ID); got != 0 {
		t.Errorf("%d sealed password(s) of a failed job remain", got)
	}
	if got := w.job(t, job.ID).Created; got != domain.ImportBatchSize {
		t.Errorf("the accounts already made were counted %d, want %d", got, domain.ImportBatchSize)
	}
}

func TestAJobWhoseImporterIsGoneIsFailedAndCreatesNothing(t *testing.T) {
	cases := map[string]func(t *testing.T, w *workerWorld){
		"disabled": func(t *testing.T, w *workerWorld) {
			if _, err := w.admin.Exec(context.Background(), `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, w.importer); err != nil {
				t.Fatal(err)
			}
		},
		"demoted to teacher": func(t *testing.T, w *workerWorld) {
			if _, err := w.admin.Exec(context.Background(), `
				UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'teacher') WHERE id = $1::uuid`, w.importer); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorkerWorld(t)
			file, _ := w.students(3, "")
			job := w.submitRows(t, domain.ImportEmailLink, file)
			change(t, w)

			w.runOnce(t, w.runner())

			w.wantFailed(t, job.ID, domain.ImportFailedNotAllowed)
			if got := w.madeByTheImporter(t); got != 0 {
				t.Errorf("%d account(s) created for an importer who may no longer", got)
			}
			if got := w.outcomes(t, job.ID); got["pending"] != 3 {
				t.Errorf("outcomes = %v, want the three rows left pending", got)
			}
		})
	}
}

type afterFirstSettle struct {
	domain.UserImportQueue
	then func()
}

func (a *afterFirstSettle) Settle(ctx context.Context, batch domain.ImportBatch, issue domain.LinkIssue) (domain.ImportProgress, error) {
	progress, err := a.UserImportQueue.Settle(ctx, batch, issue)
	if err == nil && a.then != nil {
		a.then()
		a.then = nil
	}
	return progress, err
}

func TestAnImporterWhoseRightsChangeBetweenBatchesIsRefusedAtTheNextOneWhateverThisProcessRemembers(t *testing.T) {
	cases := map[string]string{
		"disabled":           `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`,
		"demoted to teacher": `UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'teacher') WHERE id = $1::uuid`,
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorkerWorld(t)
			file, _ := w.students(3*domain.ImportBatchSize, "")
			job := w.submitRows(t, domain.ImportEmailLink, file)
			run := w.runner()
			run.Queue = &afterFirstSettle{UserImportQueue: w.queue, then: func() {
				if _, err := w.admin.Exec(context.Background(), change, w.importer); err != nil {
					t.Error(err)
				}
			}}

			w.runOnce(t, run)

			w.wantFailed(t, job.ID, domain.ImportFailedNotAllowed)
			if got := w.madeByTheImporter(t); got != domain.ImportBatchSize {
				t.Errorf("%d account(s) created, want only the first batch of %d", got, domain.ImportBatchSize)
			}
		})
	}
}

func TestAJobFailsWhenTheImporterNoLongerHasTheScopeToPlaceAccountsInClasses(t *testing.T) {
	w := newWorkerWorld(t)
	w.classNamed(t, "Lớp riêng "+w.marker, false)
	file, _ := w.students(2, "Lớp riêng "+w.marker)
	job := w.submitRows(t, domain.ImportEmailLink, file)
	role := w.customRole(t, access.PeopleUsersManage)
	w.dropJobsOnCleanup(t)
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.users SET role_id = $2::uuid WHERE id = $1::uuid`, w.importer, role); err != nil {
		t.Fatal(err)
	}

	w.runOnce(t, w.runner())

	w.wantFailed(t, job.ID, domain.ImportFailedNotAllowed)
	if n := w.madeByTheImporter(t); n != 0 {
		t.Errorf("%d account(s) created in classes for an importer without scope.all", n)
	}
}

func TestAnAddressTakenAfterTheJobWasStoredIsSkippedAndTheRestIsCreated(t *testing.T) {
	w := newWorkerWorld(t)
	file, emails := w.students(4, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)
	if _, err := w.admin.Exec(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Đăng ký trước', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, strings.ToUpper(emails[2])); err != nil {
		t.Fatal(err)
	}

	w.runOnce(t, w.runner())

	done := w.job(t, job.ID)
	if done.Status != domain.ImportDone || done.Created != 3 || done.Skipped != 1 {
		t.Errorf("job = %+v, want done with 3 created and the racing address skipped", done)
	}
	var name string
	if err := w.admin.QueryRow(context.Background(), `SELECT full_name FROM app.users WHERE lower(email) = lower($1)`, emails[2]).Scan(&name); err != nil || name != "Đăng ký trước" {
		t.Errorf("the account that won the address is %q (%v), want it left as it was", name, err)
	}
	if got := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user.created' AND diff->>'jobId' = $1`, job.ID); got != 3 {
		t.Errorf("%d user.created entries, want 3", got)
	}
}

func TestAClassArchivedAfterTheJobWasStoredLeavesTheAccountsCreatedWithoutIt(t *testing.T) {
	w := newWorkerWorld(t)
	class := w.classNamed(t, "Lớp sắp đóng "+w.marker, false)
	file, emails := w.students(3, "Lớp sắp đóng "+w.marker)
	job := w.submitRows(t, domain.ImportEmailLink, file)
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.classes SET archived_at = now() WHERE id = $1::uuid`, class); err != nil {
		t.Fatal(err)
	}

	w.runOnce(t, w.runner())

	if done := w.job(t, job.ID); done.Status != domain.ImportDone || done.Created != 3 {
		t.Errorf("job = %+v, want done with 3 created", done)
	}
	if got := w.count(t, `SELECT count(*) FROM app.users WHERE email = ANY($1::text[])`, emails); got != 3 {
		t.Errorf("%d account(s) created, want 3", got)
	}
	if got := w.count(t, `SELECT count(*) FROM app.class_members WHERE class_id = $1::uuid`, class); got != 0 {
		t.Errorf("%d member(s) placed in an archived class", got)
	}
}

func TestARoleThatChangesBetweenTheReadAndTheBatchIsReadAgain(t *testing.T) {
	w := newWorkerWorld(t)
	role := w.customRole(t, access.TeachingGrading)
	w.dropJobsOnCleanup(t)
	var name string
	if err := w.admin.QueryRow(context.Background(), `SELECT name FROM app.roles WHERE id = $1::uuid`, role).Scan(&name); err != nil {
		t.Fatal(err)
	}
	email := w.email("vai-tro")
	job := w.submitRows(t, domain.ImportEmailLink, csvOf("full_name,email,role", "Người Vai Riêng,"+email+","+name))
	hooked := &changingRoles{inner: w.principals.Queries.AssignableRole}
	hooked.between = func() {
		hooked.between = nil
		if _, err := w.admin.Exec(context.Background(), `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, role, string(access.ContentShare)); err != nil {
			t.Error(err)
		}
	}
	run := w.runner()
	run.Roles = hooked

	w.runOnce(t, run)

	if done := w.job(t, job.ID); done.Status != domain.ImportDone || done.Created != 1 {
		t.Errorf("job = %+v, want done with the account made", done)
	}
	if hooked.reads != 2 {
		t.Errorf("the role was read %d time(s), want twice: before the change and after it", hooked.reads)
	}
}

func TestADeliveryThatBecomesUnavailableFailsTheJobBeforeAnyAccount(t *testing.T) {
	cases := map[string]struct {
		delivery domain.ImportDelivery
		change   func(w *workerWorld, run *worker.UserImports)
	}{
		"password sign-in turned off": {domain.ImportSheet, func(w *workerWorld, _ *worker.UserImports) { w.policy.Password = false }},
		"no key to seal with":         {domain.ImportSheet, func(_ *workerWorld, run *worker.UserImports) { run.Sealer = nil }},
		"no store for links":          {domain.ImportEmailLink, func(_ *workerWorld, run *worker.UserImports) { run.Links = nil }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorkerWorld(t)
			file, _ := w.students(3, "")
			job := w.submitRows(t, c.delivery, file)
			run := w.runner()
			c.change(w, &run)

			w.runOnce(t, run)

			w.wantFailed(t, job.ID, domain.ImportFailedDelivery)
			if got := w.madeByTheImporter(t); got != 0 {
				t.Errorf("%d account(s) created without a way to hand them a password", got)
			}
		})
	}
}

func TestARunThatOutlastsItsLimitGivesTheJobBackWithTheAttemptSpent(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(3, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)
	slow := w.runner()
	slow.Timeout = time.Nanosecond

	worked, err := slow.RunOne(context.Background())
	if worked || err == nil {
		t.Fatalf("RunOne = %v, %v, want the failure reported for another poll", worked, err)
	}
	back := w.job(t, job.ID)
	if back.Status != domain.ImportQueued || back.Attempts != 1 {
		t.Fatalf("job = %+v, want queued with the attempt spent", back)
	}

	w.runOnce(t, w.runner())
	if done := w.job(t, job.ID); done.Status != domain.ImportDone || done.Attempts != 2 {
		t.Errorf("job = %+v, want done on the second attempt", done)
	}
}

type cancellingQueue struct {
	domain.UserImportQueue
	cancel context.CancelFunc
}

func (c cancellingQueue) Todo(context.Context, string, int) ([]domain.ImportTodo, error) {
	c.cancel()
	return nil, context.Canceled
}

func TestAShutdownGivesTheJobBackAndTheAttemptWithIt(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(3, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)
	ctx, cancel := context.WithCancel(context.Background())
	run := w.runner()
	run.Queue = cancellingQueue{UserImportQueue: w.queue, cancel: cancel}

	if worked, err := run.RunOne(ctx); err != nil || !worked {
		t.Fatalf("RunOne = %v, %v, want the job released quietly", worked, err)
	}

	back := w.job(t, job.ID)
	if back.Status != domain.ImportQueued || back.Attempts != 0 || back.StartedAt != nil {
		t.Errorf("job = %+v, want queued as if never claimed", back)
	}
}

func TestOnlyTheHolderOfTheClaimMayWriteToAJob(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(2, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)
	ctx := context.Background()
	claimed, err := w.queue.Claim(ctx)
	if err != nil || claimed.ID != job.ID || claimed.Attempts != 1 {
		t.Fatalf("Claim = %+v, %v, want the job on attempt 1", claimed, err)
	}
	todo, err := w.queue.Todo(ctx, job.ID, 10)
	if err != nil || len(todo) != 2 {
		t.Fatalf("Todo = %+v, %v, want the two rows", todo, err)
	}

	batch := domain.ImportBatch{JobID: job.ID, Attempt: 2, By: w.importer, Delivery: domain.ImportEmailLink, Now: time.Now()}
	if _, err := w.queue.Settle(ctx, batch, nil); !errors.Is(err, domain.ErrImportLeaseLost) {
		t.Errorf("Settle under another attempt = %v, want ErrImportLeaseLost", err)
	}
	for name, err := range map[string]error{
		"Fail":    w.queue.Fail(ctx, job.ID, 2, "X"),
		"Retry":   w.queue.Retry(ctx, job.ID, 2),
		"Release": w.queue.Release(ctx, job.ID, 2),
	} {
		if !errors.Is(err, domain.ErrImportLeaseLost) {
			t.Errorf("%s under another attempt = %v, want ErrImportLeaseLost", name, err)
		}
	}
	if still := w.job(t, job.ID); still.Status != domain.ImportRunning || still.Attempts != 1 || still.ErrorCode != "" {
		t.Errorf("job = %+v, want it untouched", still)
	}
}

func TestOnlyOneOfTwoWorkersClaimsAJob(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(2, "")
	w.submitRows(t, domain.ImportEmailLink, file)
	var wins, empty atomic.Int32
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			_, err := w.queue.Claim(context.Background())
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, domain.ErrNoImportWork):
				empty.Add(1)
			default:
				t.Error(err)
			}
		})
	}
	group.Wait()
	if wins.Load() != 1 || empty.Load() != 1 {
		t.Errorf("%d claim(s) won and %d found nothing, want one and one", wins.Load(), empty.Load())
	}
}

func TestTheQueueSaysWhenAJobIsDue(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(2, "")
	job := w.submitRows(t, domain.ImportEmailLink, file)
	due, pending, err := w.queue.NextDue(context.Background())
	if err != nil || !pending || time.Since(due) > time.Minute {
		t.Fatalf("NextDue = %v, %v, %v, want a queued job due now", due, pending, err)
	}
	if _, err := w.queue.Claim(context.Background()); err != nil {
		t.Fatal(err)
	}
	due, pending, err = w.queue.NextDue(context.Background())
	if err != nil || !pending || time.Until(due) < 29*time.Minute || time.Until(due) > 31*time.Minute {
		t.Fatalf("NextDue = %v, %v, %v, want the lease's end for a running job", due, pending, err)
	}
	if _, err := w.admin.Exec(context.Background(), `DELETE FROM app.user_import_jobs WHERE id = $1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
}

func TestReadingAJobShowsTheSheetOnlyToItsImporter(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(1, "")
	job := w.submitRows(t, domain.ImportSheet, file)
	w.runOnce(t, w.runner())
	got, err := w.app.Queries.GetUserImport.Handle(context.Background(), query.GetUserImport{Request: w.req(), ID: job.ID})
	if err != nil || got.Sheet == nil || !got.Sheet.Available || got.Sheet.ExpiresAt == nil {
		t.Fatalf("GetUserImport = %+v, %v, want the sheet available with its deadline", got, err)
	}
}

func TestAJobThatFailsForItsImporterErasesThePasswordsItAlreadySealed(t *testing.T) {
	w := newWorkerWorld(t)
	file, _ := w.students(30, "")
	job := w.submitRows(t, domain.ImportSheet, file)
	if worked, err := w.crashAfter(t, 1).RunOne(context.Background()); worked || err == nil {
		t.Fatalf("the dying worker returned %v, %v", worked, err)
	}
	if w.sealed(t, job.ID) != domain.ImportBatchSize {
		t.Fatalf("%d sealed password(s) after the first batch, want %d", w.sealed(t, job.ID), domain.ImportBatchSize)
	}
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, w.importer); err != nil {
		t.Fatal(err)
	}
	w.lapse(t, job.ID)

	w.runOnce(t, w.runner())

	w.wantFailed(t, job.ID, domain.ImportFailedNotAllowed)
	if got := w.sealed(t, job.ID); got != 0 {
		t.Errorf("%d sealed password(s) of a failed job remain", got)
	}
	if got := w.job(t, job.ID); got.Created != domain.ImportBatchSize || got.SheetExpiresAt != nil {
		t.Errorf("job = %+v, want the %d accounts kept and no sheet offered", got, domain.ImportBatchSize)
	}
}

func TestASheetJobThatCreatesNoAccountOffersNoSheet(t *testing.T) {
	w := newWorkerWorld(t)
	file, emails := w.students(1, "")
	job := w.submitRows(t, domain.ImportSheet, file)
	if _, err := w.admin.Exec(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Đăng ký trước', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, emails[0]); err != nil {
		t.Fatal(err)
	}

	w.runOnce(t, w.runner())

	done := w.job(t, job.ID)
	if done.Status != domain.ImportDone || done.Created != 0 || done.Skipped != 1 || done.SheetExpiresAt != nil {
		t.Errorf("job = %+v, want done with nothing created and no sheet deadline", done)
	}
	if state, ok := done.Sheet(time.Now()); !ok || state.Available {
		t.Errorf("sheet = %+v (%v), want a sheet job with nothing to hand out", state, ok)
	}
	if _, err := w.app.Commands.DownloadUserImportSheet.Handle(context.Background(), command.DownloadUserImportSheet{Request: w.req(), ID: job.ID}); !errors.Is(err, domain.ErrSheetUnavailable) {
		t.Errorf("download = %v, want ErrSheetUnavailable", err)
	}
}
