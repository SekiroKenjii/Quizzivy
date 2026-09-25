//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/pdftext"
)

type convertingEngine struct {
	ports.ProcessingEngine
	normalizations int
	formats        []string
}

func (e *convertingEngine) Converts() bool { return true }

func (e *convertingEngine) Normalize(context.Context, ports.DocumentInput) (ports.StageOutput, error) {
	e.normalizations++
	return ports.StageOutput{}, errors.New("a PDF must not be converted")
}

func (e *convertingEngine) Extract(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	e.formats = append(e.formats, in.Format)
	return e.ProcessingEngine.Extract(ctx, in)
}

func textPDF(lines []string) []byte {
	var content strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&content, "BT /F1 11 Tf 72 %d Td (%s) Tj ET\n", 780-18*i, line)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents 5 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func TestAPDFExamAndKeyReachReviewWithoutConversion(t *testing.T) {
	h := setup(t)
	store := privateStore(t)
	ctx := context.Background()
	work := t.TempDir()
	reader := &pdftext.Reader{}
	t.Cleanup(func() { _ = reader.Close() })
	engine := &convertingEngine{ProcessingEngine: adapters.ImportProcessing{PDF: reader, WorkDir: work}}
	parent := h.create(t)
	t.Cleanup(func() { cleanupPipelineObjects(t, h, store, parent.ID) })
	intake := command.UploadHandler{Repo: h.repo, Store: store, Inspector: adapters.ImportInspector{}, WorkDir: work, Quotas: h.quotas, Slots: make(chan struct{}, 1)}
	for _, source := range []struct {
		role  string
		lines []string
	}{{"exam", []string{"Part I. Choose the best answer", "1. Which one is a fruit?", "A. apple    B. chair"}}, {"answer_key", []string{"1. A"}}} {
		receipt, err := intake.Handle(ctx, command.Upload{ImportID: parent.ID, UploadID: uuid.NewString(), Role: source.role, Filename: "De thi.PDF", ExpectedRevision: parent.Revision, Actor: h.actor, Body: bytes.NewReader(textPDF(source.lines))})
		if err != nil {
			t.Fatal(err)
		}
		parent = receipt.Import
	}
	for _, s := range parent.Sources {
		if s.Format != "pdf" {
			t.Fatalf("source %s stored as %q", s.Role, s.Format)
		}
	}
	run, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, SourceRevision: parent.SourceRevision, ExpectedRevision: parent.Revision, Actor: h.actor, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	processor := worker.Pipeline{Sources: h.repo, Artifacts: h.repo, Store: store, Engine: engine, Quotas: artifactQuotas(), WorkDir: work}
	runner := worker.Runner{Queue: h.repo, Processor: processor, Policy: policy(worker.PipelineVersion), HeartbeatEvery: time.Second, Timeout: time.Minute}
	if worked, err := runner.RunOne(ctx); err != nil || !worked {
		t.Fatalf("processing: %v", err)
	}
	if engine.normalizations != 0 || strings.Join(engine.formats, ",") != "pdf,pdf" {
		t.Fatalf("normalized %d times, extracted %v", engine.normalizations, engine.formats)
	}
	updated, err := h.repo.Get(ctx, parent.ID)
	if err != nil || updated.Status != "needs_review" {
		t.Fatalf("import = %+v, %v", updated, err)
	}
	completed, err := h.repo.Run(ctx, parent.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var result domain.ProcessingResult
	if err := json.Unmarshal(completed.Result, &result); err != nil {
		t.Fatal(err)
	}
	set, err := h.repo.ArtifactSet(ctx, parent.ID, result.CandidateSetID)
	if err != nil || len(set.Files) != 1 {
		t.Fatalf("candidate set: %v", err)
	}
	file, err := worker.FetchPrivate(ctx, store, work, set.Files[0].StorageKey, set.Files[0].Bytes, set.Files[0].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var candidate domain.Draft
	if err := json.NewDecoder(file).Decode(&candidate); err != nil {
		t.Fatal(err)
	}
	questions := candidate.Questions()
	if len(questions) != 1 || questions[0].Answer.State != domain.AnswerKnown || questions[0].Answer.OptionIDs[0] != questions[0].Options[0].ID {
		t.Fatalf("questions = %+v", questions)
	}
}
