package recognition_test

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"strings"
	"testing"
)

// The fixtures below are written for these tests; no real paper is copied into the repo.

func tfngPaper(instruction string) domain.EvidenceDocument {
	return exam(lines(
		"I. "+instruction,
		"1. The museum opens every day.",
		"2. Tickets are free for children.",
		"3. The café closes at six.",
		"4. Guides speak French.",
	))
}

func TestEveryFormOfATFNGInstructionGivesTFNGQuestions(t *testing.T) {
	for _, instruction := range []string{
		"Do the statements agree with the passage? Write TRUE, FALSE or NOT GIVEN.",
		"Write TRUE if the statement agrees with the information, FALSE if it contradicts it, NOT GIVEN if there is no information",
		"Write T / F / NG for each statement.",
		"Write T, F or NG.",
		"Chọn đúng / sai / không có thông tin.",
	} {
		t.Run(instruction, func(t *testing.T) {
			d := recognize(t, tfngPaper(instruction), key(lines("I.\t1. TRUE\t2. NOT GIVEN\t3. F\t4. NG")))
			for label, want := range map[string]string{"1": "T", "2": "NG", "3": "F", "4": "NG"} {
				q := question(t, d, label)
				if q.Type != "tfng" || q.Answer.State != domain.AnswerKnown || !slices.Equal(answerLabels(q), []string{want}) {
					t.Fatalf("question %s: %+v", label, q)
				}
				if len(q.Options) != 3 || q.Options[0].Label != "T" || q.Options[1].Label != "F" || q.Options[2].Label != "NG" || text(t, q.Options[2].Content) != "Not given" {
					t.Fatalf("tfng options are the bank's fixed three: %+v", q.Options)
				}
			}
			if review := domain.Assess(d); review.Summary.Blocking != 0 {
				t.Fatalf("a keyed tfng paper is ready: %+v", review.Findings)
			}
		})
	}
}

func TestATFNGKeyOutsideTheThreeValuesIsAFinding(t *testing.T) {
	d := recognize(t, tfngPaper("Write TRUE, FALSE or NOT GIVEN."), key(lines("I.\t1. TRUE\t2. YES\t3. F\t4. NG")))
	q := question(t, d, "2")
	if q.Type != "tfng" || q.Answer.State != domain.AnswerConflict || len(q.Answer.OptionIDs) != 0 {
		t.Fatalf("an unreadable tfng key stays unset: %+v", q)
	}
	f := finding(t, domain.Assess(d), domain.CodeUnusableKey, q.ID)
	if f.Severity != domain.Blocking || f.Field != "answer" {
		t.Fatalf("unusable tfng key: %+v", f)
	}
}

func TestAPaperWithoutATFNGInstructionKeepsItsTypes(t *testing.T) {
	d := recognize(t, tfngPaper("Read the passage and decide whether the sentences are True or False."), key(lines("I.\t1. T\t2. F\t3. T\t4. F")))
	for _, label := range []string{"1", "2", "3", "4"} {
		if q := question(t, d, label); q.Type != "true_false" {
			t.Fatalf("a True or False part stays true_false: %+v", q)
		}
	}
}

func TestASectionKeyedTrueFalseAndNotGivenIsTFNG(t *testing.T) {
	d := recognize(t, tfngPaper("Read the passage about the museum."), key(lines("I.\t1. TRUE\t2. NOT GIVEN\t3. FALSE\t4. TRUE")))
	for label, want := range map[string]string{"1": "T", "2": "NG", "3": "F", "4": "T"} {
		if q := question(t, d, label); q.Type != "tfng" || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s keyed from the answer key: %+v", label, q)
		}
	}
}

func TestPastedTextTFNGPartMapsItsAnswerLines(t *testing.T) {
	d := recognize(t, textExam(
		"Part 1. True, False or Not given.",
		"1. Parks are cheaper than roads.",
		"Answer: Not given",
		"2. Trees cool streets.",
		"Answer: True",
		"3. Homes near parks sell for less.",
	))
	for label, want := range map[string][]string{"1": {"NG"}, "2": {"T"}, "3": nil} {
		q := question(t, d, label)
		if q.Type != "tfng" || !slices.Equal(answerLabels(q), want) {
			t.Fatalf("pasted question %s: %+v", label, q)
		}
	}
	if q := question(t, d, "3"); q.Answer.State != domain.AnswerUnknown {
		t.Fatalf("an unkeyed tfng question waits for the teacher: %+v", q)
	}
}

func choosePaper(instruction string) domain.EvidenceDocument {
	return exam(lines(
		"I. "+instruction,
		"1. Which two activities does the club offer?",
		"A. hiking\tB. painting\tC. chess\tD. cooking\tE. rowing",
	))
}

func TestChooseNOnAMultipleChoiceGivesSelectCount(t *testing.T) {
	for instruction, want := range map[string]int{
		"Choose TWO letters, A-E.":             2,
		"Choose 2 letters.":                    2,
		"Choose THREE correct answers.":        3,
		"Choose two answers from the list.":    2,
		"Choose ONE letter for each question.": 0,
	} {
		t.Run(instruction, func(t *testing.T) {
			keyed := "1. B, D"
			if want == 3 {
				keyed = "1. B, C, D"
			}
			if want == 0 {
				keyed = "1. B"
			}
			d := recognize(t, choosePaper(instruction), key(lines(keyed)))
			q := question(t, d, "1")
			if want == 0 {
				if q.Type != "single_choice" || q.SelectCount != nil {
					t.Fatalf("Choose ONE is a single choice: %+v", q)
				}
				return
			}
			if q.Type != "multiple_choice" || q.SelectCount == nil || *q.SelectCount != want || q.Answer.State != domain.AnswerKnown {
				t.Fatalf("Choose %d: %+v", want, q)
			}
			if review := domain.Assess(d); review.Summary.Blocking != 0 {
				t.Fatalf("a matching key is ready: %+v", review.Findings)
			}
		})
	}
}

func TestAChooseNKeyWithAnotherNumberOfLettersIsAFinding(t *testing.T) {
	d := recognize(t, choosePaper("Choose TWO letters, A-E."), key(lines("1. B")))
	q := question(t, d, "1")
	if q.Type != "multiple_choice" || q.SelectCount == nil || *q.SelectCount != 2 || !slices.Equal(answerLabels(q), []string{"B"}) {
		t.Fatalf("the key is kept for the teacher to fix: %+v", q)
	}
	f := finding(t, domain.Assess(d), domain.CodeInvalidQuestion, q.ID)
	if f.Severity != domain.Blocking || f.Field != "selectCount" {
		t.Fatalf("miscounted Choose N key: %+v", f)
	}
}

func TestAWordLimitOnAShortAnswerGivesMaxWords(t *testing.T) {
	for instruction, want := range map[string]int{
		"Answer the questions. Write NO MORE THAN TWO WORDS for each answer.": 2,
		"Answer in NO MORE THAN 3 WORDS AND/OR A NUMBER.":                     3,
		"Answer the questions with ONE WORD ONLY.":                            1,
		"Answer the questions about the passage.":                             0,
	} {
		t.Run(instruction, func(t *testing.T) {
			d := recognize(t, exam(lines("I. "+instruction, "1. Where does the guide live?")), key(lines("1. in Hue")))
			q := question(t, d, "1")
			if q.Type != "short_answer" {
				t.Fatalf("short answer: %+v", q)
			}
			switch {
			case want == 0 && q.MaxWords != nil:
				t.Fatalf("no limit was named: %+v", q)
			case want > 0 && (q.MaxWords == nil || *q.MaxWords != want):
				t.Fatalf("limit %d: %+v", want, q)
			}
		})
	}
}

func TestAWordLimitOnAFillInStaysInstructionText(t *testing.T) {
	instruction := "Complete the notes. Write NO MORE THAN TWO WORDS for each answer."
	d := recognize(t, exam(lines("I. "+instruction, "1. The tour starts at the ……….", "2. Visitors should bring ……….")), key(lines("1. main gate", "2. warm clothes")))
	for _, label := range []string{"1", "2"} {
		if q := question(t, d, label); q.Type != "fill_blank" || q.MaxWords != nil {
			t.Fatalf("fill-in %s keeps no word limit: %+v", label, q)
		}
	}
	if s := d.Sections[0]; !strings.Contains(s.Title+s.Instructions, "NO MORE THAN TWO WORDS") {
		t.Fatalf("the limit stays in the instruction: %+v", s)
	}
}

func TestMatchingStaysUnsupported(t *testing.T) {
	d := recognize(t, exam(lines(
		"I. Match each person with the correct statement, A-E.",
		"1. Lan",
		"2. Minh",
	)), key(lines("1. C", "2. A")))
	for _, label := range []string{"1", "2"} {
		q := question(t, d, label)
		if q.Type != domain.UnsupportedType {
			t.Fatalf("matching %s: %+v", label, q)
		}
		if f := finding(t, domain.Assess(d), domain.CodeUnsupportedInteraction, q.ID); f.Severity != domain.Blocking {
			t.Fatalf("matching must be excluded: %+v", f)
		}
	}
}

func TestPlanCarriesTheNewFieldsToTheBank(t *testing.T) {
	d := recognize(t,
		exam(lines(
			"I. Write TRUE, FALSE or NOT GIVEN.",
			"1. The museum opens every day.",
			"II. Choose TWO letters, A-E.",
			"2. Which two activities does the club offer?",
			"A. hiking\tB. painting\tC. chess\tD. cooking\tE. rowing",
			"III. Write NO MORE THAN TWO WORDS for each answer.",
			"3. Where does the guide live?",
		)),
		key(lines("I.\t1. NG", "II.\t2. B, D", "III.\t3. in Hue")),
	)
	for _, label := range []string{"1", "2", "3"} {
		q := question(t, d, label)
		in, err := q.Input()
		if err != nil {
			t.Fatal(err)
		}
		if err := in.Validate(nil); err != nil {
			t.Fatalf("question %s does not validate for the bank: %v", label, err)
		}
		switch label {
		case "1":
			if string(in.Type) != "tfng" || len(in.Options) != 3 || !in.Options[2].IsCorrect || in.Options[0].IsCorrect || in.Options[1].IsCorrect {
				t.Fatalf("tfng input: %+v", in)
			}
		case "2":
			if in.SelectCount == nil || *in.SelectCount != 2 {
				t.Fatalf("selectCount input: %+v", in)
			}
		case "3":
			if in.MaxWords == nil || *in.MaxWords != 2 {
				t.Fatalf("maxWords input: %+v", in)
			}
		}
	}
}

func finding(t *testing.T, r domain.Review, code, target string) domain.Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Code == code && f.Target == target {
			return f
		}
	}
	t.Fatalf("no %s finding on %s among %+v", code, target, r.Findings)
	return domain.Finding{}
}
