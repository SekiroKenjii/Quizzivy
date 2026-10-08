package adapters_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

type pastedCountCase struct {
	Name       string `json:"name"`
	Text       string `json:"text"`
	Sections   int    `json:"sections"`
	Questions  int    `json:"questions"`
	Answered   int    `json:"answered"`
	Missing    []int  `json:"missing"`
	Duplicates []int  `json:"duplicates,omitempty"`
}

func TestPastedTextCountsUseRealTextEvidence(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture source path missing")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../api/testdata/pasted-text-counts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []pastedCountCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatal("shared grammar corpus unexpectedly small")
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			evidence, err := adapters.TextEvidence(c.Text, "shared-count-exam", "exam")
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range evidence.Blocks {
				if !norm.NFC.IsNormalString(b.Text) {
					t.Fatal("TextEvidence did not normalize NFC")
				}
			}
			d, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{evidence}, domain.RecognitionProfile{})
			if err != nil {
				t.Fatal(err)
			}
			review := domain.Assess(d)
			if review.Summary.Sections != c.Sections || review.Summary.Questions != c.Questions || review.Summary.AnswersKnown != c.Answered || review.Summary.AnswersMissing != len(c.Missing) {
				t.Fatalf("actual summary %+v; expected sections=%d questions=%d answered=%d missing=%v", review.Summary, c.Sections, c.Questions, c.Answered, c.Missing)
			}
			byID := map[string]*domain.DraftQuestion{}
			for _, q := range d.Questions() {
				byID[q.ID] = q
			}
			missing := map[string]bool{}
			for _, f := range review.Findings {
				if f.Code == domain.CodeMissingAnswer {
					if byID[f.Target] == nil {
						t.Fatal("missing finding has no actual question")
					}
					missing[f.Target] = true
				}
			}
			got := []int{}
			for _, q := range d.Questions() {
				if q.Type != "short_answer" && (q.Answer.State == domain.AnswerUnknown) != missing[q.ID] {
					t.Fatal("missing-answer finding disagrees with actual answer state")
				}
				if q.Answer.State == domain.AnswerUnknown {
					number, err := strconv.Atoi(q.Label)
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, number)
				}
			}
			if !slices.Equal(got, c.Missing) {
				t.Fatalf("exact missing identities: got%v expected%v", got, c.Missing)
			}
			if c.Name == "nfd-section" && !strings.Contains(d.Sections[0].Title, "Phần") {
				t.Fatalf("NFD heading lost after real NFC: %q", d.Sections[0].Title)
			}
			if c.Name == "deck-example" {
				found := false
				for _, f := range review.Findings {
					if f.Code == domain.CodeUnmatchedKey {
						found = true
					}
				}
				if !found {
					t.Fatal("example's nonexistent key targets were hidden")
				}
			}
		})
	}
}
