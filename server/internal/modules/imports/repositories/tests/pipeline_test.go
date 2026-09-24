//go:build integration

package repositories_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/storage"
	"quizzivy/internal/platform/wordconvert"
	"testing"
	"time"
)

type interruptedEngine struct {
	ports.ProcessingEngine
	normalizations, extractions int
}

func (e *interruptedEngine) Normalize(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	e.normalizations++
	return e.ProcessingEngine.Normalize(ctx, in)
}
func (e *interruptedEngine) Extract(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	e.extractions++
	if e.extractions == 1 {
		return ports.StageOutput{}, worker.Failure{Code: "STORAGE_UNAVAILABLE", Retryable: true}
	}
	return e.ProcessingEngine.Extract(ctx, in)
}

func nativePaper(t *testing.T, paragraphs []string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	body := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`
	for _, p := range paragraphs {
		body += `<w:p><w:r><w:t>` + p + `</w:t></w:r></w:p>`
	}
	body += `</w:body></w:document>`
	files := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   body,
	}
	for name, data := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestRealPipelineResumesPrivateArtifactsAndKeepsCandidateOutOfRunEnvelope(t *testing.T) {
	image := os.Getenv("TEST_WORD_CONVERTER_IMAGE")
	if image == "" {
		t.Skip("isolated local converter image required")
	}
	h := setup(t)
	store := privateStore(t)
	ctx := context.Background()
	work := t.TempDir()
	if err := os.Chmod(work, 0o700); err != nil {
		t.Fatal(err)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	converter, err := wordconvert.New(work, docker, image, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	engine := &interruptedEngine{ProcessingEngine: adapters.ImportProcessing{Converter: converter, ImageID: image, WorkDir: work}}
	parent := h.create(t)
	t.Cleanup(func() { cleanupPipelineObjects(t, h, store, parent.ID) })
	intake := command.UploadHandler{Repo: h.repo, Store: store, Inspector: adapters.ImportInspector{}, WorkDir: work, Quotas: h.quotas, Slots: make(chan struct{}, 1)}
	for _, source := range []struct {
		role  string
		lines []string
	}{{"exam", []string{"Part I: Synthetic", "Question 1 Choose A. first B. second"}}, {"answer_key", []string{"Part I: Synthetic", "Question 1. B"}}} {
		receipt, err := intake.Handle(ctx, command.Upload{ImportID: parent.ID, UploadID: uuid.NewString(), Role: source.role, Filename: "synthetic.docx", ExpectedRevision: parent.Revision, Actor: h.actor, Body: bytes.NewReader(nativePaper(t, source.lines))})
		if err != nil {
			t.Fatal(err)
		}
		parent = receipt.Import
	}
	run, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, SourceRevision: parent.SourceRevision, ExpectedRevision: parent.Revision, Actor: h.actor, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	processor := worker.Pipeline{Sources: h.repo, Artifacts: h.repo, Store: store, Engine: engine, Quotas: artifactQuotas(), WorkDir: work}
	p := policy(worker.PipelineVersion)
	p.Lease = 10 * time.Second
	runner := worker.Runner{Queue: h.repo, Processor: processor, Policy: p, HeartbeatEvery: time.Second, Timeout: time.Minute}
	if worked, err := runner.RunOne(ctx); err != nil || !worked {
		t.Fatalf("first execution: %v", err)
	}
	failed, err := h.repo.Run(ctx, parent.ID, run.ID)
	if err != nil || failed.Status != "queued" {
		t.Fatalf("retry was not durable: %v", err)
	}
	if worked, err := runner.RunOne(ctx); err != nil || !worked {
		t.Fatalf("resumed execution: %v", err)
	}
	completed, err := h.repo.Run(ctx, parent.ID, run.ID)
	if err != nil || completed.Status != "succeeded" {
		t.Fatalf("pipeline did not finish: %+v %v", completed, err)
	}
	if engine.normalizations != 2 || engine.extractions != 3 {
		t.Fatalf("completed converter stages were repeated: %d/%d", engine.normalizations, engine.extractions)
	}
	var result domain.ProcessingResult
	if err := json.Unmarshal(completed.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 2 || result.CandidateSetID == "" || bytes.Contains(completed.Result, []byte("prompt")) {
		t.Fatal("run envelope contains source/candidate bodies or lost lineage")
	}
	set, err := h.repo.ArtifactSet(ctx, parent.ID, result.CandidateSetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Files) != 1 {
		t.Fatal("candidate artifact missing")
	}
	a := set.Files[0]
	file, err := worker.FetchPrivate(ctx, store, work, a.StorageKey, a.Bytes, a.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var candidate domain.Candidate
	if err := json.NewDecoder(file).Decode(&candidate); err != nil {
		t.Fatal(err)
	}
	if len(candidate.Questions) != 1 || candidate.Questions[0].Answer.State != "known" || candidate.Questions[0].Answer.OptionIDs[0] != candidate.Questions[0].Options[1].ID {
		t.Fatal("real source/key integration changed association")
	}
	updated, err := h.repo.Get(ctx, parent.ID)
	if err != nil || updated.Status != "needs_review" || len(candidate.Issues) == 0 {
		t.Fatal("processing bypassed teacher review")
	}
	var unfinished int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM app.word_import_artifact_sets WHERE import_id=$1 AND NOT ready`, parent.ID).Scan(&unfinished); err != nil || unfinished != 0 {
		t.Fatalf("completed pipeline left incomplete artifacts: %v", err)
	}
}

func cleanupPipelineObjects(t *testing.T, h harness, store *storage.Client, importID string) {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT storage_key FROM app.word_import_sources WHERE import_id=$1 UNION ALL SELECT storage_key FROM app.word_import_artifacts WHERE import_id=$1`, importID)
	if err != nil {
		t.Error(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Error(err)
			return
		}
		if err := store.Delete(context.Background(), key); err != nil {
			t.Error(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Error(err)
	}
}
