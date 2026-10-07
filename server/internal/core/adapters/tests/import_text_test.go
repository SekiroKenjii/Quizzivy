package adapters_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"reflect"
	"strings"
	"testing"
)

func TestTextEvidencePreservesPlainContentAndOriginalNormalizedLineNumbers(t *testing.T) {
	text := "\ufefftiếng 😀\r\n \r<html>literal</html>\n\nlast"
	doc, err := adapters.TextEvidence(text, "source", "exam")
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceID != "source" || doc.Role != "exam" || doc.Version != "text-lines-v1" || len(doc.Findings) != 1 || doc.Findings[0].Code != "TEXT_MARKS_UNAVAILABLE" || !doc.Findings[0].Main {
		t.Fatalf("projection identity %+v", doc)
	}
	want := []string{"tiếng 😀", "<html>literal</html>", "last"}
	ids := []string{"l1", "l3", "l5"}
	if len(doc.Blocks) != len(want) {
		t.Fatal(doc.Blocks)
	}
	for i, b := range doc.Blocks {
		if b.Text != want[i] || b.ID != ids[i] || b.Kind != "paragraph" || !b.Main || !b.Meaningful || !b.Safe || len(b.Spans) != 0 || len(b.Reasons) != 0 {
			t.Fatalf("block %d: %+v", i, b)
		}
	}
	again, err := adapters.TextEvidence(text, "source", "exam")
	if err != nil || !reflect.DeepEqual(doc, again) {
		t.Fatal("projection is not deterministic")
	}
}

func TestTextEvidenceEnforcesScalarAndLineBounds(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       error
	}{
		{"maximum", strings.Repeat("😀", 100000), nil}, {"overlimit", strings.Repeat("😀", 100001), domain.ErrTooLarge},
		{"NFC maximum", strings.Repeat("e\u0301", 100000), nil},
		{"maximum lines", strings.Repeat("x\n", 20000), nil}, {"extra line", strings.Repeat("x\n", 20001), domain.ErrTooLarge},
		{"blank", "\ufeff \n\r\t", domain.ErrInvalid}, {"NUL", "x\x00", domain.ErrInvalid}, {"invalid UTF8", string([]byte{0xff}), domain.ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := adapters.TextEvidence(test.text, "source", "exam")
			if !errors.Is(err, test.want) {
				t.Fatalf("projection: %v want %v", err, test.want)
			}
		})
	}
}

func TestTextExtractionUsesPrivateVersionedParagraphArtifactsWithoutConverter(t *testing.T) {
	p := adapters.ImportProcessing{WorkDir: t.TempDir()}
	text := "Question 1. Plain question\nA. first\nB. second"
	in := ports.DocumentInput{OriginalID: "original", Identity: "original", Role: "exam", Format: "text", Body: bytes.NewReader([]byte(text)), Bytes: int64(len(text))}
	out, err := p.Extract(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			t.Error(err)
		}
	}()
	if out.Plan.SourceID != "original" || out.Plan.Stage != "extraction" || out.Plan.ComponentVersion != "text-lines-v1:projection-v1" || p.ExtractionVersion("docx") == p.ExtractionVersion("text") || p.ExtractionVersion("pdf") == p.ExtractionVersion("text") {
		t.Fatalf("plan %+v", out.Plan)
	}
	var manifest domain.ExtractionManifest
	if err := json.Unmarshal(out.Plan.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "text-lines-v1" || manifest.Identity != "original" || manifest.MainPart != "text" || manifest.BlockCount != 3 {
		t.Fatalf("manifest %+v", manifest)
	}
	body, err := out.Open(manifest.EvidenceFile)
	if err != nil {
		t.Fatal(err)
	}
	var doc domain.EvidenceDocument
	err = json.NewDecoder(body).Decode(&doc)
	closeErr := body.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("evidence: %v %v", err, closeErr)
	}
	if len(doc.Blocks) != 3 || doc.Blocks[0].Text != "Question 1. Plain question" {
		t.Fatal(doc)
	}
	for _, f := range out.Plan.Files {
		body, err := out.Open(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(body)
		closeErr := body.Close()
		if err != nil || closeErr != nil || int64(len(data)) != f.Bytes {
			t.Fatalf("artifact: %v %v", err, closeErr)
		}
	}
	if _, _, err := out.Plan.Digest(); err != nil {
		t.Fatal(err)
	}
}

func TestTextEvidenceAllowsReviewWithoutQuestionsAndKeepsMarksInformational(t *testing.T) {
	doc, err := adapters.TextEvidence("Only a heading", "source", "exam")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{doc}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	noQuestions, marks := false, false
	for _, n := range domain.Assess(draft).Findings {
		if n.Code == "NO_QUESTIONS" {
			noQuestions = true
		}
		if n.Field == "TEXT_MARKS_UNAVAILABLE" {
			marks = true
			if n.Severity != domain.Informational {
				t.Fatalf("marks block review: %+v", n)
			}
		}
	}
	if !noQuestions || !marks {
		t.Fatalf("missing review notices %+v", draft.Notices)
	}
}

type textSources struct {
	sources []domain.Source
	calls   int
}

func (s *textSources) Sources(context.Context, string, int64) ([]domain.Source, error) {
	s.calls++
	return s.sources, nil
}

type textObjects map[string][]byte

func (s textObjects) Open(_ context.Context, key string) (io.ReadCloser, int64, error) {
	data, ok := s[key]
	if !ok {
		return nil, 0, domain.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}
func (s textObjects) PutImmutable(_ context.Context, key, _ string, body io.ReadSeeker, n int64, digest []byte) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	if int64(len(data)) != n || !bytes.Equal(hash[:], digest) {
		return errors.New("artifact identity mismatch")
	}
	s[key] = data
	return nil
}
func (s textObjects) Delete(_ context.Context, key string) error { delete(s, key); return nil }

type textArtifacts struct {
	domain.Artifacts
	sets map[string]domain.ArtifactSet
}

func (s *textArtifacts) ReusableArtifacts(_ context.Context, _ domain.Claim, role, stage, version string) (domain.ArtifactSet, error) {
	for _, set := range s.sets {
		if set.Role == role && set.Stage == stage && set.ComponentVersion == version && set.Ready {
			return set, nil
		}
	}
	return domain.ArtifactSet{}, domain.ErrNotFound
}
func (s *textArtifacts) ReserveArtifacts(_ context.Context, c domain.Claim, p domain.ArtifactPlan, _ domain.ArtifactQuotas) (domain.ArtifactSet, error) {
	digest, n, err := p.Digest()
	if err != nil {
		return domain.ArtifactSet{}, err
	}
	id := fmt.Sprintf("set-%d", len(s.sets)+1)
	set := domain.ArtifactSet{ID: id, ImportID: c.ImportID, SourceID: p.SourceID, Role: p.Role, Stage: p.Stage, ComponentVersion: p.ComponentVersion, Manifest: p.Manifest, PlanDigest: digest, Bytes: n, FileCount: len(p.Files)}
	for i, f := range p.Files {
		set.Files = append(set.Files, domain.Artifact{ArtifactSpec: f, ID: fmt.Sprintf("file-%d", i), StorageKey: id + "/" + f.Name})
	}
	s.sets[id] = set
	return set, nil
}
func (s *textArtifacts) ArtifactStored(_ context.Context, _ domain.Claim, id, file string) error {
	set := s.sets[id]
	for i := range set.Files {
		if set.Files[i].ID == file {
			set.Files[i].Ready = true
		}
	}
	s.sets[id] = set
	return nil
}
func (s *textArtifacts) FinishArtifacts(_ context.Context, _ domain.Claim, id string) (domain.ArtifactSet, error) {
	set := s.sets[id]
	for _, file := range set.Files {
		if !file.Ready {
			return set, errors.New("partial artifact")
		}
	}
	set.Ready = true
	s.sets[id] = set
	return set, nil
}

type textEngine struct {
	adapters.ImportProcessing
	normalizations, extractions int
}

func (*textEngine) Converts() bool { return true }
func (e *textEngine) Normalize(context.Context, ports.DocumentInput) (ports.StageOutput, error) {
	e.normalizations++
	return ports.StageOutput{}, errors.New("plain text reached converter")
}
func (e *textEngine) Extract(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	e.extractions++
	return e.ImportProcessing.Extract(ctx, in)
}

func TestTextPipelineSkipsConversionReusesEvidenceAndFencesOldWorkers(t *testing.T) {
	text := "Only a heading"
	digest := sha256.Sum256([]byte(text))
	sources := &textSources{sources: []domain.Source{{ID: "source", Role: "exam", Format: "text", StorageKey: "original", Bytes: int64(len(text)), SHA256: digest[:]}}}
	objects := textObjects{"original": []byte(text)}
	artifacts := &textArtifacts{sets: map[string]domain.ArtifactSet{}}
	engine := &textEngine{ImportProcessing: adapters.ImportProcessing{WorkDir: t.TempDir()}}
	pipeline := worker.Pipeline{Sources: sources, Artifacts: artifacts, Store: objects, Engine: engine, WorkDir: t.TempDir()}
	run := domain.Run{ID: "run", ImportID: "import", PipelineVersion: worker.PipelineVersion, SourceRevision: 1}
	var first domain.Outcome
	for i := range 2 {
		out, err := pipeline.Process(context.Background(), run, func(string) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = out
		} else if !bytes.Equal(first.Result, out.Result) || !bytes.Equal(first.Draft, out.Draft) {
			t.Fatal("replay changed the candidate")
		}
	}
	if engine.normalizations != 0 || engine.extractions != 1 || len(artifacts.sets) != 2 {
		t.Fatalf("stages normalize=%d extract=%d sets=%d", engine.normalizations, engine.extractions, len(artifacts.sets))
	}
	var draft domain.Draft
	if err := json.Unmarshal(first.Draft, &draft); err != nil {
		t.Fatal(err)
	}
	noQuestions := false
	for _, n := range domain.Assess(draft).Findings {
		if n.Code == "NO_QUESTIONS" {
			noQuestions = true
		}
	}
	if !noQuestions {
		t.Fatal("plain non-question text did not reach review")
	}
	calls := sources.calls
	run.PipelineVersion = "word-pipeline-v2"
	_, err := pipeline.Process(context.Background(), run, func(string) error { return nil })
	var failure worker.Failure
	if !errors.As(err, &failure) || failure.Code != "PROCESSOR_OUTPUT_INVALID" {
		t.Fatalf("old pipeline accepted: %v", err)
	}
	if sources.calls != calls {
		t.Fatal("old worker read sources before refusing new pipeline")
	}
}
