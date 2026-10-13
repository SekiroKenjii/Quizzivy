//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/internal/core/adapters"
	aiapp "quizzivy/internal/modules/ai/application"
	aiports "quizzivy/internal/modules/ai/application/ports"
	aidomain "quizzivy/internal/modules/ai/domain"
	airepo "quizzivy/internal/modules/ai/repositories"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/reader"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/pdftext/tests/scanpdf"
)

type scanProvider struct{ calls atomic.Int64 }

func (p *scanProvider) Complete(context.Context, aiports.Request) (aiports.Response, error) {
	n := p.calls.Add(1)
	object, _ := json.Marshal(map[string]any{"readable": true, "lines": []string{
		"Part I. Choose the best answer", "1. Which one is a fruit " + string(rune('a'+n)) + "?", "A. apple    B. chair",
	}})
	return aiports.Response{Object: object, Usage: aiports.Usage{InputTokens: 2000, OutputTokens: 1200}, RequestID: "req"}, nil
}

type creditsWorld struct {
	h       harness
	month   time.Time
	credits *aiapp.Application
}

func newCreditsWorld(t *testing.T) creditsWorld {
	t.Helper()
	h := setup(t)
	month := time.Date(2400+int(time.Now().UnixNano()%90), time.March, 1, 0, 0, 0, 0, time.UTC)
	credits := aiapp.New(airepo.NewPostgres(db.NewContext(h.pool)), aiapp.Options{
		Prices: aidomain.Prices{Fast: aidomain.Price{Input: 100_000, Output: 500_000}, Capable: aidomain.Price{Input: 2_000_000, Output: 10_000_000}},
		Zone:   time.UTC,
	})
	credits.SetClock(func() time.Time { return month.Add(14 * 24 * time.Hour) })
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM app.ai_reservations WHERE month = $1::date`,
			`DELETE FROM app.ai_usage_ledger WHERE month = $1::date`,
			`DELETE FROM app.ai_teacher_credits WHERE month = $1::date`,
			`DELETE FROM app.ai_credit_periods WHERE month = $1::date`,
		} {
			if _, err := h.pool.Exec(context.Background(), sql, month); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
	})
	return creditsWorld{h: h, month: month, credits: credits}
}

func (w creditsWorld) allowance(t *testing.T, credits, used int64) {
	t.Helper()
	for _, sql := range []string{
		`INSERT INTO app.ai_credit_periods (month, pool_credits) SELECT $1::date, pool_credits FROM app.ai_settings ON CONFLICT (month) DO NOTHING`,
	} {
		if _, err := w.h.pool.Exec(context.Background(), sql, w.month); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.h.pool.Exec(context.Background(), `INSERT INTO app.ai_teacher_credits (month, user_id, allowance_credits, used_credits) VALUES ($1::date, $2::uuid, $3, $4)`, w.month, w.h.actor.ID, credits, used); err != nil {
		t.Fatal(err)
	}
}

func (w creditsWorld) usage(t *testing.T) (used, reserved, ledgerRows int64) {
	t.Helper()
	err := w.h.pool.QueryRow(context.Background(), `SELECT coalesce((SELECT used_credits FROM app.ai_teacher_credits WHERE month=$1::date AND user_id=$2::uuid),0),
 coalesce((SELECT reserved_credits FROM app.ai_teacher_credits WHERE month=$1::date AND user_id=$2::uuid),0),
 (SELECT count(*) FROM app.ai_usage_ledger WHERE month=$1::date AND user_id=$2::uuid AND feature='import')`, w.month, w.h.actor.ID).Scan(&used, &reserved, &ledgerRows)
	if err != nil {
		t.Fatal(err)
	}
	return used, reserved, ledgerRows
}

type scanRun struct {
	runner worker.Runner
	h      harness
	id     string
}

func (w creditsWorld) processScan(t *testing.T, readWithAi bool, provider *scanProvider) (domain.Import, domain.Run) {
	t.Helper()
	updated, run, _ := w.startScan(t, readWithAi, provider)
	return updated, run
}

func (r scanRun) again(t *testing.T) (domain.Import, domain.Run) {
	t.Helper()
	ctx := context.Background()
	current, err := r.h.repo.Get(ctx, everyone, r.id)
	if err != nil {
		t.Fatal(err)
	}
	run, err := r.h.repo.Schedule(ctx, domain.Schedule{ImportID: r.id, RequestID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, SourceRevision: current.SourceRevision, ExpectedRevision: current.Revision, Actor: r.h.actor, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := r.runner.RunOne(ctx); err != nil || !worked {
		t.Fatalf("processing again: %v", err)
	}
	updated, err := r.h.repo.Get(ctx, everyone, r.id)
	if err != nil {
		t.Fatal(err)
	}
	finished, err := r.h.repo.Run(ctx, r.id, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return updated, finished
}

func (w creditsWorld) startScan(t *testing.T, readWithAi bool, provider *scanProvider) (domain.Import, domain.Run, scanRun) {
	t.Helper()
	ctx := context.Background()
	store := privateStore(t)
	work := t.TempDir()
	pdf := &pdftext.Reader{}
	t.Cleanup(func() { _ = pdf.Close() })
	parent, err := w.h.repo.Create(ctx, domain.Create{RequestID: uuid.NewString(), Title: "Đề scan", ReadWithAi: readWithAi, Actor: w.h.actor}, w.h.quotas)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupPipelineObjects(t, w.h, store, parent.ID) })
	intake := command.UploadHandler{Repo: w.h.repo, Store: store, Inspector: adapters.ImportInspector{}, WorkDir: work, Quotas: w.h.quotas, Slots: make(chan struct{}, 1)}
	receipt, err := intake.Handle(ctx, command.Upload{ImportID: parent.ID, UploadID: uuid.NewString(), Role: "exam", Filename: "scan.pdf", ExpectedRevision: parent.Revision, Actor: w.h.actor, Body: bytes.NewReader(scanpdf.Build(2))})
	if err != nil {
		t.Fatal(err)
	}
	parent = receipt.Import
	if parent.ReadWithAi != readWithAi {
		t.Fatalf("the import reads back with readWithAi %v, created with %v", parent.ReadWithAi, readWithAi)
	}
	run, err := w.h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, SourceRevision: parent.SourceRevision, ExpectedRevision: parent.Revision, Actor: w.h.actor, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	engine := adapters.ImportProcessing{PDF: pdf, WorkDir: work, AI: &reader.Reader{
		Pages: adapters.PDFPages{PDF: pdf}, Reserve: w.credits.Commands.Reserve, Finish: w.credits.Commands.Finish, Completer: provider,
	}}
	processor := worker.Pipeline{Sources: w.h.repo, Artifacts: w.h.repo, Store: store, Engine: engine, Quotas: artifactQuotas(), WorkDir: work}
	runner := worker.Runner{Queue: w.h.repo, Processor: processor, Policy: policy(worker.PipelineVersion), HeartbeatEvery: time.Second, Timeout: 2 * time.Minute}
	if worked, err := runner.RunOne(ctx); err != nil || !worked {
		t.Fatalf("processing: %v", err)
	}
	updated, err := w.h.repo.Get(ctx, everyone, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	finished, err := w.h.repo.Run(ctx, parent.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return updated, finished, scanRun{runner: runner, h: w.h, id: parent.ID}
}

func TestAScannedExamIsReadWithAIAndMeteredThroughTheRealCredits(t *testing.T) {
	w := newCreditsWorld(t)
	w.allowance(t, 1000, 0)
	provider := &scanProvider{}
	updated, run := w.processScan(t, true, provider)
	if updated.Status != "needs_review" || run.Status != "succeeded" || !updated.ReadWithAi {
		t.Fatalf("import %+v, run %+v", updated, run)
	}
	if provider.calls.Load() != 2 {
		t.Fatalf("%d pages sent, want 2", provider.calls.Load())
	}
	used, reserved, rows := w.usage(t)
	if used != 32 || reserved != 0 || rows != 1 {
		t.Fatalf("used %d, still reserved %d, ledger rows %d; want 32 credits settled in one row", used, reserved, rows)
	}
}

func TestProcessingAReadScanAgainReusesTheReadAndSpendsNothingMore(t *testing.T) {
	w := newCreditsWorld(t)
	w.allowance(t, 1000, 0)
	provider := &scanProvider{}
	_, _, scan := w.startScan(t, true, provider)
	updated, run := scan.again(t)
	if updated.Status != "needs_review" || run.Status != "succeeded" {
		t.Fatalf("import %+v, run %+v", updated, run)
	}
	if provider.calls.Load() != 2 {
		t.Fatalf("%d pages sent over two runs, want the first run's 2", provider.calls.Load())
	}
	if used, reserved, rows := w.usage(t); used != 32 || reserved != 0 || rows != 1 {
		t.Fatalf("used %d, reserved %d, rows %d", used, reserved, rows)
	}
}

func TestAScanWithoutTheSwitchStillFailsWithNoTextAndSpendsNothing(t *testing.T) {
	w := newCreditsWorld(t)
	w.allowance(t, 1000, 0)
	provider := &scanProvider{}
	updated, run := w.processScan(t, false, provider)
	if updated.Status != "failed" || run.ErrorCode == nil || *run.ErrorCode != "PDF_NO_TEXT" || provider.calls.Load() != 0 {
		t.Fatalf("import %+v, run %+v, %d calls", updated, run, provider.calls.Load())
	}
	if used, reserved, rows := w.usage(t); used != 0 || reserved != 0 || rows != 0 {
		t.Fatalf("used %d, reserved %d, rows %d", used, reserved, rows)
	}
}

func TestAScanTheAllowanceCannotCoverIsRefusedWholeAndNothingIsSent(t *testing.T) {
	w := newCreditsWorld(t)
	w.allowance(t, 1000, 950)
	provider := &scanProvider{}
	updated, run := w.processScan(t, true, provider)
	if updated.Status != "failed" || run.ErrorCode == nil || *run.ErrorCode != "AI_CREDITS_EXHAUSTED" || provider.calls.Load() != 0 {
		t.Fatalf("import %+v, run %+v, %d calls", updated, run, provider.calls.Load())
	}
	if used, reserved, rows := w.usage(t); used != 950 || reserved != 0 || rows != 0 {
		t.Fatalf("used %d, reserved %d, rows %d: a refused file must hold nothing", used, reserved, rows)
	}
}

func TestACreatedImportRefusesAReplayThatChangesTheSwitch(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	request := uuid.NewString()
	first, err := h.repo.Create(ctx, domain.Create{RequestID: request, Title: "Đề", ReadWithAi: true, Actor: h.actor}, h.quotas)
	if err != nil || !first.ReadWithAi {
		t.Fatalf("created %+v, %v", first, err)
	}
	again, err := h.repo.Create(ctx, domain.Create{RequestID: request, Title: "Đề", ReadWithAi: true, Actor: h.actor}, h.quotas)
	if err != nil || again.ID != first.ID {
		t.Fatalf("an identical replay: %+v, %v", again, err)
	}
	if _, err := h.repo.Create(ctx, domain.Create{RequestID: request, Title: "Đề", ReadWithAi: false, Actor: h.actor}, h.quotas); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("a replay that drops the switch: %v, want ErrConflict", err)
	}
}
