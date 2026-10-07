//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/shared/access"
)

func TestTextPipelineDeckSampleHasExactReviewCounts(t *testing.T) {
	h := newTextPurpose(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture source path missing")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../api/testdata/pasted-text-counts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	var sample string
	for _, c := range cases {
		if c.Name == "deck-sample" {
			sample = c.Text
		}
	}
	if sample == "" {
		t.Fatal("pinned deck sample missing")
	}
	parent := h.create(t, "Task56 exact deck sample")
	paste, err := h.app.Commands.Paste.Handle(ctx, command.Paste{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: parent.Revision, Actor: h.actor, Text: sample})
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), ExpectedRevision: paste.Import.Revision, SourceRevision: paste.Import.SourceRevision, Actor: h.actor, PipelineVersion: worker.PipelineVersion, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	run, err := h.repo.Claim(ctx, domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, Lease: time.Minute, GlobalLimit: 2, ActorLimit: 1})
	if err != nil || run.ID != scheduled.ID {
		t.Fatalf("claim exact scheduled run: %+v %v", run, err)
	}
	pipeline := worker.Pipeline{Sources: h.repo, Artifacts: h.repo, Store: h.store, Engine: adapters.ImportProcessing{WorkDir: t.TempDir()}, WorkDir: t.TempDir(), Quotas: domain.ArtifactQuotas{ActorBytes: 1 << 30, GlobalBytes: 2 << 30, SetsPerImport: 100}}
	out, err := pipeline.Process(ctx, run, func(stage string) error { return h.repo.Progress(ctx, run.Claim(), stage) })
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Complete(ctx, run.Claim(), out); err != nil {
		t.Fatal(err)
	}
	persisted, err := h.app.Queries.Review.Handle(ctx, query.Review{ImportID: parent.ID, Scope: h.actor.Scope})
	if err != nil {
		t.Fatal(err)
	}
	summary := persisted.Review.Summary
	if summary.Sections != 3 || summary.Questions != 10 || summary.AnswersKnown != 8 || summary.AnswersMissing != 2 {
		t.Fatalf("persisted exact deck summary: %+v", summary)
	}
	missing := []int{}
	for _, q := range persisted.Draft.Draft.Questions() {
		if q.Answer.State == domain.AnswerUnknown {
			number, err := strconv.Atoi(q.Label)
			if err != nil {
				t.Fatal(err)
			}
			missing = append(missing, number)
		}
	}
	if !slices.Equal(missing, []int{4, 7}) {
		t.Fatalf("persisted missing question identities: %v", missing)
	}
	var emitted domain.Draft
	if err := json.Unmarshal(out.Draft, &emitted); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(persisted.Draft.Draft)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(emitted)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Fatal("stored review changed emitted candidate draft")
	}
	if recognition.Version != "rules-v3" || worker.PipelineVersion != "word-pipeline-v3" {
		t.Fatal("recognition/pipeline identity changed")
	}
	if _, err := h.app.Queries.Review.Handle(ctx, query.Review{ImportID: parent.ID, Scope: access.Scope{UserID: uuid.NewString()}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign review: %v", err)
	}
}
