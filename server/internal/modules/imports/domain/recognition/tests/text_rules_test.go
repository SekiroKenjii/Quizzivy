package recognition_test

import (
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"slices"
	"strings"
	"testing"
)

func textExam(linesOfText ...string) domain.EvidenceDocument {
	d := exam(lines(linesOfText...))
	d.Version = "text-lines-v1"
	return d
}

func TestTextOptionStarsPreserveContentAndAggregateAnswers(t *testing.T) {
	d := recognize(t, textExam("1. The * stem * remains.", "a. interior*star", "b. * beta", "c. gamma", "d. delta *"))
	q := question(t, d, "1")
	if q.Type != "multiple_choice" || q.Answer.State != domain.AnswerKnown || !slices.Equal(answerLabels(q), []string{"B", "D"}) {
		t.Fatalf("marked choices: %+v", q)
	}
	if text(t, q.Prompt) != "The * stem * remains." || text(t, q.Options[0].Content) != "interior*star" || text(t, q.Options[1].Content) != "beta" || text(t, q.Options[3].Content) != "delta" {
		t.Fatalf("star removal changed literal content: %+v", q)
	}
	if len(notices(d, domain.CodeUnassignedText)) != 0 || len(q.Answer.Evidence) != 2 {
		t.Fatalf("star source coverage/evidence: %+v %+v", d.Notices, q.Answer.Evidence)
	}
}

func TestTextAnswerLineSpellingsAndInlineAnswer(t *testing.T) {
	for _, key := range []string{"Answer: B", "Ans: B", "Answer - B", "Đáp án: B"} {
		for _, inline := range []bool{false, true} {
			t.Run(key+strings.Repeat("inline", map[bool]int{false: 0, true: 1}[inline]), func(t *testing.T) {
				source := []string{"1. First?", "A. alpha", "B. beta", "C. gamma"}
				if inline {
					source[0] += " " + key
				} else {
					source = append(source, key)
				}
				q := question(t, recognize(t, textExam(source...)), "1")
				if q.Answer.State != domain.AnswerKnown || !slices.Equal(answerLabels(q), []string{"B"}) || strings.Contains(text(t, q.Prompt), key) {
					t.Fatalf("answer spelling/learner prose: %+v", q)
				}
			})
		}
	}
}

func TestTextLabelsAndUnderscoreGapsHaveMatchingRichNodes(t *testing.T) {
	d := recognize(t, textExam("1: We live ___ here.", "Answer: since", "2. Word ...", "Answer: north", "3. Word …", "Answer: south", "4. Word ....", "Answer: east", "5. Word ……", "Answer: west"))
	if !slices.Equal(labels(d), []string{"1", "2", "3", "4", "5"}) {
		t.Fatalf("colon labels: %v", labels(d))
	}
	for _, label := range []string{"1", "4", "5"} {
		q := question(t, d, label)
		if q.Type != "fill_blank" || len(q.Blanks) != 1 || !strings.Contains(string(q.Prompt), `"type":"gap"`) || len(q.Blanks[0].Accepted) != 1 {
			t.Fatalf("rich gap %s: %+v", label, q)
		}
	}
	for _, label := range []string{"2", "3"} {
		q := question(t, d, label)
		if q.Type != "short_answer" || len(q.Blanks) != 0 || strings.Contains(string(q.Prompt), `"type":"gap"`) {
			t.Fatalf("old dots/ellipsis threshold %s: %+v", label, q)
		}
	}
}

func TestTextWritingAnswerLinesUseTheSameGapMode(t *testing.T) {
	d := recognize(t, textExam("Part 1. Write complete sentences.", "1. We live ___", "Answer: We live in Hanoi."))
	q := question(t, d, "1")
	if q.Type != "short_answer" || q.Answer.Text != "We live in Hanoi." || text(t, q.Prompt) != "We live" {
		t.Fatalf("trailing answer line mode: %+v", q)
	}
}

func TestNotGivenRemainsShortAnswer(t *testing.T) {
	d := recognize(t, textExam("Part 1. Decide True or False or Not given.", "1. Parks are cheaper than roads.", "Answer: Not given"))
	q := question(t, d, "1")
	if q.Type != "short_answer" || q.Answer.State != domain.AnswerKnown || q.Answer.Text != "Not given" {
		t.Fatalf("Not given is teacher-reviewed short answer: %+v", q)
	}
}

func TestTextRulesAreGatedByExamEvidenceVersion(t *testing.T) {
	for _, version := range []string{"ooxml-blocks-v1", "pdf-lines-v1"} {
		for _, vector := range []struct {
			name   string
			source []string
			check  func(*testing.T, domain.Draft)
		}{
			{"star", []string{"1. Stem.", "A. alpha", "B. beta *", "C. gamma"}, func(t *testing.T, d domain.Draft) {
				q := question(t, d, "1")
				if q.Answer.State != domain.AnswerUnknown || text(t, q.Options[1].Content) != "beta *" {
					t.Fatalf("file star broadened: %+v", q)
				}
			}},
			{"lower", []string{"1. Stem.", "a. alpha", "b. beta", "c. gamma"}, func(t *testing.T, d domain.Draft) {
				if len(question(t, d, "1").Options) != 0 {
					t.Fatal("file lowercase options broadened")
				}
			}},
			{"colon", []string{"1: Stem.", "Answer: north"}, func(t *testing.T, d domain.Draft) {
				if len(d.Questions()) != 0 {
					t.Fatal("file colon label broadened")
				}
			}},
			{"underscore", []string{"1. Word ___ here.", "Answer: north"}, func(t *testing.T, d domain.Draft) {
				q := question(t, d, "1")
				if q.Type != "short_answer" || strings.Contains(string(q.Prompt), `"type":"gap"`) {
					t.Fatalf("file gap broadened: %+v", q)
				}
			}},
			{"writing", []string{"Part 1. Write complete sentences.", "1. We live ___", "Answer: We live in Hanoi."}, func(t *testing.T, d domain.Draft) {
				if !strings.Contains(text(t, question(t, d, "1").Prompt), "___") {
					t.Fatal("file answer-line trimming broadened")
				}
			}},
			{"roman", []string{"I", "1. Stem.", "II", "2. Stem."}, func(t *testing.T, d domain.Draft) {
				if len(d.Sections) != 1 {
					t.Fatal("file bare Roman broadened")
				}
			}},
			{"ans", []string{"1. Stem.", "A. alpha", "B. beta", "C. gamma", "Ans: B"}, func(t *testing.T, d domain.Draft) {
				if question(t, d, "1").Answer.State != domain.AnswerUnknown {
					t.Fatal("file Ans broadened")
				}
			}},
			{"not-given", []string{"Exam title", "Part 1. True or False or Not given.", "1. Stem.", "Answer: Not given"}, func(t *testing.T, d domain.Draft) {
				q := question(t, d, "1")
				if q.Type != "true_false" || q.Answer.State != domain.AnswerConflict {
					t.Fatalf("file Not given changed: %+v", q)
				}
			}},
			{"dash", []string{"1. Stem.", "A. alpha", "B. beta", "C. gamma", "Answer - B"}, func(t *testing.T, d domain.Draft) {
				if question(t, d, "1").Answer.State != domain.AnswerUnknown {
					t.Fatal("file Answer dash broadened")
				}
			}},
		} {
			t.Run(version+"/"+vector.name, func(t *testing.T) {
				doc := exam(lines(vector.source...))
				doc.Version = version
				companion := key(lines("Answer key: 98-A 99-B"))
				companion.Version = "text-lines-v1"
				vector.check(t, recognize(t, doc, companion))
			})
		}
	}
}

func TestTextRulesDoNotBroadenExportedPDFLabelHelpers(t *testing.T) {
	if _, ok := recognition.QuestionStart("1: Stem"); ok {
		t.Fatal("PDF question helper broadened")
	}
	if recognition.SectionStart("I") {
		t.Fatal("PDF section helper broadened")
	}
	d := recognize(t, textExam("I", "1. Stem.", "II", "2. Stem."))
	if len(d.Sections) != 2 {
		t.Fatalf("text bare Roman sections: %d", len(d.Sections))
	}
	if recognition.Version != "rules-v3" {
		t.Fatalf("recognition identity: %s", recognition.Version)
	}
}

func TestPreLabelStarsRemainSourceContent(t *testing.T) {
	d := recognize(t, textExam("1. Stem.", "*A. alpha", "B. beta"))
	q := question(t, d, "1")
	if len(q.Options) != 0 || q.Answer.State != domain.AnswerUnknown || !strings.Contains(text(t, q.Prompt), "*A. alpha") {
		t.Fatalf("pre-label star inferred grammar: %+v", q)
	}
}

func TestTextOpenClozeThreeUnderscoresKeepRichGapLinks(t *testing.T) {
	d := recognize(t, textExam("Part 1. Fill in each blank.", "We live (1)___ here and (2)___ there.", "Answer key: 1-north 2-south"))
	gs := groups(d)
	if len(gs) != 1 || len(gs[0].Gaps) != 2 || len(gs[0].Questions) != 2 {
		t.Fatalf("text shared cloze structure: %+v", gs)
	}
	g := gs[0]
	for i, q := range g.Questions {
		if q.Type != "fill_blank" || q.Answer.State != domain.AnswerKnown || len(q.Blanks) != 1 || len(q.Blanks[0].Accepted) != 1 || g.Gaps[i].QuestionID != q.ID || g.Gaps[i].BlankGapID != q.Blanks[0].GapID {
			t.Fatalf("rich gap identity %d: %+v %+v", i, g.Gaps[i], q)
		}
		if !strings.Contains(string(g.Stimulus), `"type":"gap"`) || !strings.Contains(string(g.Stimulus), g.Gaps[i].GapID) || !strings.Contains(string(q.Prompt), q.Blanks[0].GapID) {
			t.Fatalf("rich shared/question nodes differ: %s %s", g.Stimulus, q.Prompt)
		}
	}
	if !slices.Equal(g.Questions[0].Blanks[0].Accepted, []string{"north"}) || !slices.Equal(g.Questions[1].Blanks[0].Accepted, []string{"south"}) {
		t.Fatal("numbered cloze keys were reassigned")
	}
	if len(notices(d, domain.CodeUnassignedText)) != 0 {
		t.Fatalf("shared cloze source lost: %+v", d.Notices)
	}
}
