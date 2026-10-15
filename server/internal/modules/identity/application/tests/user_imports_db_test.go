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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"quizzivy/internal/core/adapters"
	accessapp "quizzivy/internal/modules/access/application"
	accessdomain "quizzivy/internal/modules/access/domain"
	accessrepo "quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/secrets"
	"quizzivy/internal/platform/tabular"
)

type countedWake struct{ n atomic.Int32 }

func (c *countedWake) Wake() { c.n.Add(1) }

type fixedBudget struct{ left model.EmailAllowance }

func (b fixedBudget) Remaining(context.Context, time.Time) (model.EmailAllowance, error) {
	return b.left, nil
}

type importsWorld struct {
	*usersWorld
	wake *countedWake
}

func newImportsWorld(t *testing.T) *importsWorld {
	t.Helper()
	w := &importsWorld{usersWorld: newUsersWorld(t, true), wake: &countedWake{}}
	w.attach(t, true)
	return w
}

func (w *importsWorld) attach(t *testing.T, sheets bool) {
	t.Helper()
	w.attachTables(t, adapters.UserImportTables{}, sheets)
}

func (w *importsWorld) attachTables(t *testing.T, tables ports.TableReader, sheets bool) {
	t.Helper()
	w.attachOn(t, linkAppPool(t), tables, sheets)
}

func (w *importsWorld) attachOn(t *testing.T, conn db.Conn, tables ports.TableReader, sheets bool) {
	t.Helper()
	dbx := db.NewContext(conn)
	var sealer ports.Sealer
	if sheets {
		sealer = sheetSealer(t)
	}
	w.app.SetUserImports(repositories.NewUserImports(dbx), accessapp.New(accessrepo.NewPostgres(dbx)).Queries.RoleChoices,
		tables, fixedBudget{left: model.EmailAllowance{Remaining: 7, Limit: 100}}, w.wake, sealer)
}

func sheetSealer(t *testing.T) *secrets.Sealer {
	t.Helper()
	sealer, err := secrets.New([]byte("0123456789abcdef0123456789abcdef"), secrets.PurposeUserImportSheet)
	if err != nil {
		t.Fatal(err)
	}
	return sealer
}

type commitRefusingPool struct{ *pgxpool.Pool }

func (p commitRefusingPool) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := p.Pool.Begin(ctx)
	return refusedCommit{tx}, err
}

type refusedCommit struct{ pgx.Tx }

func (r refusedCommit) Commit(ctx context.Context) error {
	if err := r.Rollback(ctx); err != nil {
		return err
	}
	return errors.New("commit refused")
}

type gatedTables struct {
	entered chan struct{}
	release chan struct{}
}

func (g gatedTables) Read(file []byte, format domain.ImportFormat) (domain.ImportSource, error) {
	g.entered <- struct{}{}
	<-g.release
	return adapters.UserImportTables{}.Read(file, format)
}

func csvOf(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

func (w *importsWorld) preview(req domain.WriteRequest, file []byte, role domain.ImportDefaultRole, delivery domain.ImportDelivery) (command.UserImportPreview, error) {
	return w.app.Commands.PreviewUserImport.Handle(context.Background(), command.PreviewUserImport{
		Request: req, File: file, DefaultRole: role, Delivery: delivery,
	})
}

func (w *importsWorld) submit(req domain.WriteRequest, requestID string, file []byte, role domain.ImportDefaultRole, delivery domain.ImportDelivery) (command.CreateUserImportResult, error) {
	return w.app.Commands.CreateUserImport.Handle(context.Background(), command.CreateUserImport{
		Request: req, RequestID: requestID, Filename: "danh-sach.csv", File: file, DefaultRole: role, Delivery: delivery,
	})
}

func (w *importsWorld) taken(t *testing.T, name string) string {
	t.Helper()
	email := w.email(name)
	if _, err := w.admin.Exec(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Đã có tài khoản', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, email); err != nil {
		t.Fatal(err)
	}
	return email
}

func (w *importsWorld) classNamed(t *testing.T, name string, archived bool) string {
	t.Helper()
	var id string
	if err := w.admin.QueryRow(context.Background(), `
		INSERT INTO app.classes (name, teacher_id, archived_at)
		VALUES ($1, $2::uuid, CASE WHEN $3 THEN now() END) RETURNING id::text`, name, w.actor, archived).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type storedImportRow struct {
	Ordinal  int
	FullName string
	Email    string
	RoleID   string
	ClassID  *string
	Outcome  string
}

func (w *importsWorld) storedRows(t *testing.T, jobID string) []storedImportRow {
	t.Helper()
	rows, err := w.admin.Query(context.Background(), `
		SELECT ordinal, full_name, email, role_id::text, class_id::text, outcome
		  FROM app.user_import_rows WHERE job_id = $1::uuid ORDER BY ordinal`, jobID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []storedImportRow
	for rows.Next() {
		var r storedImportRow
		if err := rows.Scan(&r.Ordinal, &r.FullName, &r.Email, &r.RoleID, &r.ClassID, &r.Outcome); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (w *importsWorld) accounts(t *testing.T) int {
	t.Helper()
	return w.count(t, `SELECT count(*) FROM app.users WHERE email LIKE '%@' || $1`, w.marker)
}

func (w *importsWorld) jobsOf(t *testing.T, userID string) int {
	t.Helper()
	return w.count(t, `SELECT count(*) FROM app.user_import_jobs WHERE created_by = $1::uuid`, userID)
}

func issueAt(issues []domain.ImportIssue, line int, field domain.ImportField) (domain.ImportCode, bool) {
	for _, issue := range issues {
		if issue.Line == line && issue.Field == field {
			return issue.Code, true
		}
	}
	return "", false
}

func wantIssue(t *testing.T, issues []domain.ImportIssue, line int, field domain.ImportField, code domain.ImportCode) {
	t.Helper()
	if got, ok := issueAt(issues, line, field); !ok || got != code {
		t.Errorf("line %d, %s: got %q (found %v), want %s in %+v", line, field, got, ok, code, issues)
	}
}

func TestAPreviewCountsNewExistingAndRefusedRowsAndWritesNothing(t *testing.T) {
	w := newImportsWorld(t)
	taken, fresh, teacher := w.taken(t, "co-san"), w.email("hs-moi"), w.email("gv-moi")
	before := w.accounts(t)
	file := csvOf(
		"full_name,email,role,class",
		"Nguyễn Văn An,"+fresh+",,",
		"Trần Thị Bình,"+teacher+",teacher,",
		"Lê Văn Cường,"+strings.ToUpper(taken)+",,",
		"Phạm Thị Dung,không-phải-email,,",
		"Hoàng Văn Em,"+strings.ToUpper(fresh)+",,",
	)

	got, err := w.preview(w.everything(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows != 5 || got.NewAccounts != 2 || got.ExistingCount != 1 || got.InvalidRows != 2 {
		t.Errorf("preview = rows %d, new %d, existing %d, invalid %d, want 5, 2, 1, 2", got.Rows, got.NewAccounts, got.ExistingCount, got.InvalidRows)
	}
	if len(got.ExistingSample) != 1 || got.ExistingSample[0] != strings.ToUpper(taken) {
		t.Errorf("existing sample = %v, want the address as the file spelled it", got.ExistingSample)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("issues = %+v, want the two refused cells", got.Issues)
	}
	wantIssue(t, got.Issues, 5, domain.FieldEmail, domain.CodeInvalid)
	wantIssue(t, got.Issues, 6, domain.FieldEmail, domain.CodeDuplicateInFile)
	if got.Budget == nil || got.Budget.Remaining != 7 || got.Budget.Limit != 100 {
		t.Errorf("budget = %+v, want the 7 of 100 the email delivery has left", got.Budget)
	}

	if after := w.accounts(t); after != before {
		t.Errorf("a preview left %d account(s), want %d", after, before)
	}
	if n := w.jobsOf(t, w.actor); n != 0 {
		t.Errorf("a preview stored %d job(s)", n)
	}
	if n := w.wake.n.Load(); n != 0 {
		t.Errorf("a preview woke the worker %d time(s)", n)
	}
}

func TestAPreviewForASheetCountsNoEmail(t *testing.T) {
	w := newImportsWorld(t)
	got, err := w.preview(w.everything(), csvOf("full_name,email", "Võ Thị Giang,"+w.email("g")), domain.DefaultStudent, domain.ImportSheet)
	if err != nil {
		t.Fatal(err)
	}
	if got.Budget != nil {
		t.Errorf("a sheet preview carries a mail budget %+v", got.Budget)
	}
}

func TestACreatedImportStoresTheAcceptedRowsInOrderAndOnlyThose(t *testing.T) {
	w := newImportsWorld(t)
	taken, fresh, teacher := w.taken(t, "co-san"), w.email("hs-moi"), w.email("gv-moi")
	before := w.accounts(t)
	file := csvOf(
		"full_name,email,role",
		"  Nguyễn   Văn An ,"+fresh+",",
		"Trần Thị Bình,"+teacher+",GIÁO  viên",
		"Lê Văn Cường,"+strings.ToUpper(taken)+",",
		"Phạm Thị Dung,không-phải-email,",
	)
	requestID := uuid.NewString()

	got, err := w.app.Commands.CreateUserImport.Handle(context.Background(), command.CreateUserImport{
		Request: w.everything(), RequestID: requestID, Filename: `C:\fakepath\ds lớp 10.csv`, File: file,
		DefaultRole: domain.DefaultStudent, Delivery: domain.ImportEmailLink,
	})
	if err != nil {
		t.Fatal(err)
	}
	job := got.Job
	if got.Replayed || got.Sheet != nil {
		t.Errorf("replayed = %v, sheet = %+v, want a fresh email job with no sheet", got.Replayed, got.Sheet)
	}
	if job.Status != domain.ImportQueued || job.Total != 3 || job.Created != 0 || job.Skipped != 1 || job.Invalid != 1 || job.Processed() != 1 {
		t.Errorf("job = %+v, want queued, 3 stored, 1 skipped, 1 refused", job)
	}
	if job.Filename != "ds lớp 10.csv" || job.Format != domain.ImportCSV || job.Delivery != domain.ImportEmailLink || job.CreatedBy != w.actor || job.RequestID != requestID {
		t.Errorf("job = %+v, want the name without its folder, csv, email_link, the actor and the request id", job)
	}

	student, teacherRole := w.role(t, "student"), w.role(t, "teacher")
	want := []storedImportRow{
		{0, "Nguyễn Văn An", fresh, student, nil, "pending"},
		{1, "Trần Thị Bình", teacher, teacherRole, nil, "pending"},
		{2, "Lê Văn Cường", strings.ToUpper(taken), student, nil, "skipped_exists"},
	}
	rows := w.storedRows(t, job.ID)
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Errorf("stored rows = %+v, want %+v", rows, want)
	}
	if after := w.accounts(t); after != before {
		t.Errorf("creating the job made %d account(s), the worker makes them", after-before)
	}
	if n := w.wake.n.Load(); n != 1 {
		t.Errorf("worker woken %d time(s), want once", n)
	}
	if w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND email = 'không-phải-email'`, job.ID) != 0 {
		t.Error("a refused row was stored")
	}
}

func TestACreatedImportIsAuditedWithCountsAndNeverTheFile(t *testing.T) {
	w := newImportsWorld(t)
	taken, fresh := w.taken(t, "co-san"), w.email("hs-moi")
	file := csvOf("full_name,email", "Nguyễn Văn An,"+fresh, "Lê Văn Cường,"+taken, "Phạm Thị Dung,sai")

	got, err := w.submit(w.everything(), uuid.NewString(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	var action, entity string
	var diff []byte
	var ip, agent *string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT action, entity, diff, host(ip), user_agent FROM app.audit_log
		 WHERE entity_id = $1::uuid AND actor_user_id = $2::uuid`, got.Job.ID, w.actor).Scan(&action, &entity, &diff, &ip, &agent); err != nil {
		t.Fatal(err)
	}
	if action != "user_import.requested" || entity != "user_import" || ip == nil || *ip != "203.0.113.5" || agent == nil || *agent != "users-test" {
		t.Errorf("audit = %s on %s from %v / %v, want user_import.requested on user_import with the request's address and agent", action, entity, ip, agent)
	}
	var fields map[string]any
	if err := json.Unmarshal(diff, &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"format": "csv", "delivery": "email_link", "defaultRoleId": w.role(t, "student"), "rows": float64(2), "existing": float64(1), "invalid": float64(1)}
	if fmt.Sprint(fields) != fmt.Sprint(want) {
		t.Errorf("diff = %v, want %v", fields, want)
	}
	for _, leaked := range []string{fresh, taken, "Nguyễn", "danh-sach"} {
		if strings.Contains(string(diff), leaked) {
			t.Errorf("the audit diff carries %q from the file", leaked)
		}
	}
}

func TestARepeatedRequestReturnsTheFirstJobAndWritesNothingMore(t *testing.T) {
	w := newImportsWorld(t)
	fresh := w.email("hs-moi")
	requestID := uuid.NewString()
	first, err := w.submit(w.everything(), requestID, csvOf("full_name,email", "Nguyễn Văn An,"+fresh), domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}

	again, err := w.submit(w.everything(), requestID, nil, domain.DefaultTeacher, domain.ImportSheet)
	if err != nil {
		t.Fatalf("the repeated request was refused: %v", err)
	}
	if !again.Replayed || again.Job.ID != first.Job.ID || again.Job.Delivery != domain.ImportEmailLink {
		t.Errorf("repeat = %+v, want the first job back, replayed", again)
	}
	if n := w.jobsOf(t, w.actor); n != 1 {
		t.Errorf("%d jobs stored, want 1", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE entity_id = $1::uuid`, first.Job.ID); n != 1 {
		t.Errorf("%d audit rows for the job, want 1", n)
	}
	if n := w.wake.n.Load(); n != 1 {
		t.Errorf("worker woken %d time(s), want once", n)
	}
}

func TestAnAuditEntryThatCannotBeWrittenTakesTheJobAndItsRowsBackWithIt(t *testing.T) {
	w := newImportsWorld(t)
	fresh := w.email("hs-moi")
	file := csvOf("full_name,email", "Nguyễn Văn An,"+fresh)
	requestID := uuid.NewString()

	spoiled := w.everything()
	spoiled.IP = "not an address"
	if _, err := w.submit(spoiled, requestID, file, domain.DefaultStudent, domain.ImportEmailLink); err == nil {
		t.Fatal("an audit entry that cannot be written did not fail the request")
	}
	w.wantNothingStored(t, fresh)

	got, err := w.submit(w.everything(), requestID, file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil || got.Replayed || got.Job.Total != 1 {
		t.Fatalf("the same request once the audit entry can be written = %+v, %v, want a new job", got, err)
	}
}

func TestACommitThatFailsLeavesNoJobNoRowsAndNoAuditEntry(t *testing.T) {
	w := newImportsWorld(t)
	w.attachOn(t, commitRefusingPool{linkAppPool(t)}, adapters.UserImportTables{}, true)
	fresh := w.email("hs-moi")

	_, err := w.submit(w.everything(), uuid.NewString(), csvOf("full_name,email", "Nguyễn Văn An,"+fresh), domain.DefaultStudent, domain.ImportEmailLink)
	if err == nil || !strings.Contains(err.Error(), "commit refused") {
		t.Fatalf("a commit that failed answered %v, want its error", err)
	}
	w.wantNothingStored(t, fresh)
}

func (w *importsWorld) wantNothingStored(t *testing.T, email string) {
	t.Helper()
	if n := w.jobsOf(t, w.actor); n != 0 {
		t.Errorf("%d job(s) stored beside a failed request, want none", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE email = $1`, email); n != 0 {
		t.Errorf("%d row(s) stored beside a failed request, want none", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1::uuid AND entity = 'user_import'`, w.actor); n != 0 {
		t.Errorf("%d audit entr(ies) left by a request that stored nothing, want none", n)
	}
	if n := w.wake.n.Load(); n != 0 {
		t.Errorf("worker woken %d time(s) for a job that was not stored", n)
	}
}

func TestConcurrentCreatesWithOneRequestIDStoreOneJobAndAnswerAllWithIt(t *testing.T) {
	w := newImportsWorld(t)
	repo := repositories.NewUserImports(db.NewContext(linkAppPool(t)))
	requestID := uuid.NewString()
	in := domain.NewImportJob{
		RequestID: requestID, Filename: "ds.csv", Format: domain.ImportCSV, DefaultRoleID: w.role(t, "student"),
		Delivery: domain.ImportEmailLink, Now: time.Now().UTC(),
		Rows: []domain.ImportAccepted{{Ordinal: 0, FullName: "Nguyễn Văn An", Email: w.email("a"), RoleID: w.role(t, "student")}},
	}

	const callers = 12
	type answer struct {
		job      domain.ImportJob
		replayed bool
		err      error
	}
	answers := make(chan answer, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, replayed, err := repo.Create(context.Background(), w.everything(), in)
			answers <- answer{job, replayed, err}
		}()
	}
	close(start)
	wg.Wait()
	close(answers)

	var first string
	var created int
	for a := range answers {
		if a.err != nil {
			t.Fatalf("a concurrent create failed: %v", a.err)
		}
		if first == "" {
			first = a.job.ID
		}
		if a.job.ID != first {
			t.Errorf("answered with job %s and %s for one request", first, a.job.ID)
		}
		if !a.replayed {
			created++
		}
	}
	if created != 1 {
		t.Errorf("%d callers created the job, want exactly one", created)
	}
	if n := w.jobsOf(t, w.actor); n != 1 {
		t.Errorf("%d jobs stored, want 1", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE entity_id = $1::uuid`, first); n != 1 {
		t.Errorf("%d audit rows for the job, want 1", n)
	}
	if n := w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid`, first); n != 1 {
		t.Errorf("%d rows stored, want 1", n)
	}
}

func TestTwoCallersMayUseTheSameRequestIDAndNeverSeeEachOthersJob(t *testing.T) {
	w := newImportsWorld(t)
	other := w.insertUser(t, "other", "teacher")
	requestID := uuid.NewString()
	mine, err := w.submit(w.everything(), requestID, csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a")), domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	theirReq := w.everything()
	theirReq.ActorID = other
	theirs, err := w.submit(theirReq, requestID, csvOf("full_name,email", "Trần Thị Bình,"+w.email("b")), domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if theirs.Replayed || theirs.Job.ID == mine.Job.ID {
		t.Fatalf("the second caller got %+v, want a job of their own", theirs)
	}

	_, err = w.app.Queries.GetUserImport.Handle(context.Background(), query.GetUserImport{Request: theirReq, ID: mine.Job.ID})
	if !errors.Is(err, domain.ErrUserImportNotFound) {
		t.Errorf("another user's job answered %v, want not found", err)
	}
	_, err = w.app.Queries.GetUserImport.Handle(context.Background(), query.GetUserImport{Request: theirReq, ID: uuid.NewString()})
	if !errors.Is(err, domain.ErrUserImportNotFound) {
		t.Errorf("an unknown job answered %v, want the same not found", err)
	}
	own, err := w.app.Queries.GetUserImport.Handle(context.Background(), query.GetUserImport{Request: w.everything(), ID: mine.Job.ID})
	if err != nil || own.Job.ID != mine.Job.ID || own.Sheet != nil {
		t.Errorf("own job = %+v, %v, want it back with no sheet", own, err)
	}
}

func TestAFileThatWouldCreateNoAccountIsRefusedAndStoresNothing(t *testing.T) {
	w := newImportsWorld(t)
	taken := w.taken(t, "co-san")
	for name, file := range map[string][]byte{
		"every address already has an account": csvOf("full_name,email", "Lê Văn Cường,"+taken),
		"every row is refused":                 csvOf("full_name,email", "Phạm Thị Dung,sai", ",x@"+w.marker),
		"the file has a header and no row":     csvOf("full_name,email"),
	} {
		_, err := w.submit(w.everything(), uuid.NewString(), file, domain.DefaultStudent, domain.ImportEmailLink)
		if !errors.Is(err, domain.ErrImportNothingToCreate) {
			t.Errorf("%s: got %v, want ErrImportNothingToCreate", name, err)
		}
	}
	if n := w.jobsOf(t, w.actor); n != 0 {
		t.Errorf("%d job(s) stored for files that create nothing", n)
	}
	if n := w.wake.n.Load(); n != 0 {
		t.Errorf("worker woken %d time(s) for nothing", n)
	}
}

func TestARowIsJudgedAgainstWhatTheImporterMayGive(t *testing.T) {
	w := newImportsWorld(t)
	file := csvOf(
		"full_name,email,role",
		"Nguyễn Văn An,"+w.email("a")+",student",
		"Trần Thị Bình,"+w.email("b")+",teacher",
		"Lê Văn Cường,"+w.email("c")+",assistant",
		"Phạm Thị Dung,"+w.email("d")+",admin",
		"Hoàng Văn Em,"+w.email("e")+",không có vai trò này",
	)

	got, err := w.preview(w.peopleOnly(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if got.NewAccounts != 1 || got.InvalidRows != 4 {
		t.Errorf("a caller holding two permissions: %d new, %d refused, want 1 and 4", got.NewAccounts, got.InvalidRows)
	}
	wantIssue(t, got.Issues, 3, domain.FieldRole, domain.CodeRoleNotAllowed)
	wantIssue(t, got.Issues, 4, domain.FieldRole, domain.CodeRoleNotGivable)
	wantIssue(t, got.Issues, 5, domain.FieldRole, domain.CodeRoleNotAllowed)
	wantIssue(t, got.Issues, 6, domain.FieldRole, domain.CodeUnknownRole)

	full, err := w.preview(w.everything(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	wantIssue(t, full.Issues, 4, domain.FieldRole, domain.CodeRoleNotGivable)
	if _, ok := issueAt(full.Issues, 3, domain.FieldRole); ok {
		t.Error("an Admin was refused the Teacher role")
	}
}

func TestADefaultRoleTheImporterMayNotGiveIsRefusedBeforeTheFileIsRead(t *testing.T) {
	w := newImportsWorld(t)
	_, err := w.preview(w.peopleOnly(), []byte("\xff\xfe not a table"), domain.DefaultTeacher, domain.ImportEmailLink)
	if !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
		t.Errorf("got %v, want ErrTargetRoleNotAllowed", err)
	}
	_, err = w.submit(w.peopleOnly(), uuid.NewString(), csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a")), domain.DefaultTeacher, domain.ImportEmailLink)
	if !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
		t.Errorf("create: got %v, want ErrTargetRoleNotAllowed", err)
	}
	if n := w.jobsOf(t, w.actor); n != 0 {
		t.Errorf("%d job(s) stored for a role the importer may not give", n)
	}
}

func TestAClassIsNamedOnlyByAnImporterWhoReachesEveryClassAndOnlyForAStudent(t *testing.T) {
	w := newImportsWorld(t)
	open, shut := "Lớp mở "+nonce(t), "Lớp đóng "+nonce(t)
	openID := w.classNamed(t, open, false)
	w.classNamed(t, shut, true)
	file := csvOf(
		"full_name,email,role,class",
		"Nguyễn Văn An,"+w.email("a")+",student,"+strings.ToUpper(open),
		"Trần Thị Bình,"+w.email("b")+",student,"+shut,
		"Lê Văn Cường,"+w.email("c")+",teacher,"+open,
		"Phạm Thị Dung,"+w.email("d")+",student,không có lớp này",
		"Hoàng Văn Em,"+w.email("e")+",student,",
	)

	got, err := w.preview(w.everything(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if got.NewAccounts != 2 || got.InvalidRows != 3 {
		t.Errorf("%d new, %d refused, want 2 and 3: %+v", got.NewAccounts, got.InvalidRows, got.Issues)
	}
	wantIssue(t, got.Issues, 3, domain.FieldClass, domain.CodeUnknownClass)
	wantIssue(t, got.Issues, 4, domain.FieldClass, domain.CodeClassNotAllowed)
	wantIssue(t, got.Issues, 5, domain.FieldClass, domain.CodeUnknownClass)
	if len(got.Classes) != 1 || got.Classes[0].ID != openID || got.Classes[0].Name != open {
		t.Errorf("classes = %+v, want the one open class named", got.Classes)
	}

	stored, err := w.submit(w.everything(), uuid.NewString(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	rows := w.storedRows(t, stored.Job.ID)
	if len(rows) != 2 || rows[0].ClassID == nil || *rows[0].ClassID != openID || rows[1].ClassID != nil {
		t.Errorf("stored rows = %+v, want the first placed in the open class and the second in none", rows)
	}

	scoped, err := w.preview(w.peopleOnly(), file, domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	wantIssue(t, scoped.Issues, 2, domain.FieldClass, domain.CodeClassNotAllowed)
	if len(scoped.Classes) != 0 {
		t.Errorf("classes = %+v for an importer who reaches only their own", scoped.Classes)
	}
}

func TestADeliveryNeedsPasswordSignInOrAServerThatCanSealASecretAsItsKindAsks(t *testing.T) {
	w := newImportsWorld(t)
	file := csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a"))

	w.policy = policyWith(passwordOff)
	if _, err := w.preview(w.everything(), file, domain.DefaultStudent, domain.ImportSheet); !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Errorf("sheet with password sign-in off: got %v, want ErrPasswordSignInDisabled", err)
	}
	if _, err := w.preview(w.everything(), file, domain.DefaultStudent, domain.ImportEmailLink); err != nil {
		t.Errorf("an email import with password sign-in off was refused: %v", err)
	}

	w.policy = openPolicy()
	w.attach(t, false)
	if _, err := w.submit(w.everything(), uuid.NewString(), file, domain.DefaultStudent, domain.ImportSheet); !errors.Is(err, domain.ErrImportSheetsOff) {
		t.Errorf("sheet on a server that cannot keep one: got %v, want ErrImportSheetsOff", err)
	}
	if _, err := w.submit(w.everything(), uuid.NewString(), file, domain.DefaultStudent, domain.ImportEmailLink); !errors.Is(err, domain.ErrSetPasswordUnavailable) {
		t.Errorf("email on a server that cannot seal a link: got %v, want ErrSetPasswordUnavailable", err)
	}
	if n := w.jobsOf(t, w.actor); n != 0 {
		t.Errorf("%d job(s) stored for a refused delivery", n)
	}
}

func TestASheetJobReportsItsSheetAsNotYetAvailable(t *testing.T) {
	w := newImportsWorld(t)
	got, err := w.submit(w.everything(), uuid.NewString(), csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a")), domain.DefaultStudent, domain.ImportSheet)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sheet == nil || got.Sheet.Available || got.Sheet.ExpiresAt != nil || got.Sheet.DownloadedAt != nil {
		t.Errorf("sheet = %+v, want one that is not available yet", got.Sheet)
	}
	read, err := w.app.Queries.GetUserImport.Handle(context.Background(), query.GetUserImport{Request: w.everything(), ID: got.Job.ID})
	if err != nil || read.Sheet == nil || read.Sheet.Available {
		t.Errorf("read = %+v, %v, want the same state", read, err)
	}
}

func TestAWorkbookIsRecognisedByItsBytesAndNotByItsName(t *testing.T) {
	w := newImportsWorld(t)
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	for i, row := range [][]any{
		{"full_name", "email"},
		{"Nguyễn Văn An", w.email("a")},
		{"Trần Thị Bình", w.email("b")},
	} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.app.Commands.CreateUserImport.Handle(context.Background(), command.CreateUserImport{
		Request: w.everything(), RequestID: uuid.NewString(), Filename: "danh-sach.csv", File: buffer.Bytes(),
		DefaultRole: domain.DefaultStudent, Delivery: domain.ImportEmailLink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Job.Format != domain.ImportXLSX || got.Job.Total != 2 {
		t.Errorf("job = %+v, want an xlsx of 2 rows though the name says csv", got.Job)
	}
}

func TestTheReadersRefusalsReachTheCallerAsTheirOwnErrors(t *testing.T) {
	w := newImportsWorld(t)
	cases := []struct {
		name string
		file []byte
		want error
	}{
		{"a file in another encoding", append([]byte("full_name,email\n"), 0xff, 0xfe, ',', 'x', '\n'), tabular.ErrNotUTF8},
		{"a file with no header row", nil, tabular.ErrEmpty},
		{"a book that is not a workbook", append([]byte("PK\x03\x04"), []byte(strings.Repeat("x", 64))...), tabular.ErrFormat},
	}
	for _, c := range cases {
		if _, err := w.preview(w.everything(), c.file, domain.DefaultStudent, domain.ImportEmailLink); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}

	var rows strings.Builder
	rows.WriteString("full_name,email\n")
	for i := range 2001 {
		fmt.Fprintf(&rows, "Học sinh %d,hs%d@%s\n", i, i, w.marker)
	}
	if _, err := w.preview(w.everything(), []byte(rows.String()), domain.DefaultStudent, domain.ImportEmailLink); !errors.Is(err, tabular.ErrTooManyRows) {
		t.Errorf("2,001 rows: got %v, want ErrTooManyRows", err)
	}
}

func TestAHeaderWithoutARequiredColumnOrWithOneTwiceIsRefusedWithWhatToFix(t *testing.T) {
	w := newImportsWorld(t)
	for _, c := range []struct {
		header            string
		missing, repeated []domain.ImportField
	}{
		{"full_name", []domain.ImportField{domain.FieldEmail}, nil},
		{"Email,Họ tên", []domain.ImportField{domain.FieldFullName}, nil},
		{"full_name,email,EMAIL", nil, []domain.ImportField{domain.FieldEmail}},
	} {
		_, err := w.preview(w.everything(), csvOf(c.header, "a,b"), domain.DefaultStudent, domain.ImportEmailLink)
		var header *domain.ImportHeaderError
		if !errors.As(err, &header) {
			t.Errorf("%q: got %v, want a header problem", c.header, err)
			continue
		}
		if fmt.Sprint(header.Problem.Missing) != fmt.Sprint(c.missing) || fmt.Sprint(header.Problem.Repeated) != fmt.Sprint(c.repeated) {
			t.Errorf("%q: problem = %+v, want missing %v and repeated %v", c.header, header.Problem, c.missing, c.repeated)
		}
	}
}

func TestTheLargestImportIsStoredWholeAndInOrder(t *testing.T) {
	w := newImportsWorld(t)
	var file strings.Builder
	file.WriteString("full_name,email\n")
	for i := range domain.MaxImportRows {
		fmt.Fprintf(&file, "Học sinh %d,hs%04d@%s\n", i, i, w.marker)
	}

	got, err := w.submit(w.everything(), uuid.NewString(), []byte(file.String()), domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if got.Job.Total != domain.MaxImportRows {
		t.Fatalf("total = %d, want %d", got.Job.Total, domain.MaxImportRows)
	}
	rows := w.storedRows(t, got.Job.ID)
	if len(rows) != domain.MaxImportRows || rows[0].Ordinal != 0 || rows[len(rows)-1].Ordinal != domain.MaxImportRows-1 ||
		rows[1999].Email != "hs1999@"+w.marker || rows[0].FullName != "Học sinh 0" {
		t.Errorf("stored %d rows, first %+v, last %+v, want 2,000 in file order", len(rows), rows[0], rows[len(rows)-1])
	}
}

func TestARowKeepsASealedSecretOnlyOnceItIsCreated(t *testing.T) {
	w := newImportsWorld(t)
	got, err := w.submit(w.everything(), uuid.NewString(), csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a")), domain.DefaultStudent, domain.ImportEmailLink)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_rows SET outcome = 'created', secret_ciphertext = '\x00' WHERE job_id = $1::uuid`, got.Job.ID); err != nil {
		t.Fatalf("a created row cannot hold its sealed secret: %v", err)
	}
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_rows SET outcome = 'pending' WHERE job_id = $1::uuid`, got.Job.ID); err == nil {
		t.Error("a pending row kept a sealed secret")
	}
}

func TestOnlyOneFileIsParsedAtATimeAndAWaitingUploadMayLeave(t *testing.T) {
	w := newImportsWorld(t)
	gate := gatedTables{entered: make(chan struct{}, 4), release: make(chan struct{})}
	w.attachTables(t, gate, true)
	file := csvOf("full_name,email", "Nguyễn Văn An,"+w.email("a"))
	preview := func(ctx context.Context) error {
		_, err := w.app.Commands.PreviewUserImport.Handle(ctx, command.PreviewUserImport{
			Request: w.everything(), File: file, DefaultRole: domain.DefaultStudent, Delivery: domain.ImportEmailLink,
		})
		return err
	}

	holder := make(chan error, 1)
	go func() { holder <- preview(context.Background()) }()
	<-gate.entered

	leaving, leave := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() { waiting <- preview(leaving) }()
	queued := make(chan error, 1)
	go func() { queued <- preview(context.Background()) }()
	select {
	case <-gate.entered:
		t.Fatal("a second file was parsed while the first still held the slot")
	case <-time.After(300 * time.Millisecond):
	}

	leave()
	if err := <-waiting; !errors.Is(err, context.Canceled) {
		t.Errorf("the upload that left answered %v, want context.Canceled", err)
	}
	gate.release <- struct{}{}
	if err := <-holder; err != nil {
		t.Errorf("the first file: %v", err)
	}
	<-gate.entered
	gate.release <- struct{}{}
	if err := <-queued; err != nil {
		t.Errorf("the file that waited: %v", err)
	}
}
