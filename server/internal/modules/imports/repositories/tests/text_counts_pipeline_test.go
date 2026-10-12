//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	if !textCandidateJSONEqual(t, actual, expected) {
		path, category := textCandidateDifference(textCandidateJSONTree(t, actual), textCandidateJSONTree(t, expected), "$")
		t.Fatalf("stored review changed emitted candidate draft at %s (%s)", path, category)
	}
	if recognition.Version != "rules-v3" || worker.PipelineVersion != "word-pipeline-v3" {
		t.Fatal("recognition/pipeline identity changed")
	}
	if _, err := h.app.Queries.Review.Handle(ctx, query.Review{ImportID: parent.ID, Scope: access.Scope{UserID: uuid.NewString()}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign review: %v", err)
	}
}

func TestTextCandidateJSONEqualityIgnoresOnlyObjectOrder(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture source path missing")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../api/testdata/pasted-text-counts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string
		Text string
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
	evidence, err := adapters.TextEvidence(sample, "ordering-regression", "exam")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{evidence}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	emitted, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := json.Marshal(textCandidateJSONTree(t, emitted))
	if err != nil {
		t.Fatal(err)
	}
	var stored domain.Draft
	if err := json.Unmarshal(reordered, &stored); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(actual, emitted) {
		t.Fatal("real recognized draft did not exercise nested RawMessage object order")
	}
	if !textCandidateJSONEqual(t, actual, emitted) {
		t.Fatal("object member order changed the full recognized draft equality")
	}
}

func TestTextCandidateJSONEqualityRetainsEveryValue(t *testing.T) {
	cases := []struct {
		name     string
		actual   string
		expected string
		path     string
		category string
	}{
		{"question-id", "{\"id\":\"q1\"}", "{\"id\":\"q2\"}", "$.id", "value"},
		{"answer-id", "{\"answer\":{\"optionIds\":[\"a\"]}}", "{\"answer\":{\"optionIds\":[\"b\"]}}", "$.answer.optionIds[0]", "value"},
		{"text", "{\"prompt\":{\"text\":\"original\"}}", "{\"prompt\":{\"text\":\"changed\"}}", "$.prompt.text", "value"},
		{"option-content", "{\"options\":[{\"content\":{\"text\":\"A\"}}]}", "{\"options\":[{\"content\":{\"text\":\"B\"}}]}", "$.options[0].content.text", "value"},
		{"array-order", "{\"options\":[\"a\",\"b\"]}", "{\"options\":[\"b\",\"a\"]}", "$.options[0]", "value"},
		{"array-length", "{\"options\":[\"a\"]}", "{\"options\":[\"a\",\"b\"]}", "$.options", "array-length"},
		{"null-array", "{\"blanks\":null}", "{\"blanks\":[]}", "$.blanks", "type"},
		{"null-object", "{\"prompt\":null}", "{\"prompt\":{}}", "$.prompt", "type"},
		{"missing-key", "{}", "{\"answer\":null}", "$.answer", "missing-key"},
		{"exact-large-number", "{\"n\":9007199254740992}", "{\"n\":9007199254740993}", "$.n", "value"},
		{"numeric-token", "{\"n\":1}", "{\"n\":1.0}", "$.n", "value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if textCandidateJSONEqual(t, []byte(c.actual), []byte(c.expected)) {
				t.Fatal("changed JSON value was accepted")
			}
			path, category := textCandidateDifference(textCandidateJSONTree(t, []byte(c.actual)), textCandidateJSONTree(t, []byte(c.expected)), "$")
			if path != c.path || category != c.category {
				t.Fatalf("difference location: %s (%s), expected %s (%s)", path, category, c.path, c.category)
			}
		})
	}
}

func textCandidateJSONEqual(t *testing.T, actual, expected []byte) bool {
	t.Helper()
	return reflect.DeepEqual(textCandidateJSONTree(t, actual), textCandidateJSONTree(t, expected))
}

func textCandidateJSONTree(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var tree any
	if err := decoder.Decode(&tree); err != nil {
		t.Fatal(err)
	}
	return tree
}

func textCandidateDifference(actual, expected any, path string) (string, string) {
	if reflect.DeepEqual(actual, expected) {
		return "", ""
	}
	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		return path, "type"
	}
	switch a := actual.(type) {
	case map[string]any:
		e := expected.(map[string]any)
		keys := make([]string, 0, len(a)+len(e))
		for key := range a {
			keys = append(keys, key)
		}
		for key := range e {
			if _, ok := a[key]; !ok {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		for _, key := range keys {
			av, aok := a[key]
			ev, eok := e[key]
			child := path + "." + key
			if aok != eok {
				return child, "missing-key"
			}
			if !reflect.DeepEqual(av, ev) {
				return textCandidateDifference(av, ev, child)
			}
		}
	case []any:
		e := expected.([]any)
		if len(a) != len(e) {
			return path, "array-length"
		}
		for i := range a {
			if !reflect.DeepEqual(a[i], e[i]) {
				return textCandidateDifference(a[i], e[i], path+"["+strconv.Itoa(i)+"]")
			}
		}
	}
	return path, "value"
}
