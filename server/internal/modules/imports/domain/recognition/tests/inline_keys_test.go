package recognition_test

import (
	"fmt"
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func twoChoiceQuestions() []string {
	return []string{"1. First?", "A. alpha", "B. beta", "C. gamma", "2. Second?", "A. one", "B. two", "C. three"}
}

func TestOneLineKeysApplyToEveryEvidenceVersion(t *testing.T) {
	for _, version := range []string{"ooxml-blocks-v1", "pdf-lines-v1", "text-lines-v1"} {
		for _, header := range []string{"Đáp án", "Answer key", "Answers", "Key"} {
			for _, separator := range []string{"-", ":", ".", ")"} {
				t.Run(version+"/"+header+separator, func(t *testing.T) {
					keyLine := fmt.Sprintf("%s: 1%sB 2%sC", header, separator, separator)
					source := exam(lines(append(twoChoiceQuestions(), keyLine)...))
					source.Version = version
					d := recognize(t, source)
					for i, want := range []string{"B", "C"} {
						q := question(t, d, fmt.Sprint(i+1))
						if q.Answer.State != domain.AnswerKnown || !slices.Equal(answerLabels(q), []string{want}) {
							t.Fatalf("global pair question %s: %+v labels=%v", q.Label, q.Answer, answerLabels(q))
						}
						if len(q.Answer.Evidence) == 0 {
							t.Fatal("key has no source evidence")
						}
						for _, ref := range q.Answer.Evidence {
							if ref.SourceID != source.SourceID || ref.BlockID != "exam-9" || ref.Start < 0 || ref.End > utf8.RuneCountInString(keyLine) || ref.End <= ref.Start {
								t.Fatalf("Unicode key source range: %+v", ref)
							}
							if !strings.Contains(string([]rune(keyLine)[ref.Start:ref.End]), want) {
								t.Fatalf("key evidence lost value: %+v", ref)
							}
						}
					}
					if len(notices(d, domain.CodeConflictingKeys)) != 0 || len(notices(d, domain.CodeUnassignedText)) != 0 {
						t.Fatalf("key conflict/unassigned notices: %+v", d.Notices)
					}
				})
			}
		}
	}
}

func TestOnePairKeysKeepTheirExistingMeaning(t *testing.T) {
	for _, value := range []struct {
		line  string
		known bool
	}{
		{"Đáp án: B", true}, {"Answer: B", true}, {"Key: B", true},
		{"Answer key: B", false}, {"Answers: B", false},
	} {
		t.Run(value.line, func(t *testing.T) {
			d := recognize(t, exam(lines("1. First?", "A. alpha", "B. beta", "C. gamma", value.line)))
			q := question(t, d, "1")
			if (q.Answer.State == domain.AnswerKnown) != value.known || value.known && !slices.Equal(answerLabels(q), []string{"B"}) {
				t.Fatalf("single-value fallback: %+v", q.Answer)
			}
		})
	}
}

func TestOneLineCompanionKeysPreserveConflictsAndUnmatchedEvidence(t *testing.T) {
	for _, version := range []string{"ooxml-blocks-v1", "pdf-lines-v1", "text-lines-v1"} {
		t.Run(version, func(t *testing.T) {
			source := exam(lines(append(twoChoiceQuestions(), "Answer key: 1-B 2-C")...))
			source.Version = version
			companion := key(lines("Đáp án: 1-A 3-C"))
			companion.Version = "text-lines-v1"
			d := recognize(t, source, companion)
			if question(t, d, "1").Answer.State != domain.AnswerConflict || question(t, d, "2").Answer.State != domain.AnswerKnown || len(notices(d, domain.CodeUnmatchedKey)) == 0 {
				t.Fatalf("companion conflict/unmatched reporting: %+v %+v", d.Questions(), d.Notices)
			}
			if len(d.Questions()) != 2 {
				t.Fatal("unmatched key invented a question")
			}
		})
	}
}
