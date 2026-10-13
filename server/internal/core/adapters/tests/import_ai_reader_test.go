package adapters_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync/atomic"
	"testing"

	"quizzivy/internal/core/adapters"
	aicommand "quizzivy/internal/modules/ai/application/command"
	aiports "quizzivy/internal/modules/ai/application/ports"
	aidomain "quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/reader"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/pdftext/tests/scanpdf"
)

type reserveFunc func(context.Context, aicommand.Reserve) (aidomain.Reservation, error)

func (f reserveFunc) Handle(ctx context.Context, c aicommand.Reserve) (aidomain.Reservation, error) {
	return f(ctx, c)
}

type finishFunc func(context.Context, aicommand.Finish) (aidomain.Settled, error)

func (f finishFunc) Handle(ctx context.Context, c aicommand.Finish) (aidomain.Settled, error) {
	return f(ctx, c)
}

type completeFunc func(context.Context, aiports.Request) (aiports.Response, error)

func (f completeFunc) Complete(ctx context.Context, r aiports.Request) (aiports.Response, error) {
	return f(ctx, r)
}

type scanWorld struct {
	engine adapters.ImportProcessing
	calls  atomic.Int64
	holds  atomic.Int64
}

func newScanWorld(t *testing.T, complete func(call int64) (aiports.Response, error)) *scanWorld {
	t.Helper()
	w := &scanWorld{}
	w.engine = adapters.ImportProcessing{
		PDF:     &pdfReader,
		WorkDir: t.TempDir(),
		AI: &reader.Reader{
			Pages: adapters.PDFPages{PDF: &pdfReader},
			Reserve: reserveFunc(func(_ context.Context, c aicommand.Reserve) (aidomain.Reservation, error) {
				w.holds.Add(1)
				return aidomain.Reservation{ID: "hold", UserID: c.Actor.ID, Feature: c.Feature, Credits: c.Ceiling}, nil
			}),
			Finish: finishFunc(func(context.Context, aicommand.Finish) (aidomain.Settled, error) {
				return aidomain.Settled{Charged: 32}, nil
			}),
			Completer: completeFunc(func(_ context.Context, _ aiports.Request) (aiports.Response, error) {
				return complete(w.calls.Add(1))
			}),
		},
	}
	return w
}

func page(lines ...string) (aiports.Response, error) {
	object, _ := json.Marshal(map[string]any{"readable": true, "lines": lines})
	return aiports.Response{Object: object, Usage: aiports.Usage{InputTokens: 2000, OutputTokens: 1200}}, nil
}

func extractWith(t *testing.T, engine adapters.ImportProcessing, data []byte, readWithAi bool) (ports.StageOutput, error) {
	t.Helper()
	return engine.Extract(context.Background(), ports.DocumentInput{OriginalID: "original", Identity: "original", Role: "exam", Format: "pdf", Body: bytes.NewReader(data), Bytes: int64(len(data)), ReadWithAi: readWithAi, RequestedBy: "teacher-1"})
}

func evidenceOf(t *testing.T, out ports.StageOutput) (domain.EvidenceDocument, map[string]any) {
	t.Helper()
	t.Cleanup(func() { _ = out.Close() })
	var manifest domain.ExtractionManifest
	if err := json.Unmarshal(out.Plan.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	read := func(name string, into any) {
		file, err := out.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatal(err)
		}
	}
	var evidence domain.EvidenceDocument
	read(manifest.EvidenceFile, &evidence)
	var inventory map[string]any
	read(manifest.InventoryFile, &inventory)
	return evidence, inventory
}

func TestAScanIsReadByAIOnlyWhenTheImportAskedAndTheAssistantIsOn(t *testing.T) {
	scan := scanpdf.Build(2)
	w := newScanWorld(t, func(int64) (aiports.Response, error) {
		return page("1. Which one is a fruit?", "A. apple\tB. chair\tC. table\tD. door")
	})
	for name, c := range map[string]struct {
		engine adapters.ImportProcessing
		ask    bool
	}{
		"the switch off":  {w.engine, false},
		"no assistant":    {adapters.ImportProcessing{PDF: &pdfReader, WorkDir: t.TempDir()}, true},
		"neither of them": {adapters.ImportProcessing{PDF: &pdfReader, WorkDir: t.TempDir()}, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractWith(t, c.engine, scan, c.ask)
			var failure worker.Failure
			if !errors.As(err, &failure) || failure.Code != "PDF_NO_TEXT" || failure.Retryable {
				t.Fatalf("err = %v, want terminal PDF_NO_TEXT", err)
			}
		})
	}
	if w.calls.Load() != 0 || w.holds.Load() != 0 {
		t.Fatalf("%d provider calls and %d holds for a file nobody asked to read with AI", w.calls.Load(), w.holds.Load())
	}
}

func TestAScanReadByAIIsStagedAsPDFEvidenceWithItsFindings(t *testing.T) {
	w := newScanWorld(t, func(call int64) (aiports.Response, error) {
		return page(fmt.Sprintf("%d. Which one is a fruit?", call), fmt.Sprintf("A. apple %d\tB. chair\tC. table\tD. door", call), "Trang 1")
	})
	out, err := extractWith(t, w.engine, scanpdf.Build(2), true)
	if err != nil {
		t.Fatal(err)
	}
	evidence, inventory := evidenceOf(t, out)
	if evidence.Version != pdftext.Version || out.Plan.ComponentVersion != w.engine.ExtractionVersion("pdf") {
		t.Fatalf("evidence version %q, component %q", evidence.Version, out.Plan.ComponentVersion)
	}
	if inventory["readWith"] != "ai" || inventory["pages"] != float64(2) {
		t.Fatalf("inventory = %v", inventory)
	}
	var found []string
	for _, f := range evidence.Findings {
		found = append(found, f.Code)
	}
	if !slices.Equal(found, []string{"PDF_MARKS_UNAVAILABLE", "AI_PAGE_READ", "AI_PAGE_READ"}) {
		t.Fatalf("findings = %q", found)
	}
	if len(evidence.Blocks) != 6 || evidence.Blocks[0].ID != "p1-l1" || evidence.Blocks[3].ID != "p2-l1" {
		t.Fatalf("blocks = %+v", evidence.Blocks)
	}
	if w.holds.Load() != 1 {
		t.Fatalf("%d holds, want one for the whole file", w.holds.Load())
	}
}

func TestATextLayerPDFIsReadAsTodayWhateverTheSwitchSays(t *testing.T) {
	w := newScanWorld(t, func(int64) (aiports.Response, error) { return page("never asked") })
	data := pdfOf([][]string{{"TEST 1", "1. Choose the odd one out"}, {"2. Choose the odd one out"}}, false)
	var texts [2][]string
	for i, ask := range []bool{false, true} {
		out, err := extractWith(t, w.engine, data, ask)
		if err != nil {
			t.Fatal(err)
		}
		evidence, inventory := evidenceOf(t, out)
		if _, tagged := inventory["readWith"]; tagged {
			t.Fatalf("a text layer was tagged as read with AI: %v", inventory)
		}
		for _, b := range evidence.Blocks {
			texts[i] = append(texts[i], b.ID+" "+b.Text)
		}
	}
	if !slices.Equal(texts[0], texts[1]) || len(texts[0]) != 3 {
		t.Fatalf("with the switch off %q, on %q", texts[0], texts[1])
	}
	if w.calls.Load() != 0 || w.holds.Load() != 0 {
		t.Fatalf("%d provider calls and %d holds for a file that has a text layer", w.calls.Load(), w.holds.Load())
	}
}

func TestAScanReadByAIFailsWithWhatTheTeacherCanDoAboutIt(t *testing.T) {
	unreadable := func(int64) (aiports.Response, error) {
		return aiports.Response{Object: json.RawMessage(`{"readable":false,"lines":[]}`), Usage: aiports.Usage{InputTokens: 10, OutputTokens: 5}}, nil
	}
	down := func(int64) (aiports.Response, error) {
		return aiports.Response{}, fmt.Errorf("%w: 529", aiports.ErrUnavailable)
	}
	cases := []struct {
		name  string
		pages int
		reply func(int64) (aiports.Response, error)
		hold  error
		code  string
	}{
		{"more than forty pages", 41, unreadable, nil, "PDF_TOO_LONG_FOR_AI"},
		{"no page the model could read", 2, unreadable, nil, "AI_UNREADABLE"},
		{"the provider is down", 2, down, nil, "PROVIDER_UNAVAILABLE"},
		{"the allowance is spent", 2, unreadable, &aidomain.Exhausted{Scope: aidomain.ScopeTeacher}, "AI_CREDITS_EXHAUSTED"},
		{"the pool is spent", 2, unreadable, &aidomain.Exhausted{Scope: aidomain.ScopePool}, "AI_CREDITS_EXHAUSTED"},
		{"the teacher is paused", 2, unreadable, aidomain.ErrPaused, "AI_PAUSED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newScanWorld(t, c.reply)
			if c.hold != nil {
				w.engine.AI.Reserve = reserveFunc(func(context.Context, aicommand.Reserve) (aidomain.Reservation, error) {
					return aidomain.Reservation{}, c.hold
				})
			}
			_, err := extractWith(t, w.engine, scanpdf.Build(c.pages), true)
			var failure worker.Failure
			if !errors.As(err, &failure) || failure.Code != c.code || failure.Retryable {
				t.Fatalf("err = %v, want terminal %s", err, c.code)
			}
		})
	}
}
