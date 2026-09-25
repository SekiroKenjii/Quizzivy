package recognition_test

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"strings"
	"testing"
)

func romanPaper() domain.EvidenceDocument {
	return exam(
		lines(
			"TEST 7",
			"I. Choose the word which has the underlined part pronounced differently from the others",
			"1. A. truck\tB. unload\tC. turn\tD. lunch",
			"2. A. policeman\tB. sign\tC. bike\tD. spider",
			"II. Choose the word or phrase that best completes the sentence",
			"3. What is the ………….of that river?",
			"A. long\tB. wide\tC. length\tD. heavy",
			"4. Peter usually drives …………..Mary",
			"A. more fast\tB. fast than\tC. faster than\tD. B and C are correct",
			"III. Complete the sentences with the correct form of the verb",
			"5. We (go)…………………on holiday if there is time",
			"6. If the Earth (be)……………..warmer, the sea level (rise)………………….",
			"IV. Each of the following sentences has one mistake. Identify and correct the mistakes",
			"7. New York is an excited city with many skyscrapers     ………………………",
			"V. Read the passage and then decide whether the sentences are True or False",
			"Vietnam’s New Year is known as Tet. It begins between January and",
			"February. The exact date changes from year to year.",
			"8. Tet occurs in late January and early February     ………………….",
			"9. There are two weeks for Lunar New Year     …………………",
			"VI. Choose the correct answer to fill in the blank",
			"Television first came about 60 years ago. It is one of the most (10)………….. sources of fun and brings (11)……………….for children.",
			"10. A. cheap\tB. expensive\tC. popular\tD. exciting",
			"11. A. news\tB. cartoons\tC. sports\tD. plays",
			"VII. Fill in the blank with the given words",
			"after\tfun",
			"Liz had a lot of (12)………………..in Nha Trang. (13)……………….the trip she felt great.",
			"VIII. Finish the second sentence that it means the same as the sentence printed before",
			"14. I enjoy watching TV",
			"-> I am ………………………………………………",
		),
	)
}

func romanKey() domain.EvidenceDocument {
	return key(lines(
		"ĐÁP ÁN",
		"ĐỀ SỐ 6",
		"I.\t1.A\t2.B",
		"ĐỀ SỐ 7",
		"I.\t1.C\t\t2.A",
		"II.\t3.C\t4.D",
		"III.\t5. will go\t6. is - will rise",
		"IV.\t7.excited→exciting",
		"V.\t8. T\t\t9. F",
		"VI.\t10. C\t11. B",
		"VII.\t12. fun\t13. After",
		"VIII.\t14.I am interested in watching TV.",
		"I am fond of watching TV.",
	))
}

func TestRomanSectionsKeepContinuousNumberingAndMatchTheirPaperInAMultiPaperKey(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	if d.Title != "TEST 7" || len(d.Sections) != 8 {
		t.Fatalf("title %q sections %d", d.Title, len(d.Sections))
	}
	if got := labels(d); !slices.Equal(got, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}) {
		t.Fatalf("labels %v", got)
	}
	for _, q := range d.Questions() {
		if q.Answer.State != domain.AnswerKnown {
			t.Errorf("question %s answer %+v", q.Label, q.Answer)
		}
	}
	if got := answerLabels(question(t, d, "1")); !slices.Equal(got, []string{"C"}) {
		t.Fatalf("paper 7 key not chosen: %v", got)
	}
	if len(notices(d, domain.CodeUnassignedText)) != 0 || len(notices(d, domain.CodeKeyPaperAmbiguous)) != 0 {
		t.Fatalf("unexpected notices %+v", d.Notices)
	}
}

func TestSameLineOptionsSplitOnlyAtTheNextExpectedLetter(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	q := question(t, d, "4")
	if q.Type != "single_choice" || len(q.Options) != 4 {
		t.Fatalf("question 4: %s %d options", q.Type, len(q.Options))
	}
	if got := text(t, q.Options[3].Content); got != "B and C are correct" {
		t.Fatalf("option D %q", got)
	}
	if got := text(t, q.Prompt); got != "Peter usually drives …………..Mary" {
		t.Fatalf("prompt %q", got)
	}
}

func TestAnInstructionOnlyQuestionTakesItsSectionInstructionAsPrompt(t *testing.T) {
	d := recognize(t, romanPaper())
	got := text(t, question(t, d, "1").Prompt)
	if got != "Choose the word which has the underlined part pronounced differently from the others" {
		t.Fatalf("prompt %q", got)
	}
}

func TestGapsWithShortKeysBecomeBlanksAndMultiBlankKeysSplitInOrder(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	q := question(t, d, "5")
	if q.Type != "fill_blank" || len(q.Blanks) != 1 || !slices.Equal(q.Blanks[0].Accepted, []string{"will go"}) {
		t.Fatalf("question 5: %s %+v", q.Type, q.Blanks)
	}
	if !strings.Contains(string(q.Prompt), `"type":"gap"`) {
		t.Fatalf("prompt has no gap node: %s", q.Prompt)
	}
	two := question(t, d, "6")
	if len(two.Blanks) != 2 || two.Blanks[0].Accepted[0] != "is" || two.Blanks[1].Accepted[0] != "will rise" {
		t.Fatalf("question 6 blanks %+v", two.Blanks)
	}
}

func TestCorrectionsAndTransformationsStayTeacherGradedWithTheKeyAsSample(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	correction := question(t, d, "7")
	if correction.Type != "short_answer" || correction.Answer.Text != "excited→exciting" {
		t.Fatalf("question 7: %s %q", correction.Type, correction.Answer.Text)
	}
	if strings.Contains(text(t, correction.Prompt), "……") {
		t.Fatalf("answer line kept in prompt %q", text(t, correction.Prompt))
	}
	transform := question(t, d, "14")
	if transform.Type != "short_answer" || transform.Answer.Text != "I am interested in watching TV.\nI am fond of watching TV." {
		t.Fatalf("question 14: %s %q", transform.Type, transform.Answer.Text)
	}
}

func TestAPassageBeforeItsQuestionsBecomesASharedGroup(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	g := groups(d)
	if len(g) != 3 {
		t.Fatalf("groups %d", len(g))
	}
	reading := g[0]
	if got := text(t, reading.Stimulus); !strings.Contains(got, "January and February.") {
		t.Fatalf("soft-wrapped passage not joined: %q", got)
	}
	tf := question(t, d, "8")
	if tf.Type != "true_false" || !slices.Equal(answerLabels(tf), []string{"T"}) || len(tf.Options) != 2 {
		t.Fatalf("question 8: %s %v", tf.Type, answerLabels(tf))
	}
}

func TestNumberedPassageGapsBindToTheirChoiceQuestions(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	cloze := groups(d)[1]
	if len(cloze.Gaps) != 2 || cloze.Gaps[0].QuestionID != question(t, d, "10").ID || cloze.Gaps[0].BlankGapID != "" {
		t.Fatalf("gap links %+v", cloze.Gaps)
	}
	if got := text(t, cloze.Stimulus); !strings.Contains(got, "most [10] sources") {
		t.Fatalf("stimulus %q", got)
	}
	if got := text(t, question(t, d, "10").Prompt); got != "(10)" {
		t.Fatalf("cloze prompt %q", got)
	}
}

func TestAnOpenClozeCreatesOneBlankQuestionPerNumberedGap(t *testing.T) {
	d := recognize(t, romanPaper(), romanKey())
	open := groups(d)[2]
	if len(open.Questions) != 2 || open.Questions[0].Label != "12" || open.Questions[0].Type != "fill_blank" {
		t.Fatalf("open cloze %+v", open.Questions)
	}
	if open.Gaps[0].BlankGapID != open.Questions[0].Blanks[0].GapID || open.Questions[1].Blanks[0].Accepted[0] != "After" {
		t.Fatalf("open cloze links %+v %+v", open.Gaps, open.Questions[1].Blanks)
	}
}

func questionPaper() domain.EvidenceDocument {
	return exam(
		lines(
			"Đề số 2 ôn thi môn Tiếng Anh",
			"Question 1 Choose the word which has the underlined part pronounced differently from the others.",
		),
		[]block{p(plain("A. "), plain("b"), underlined("ea"), plain("r"))},
		lines(
			"B. teach",
			"C. meat",
			"D. sea",
			"Question 2 Choose the word which has the underlined part pronounced differently from the others.",
			"A. barbecue",
			"B. climber",
			"C. bomb",
			"D. comb",
			"Question 3 Choose the sentence that is CLOSEST in meaning to the sentence given.",
			"I will take up golf this year.",
			"A. I will begin to play golf this year.",
			"B. I will stop playing golf this year.",
			"C. I will build a golf court this year.",
			"D. I will enter a golf competition this year.",
			"Question 4 _____ I visit him, we talk about sports a lot.",
			"A. Up to",
			"B. As far as",
			"C. Whenever",
			"D. Until",
			"Question 5 Read the text and choose the best answer to fill in the blanks.",
			"Many students learn grammar very well Q5.1.................... cannot have a",
			"conversation with native speakers.",
			"Q5.2.................... will hear your mistakes.",
		),
		row("t1", 0, "Q5.1.", "A. but", "B. when", "C. so", "D. or"),
		row("t1", 1, "Q5.2.", "A. nobody", "B. everybody", "C. anybody", "D. somebody"),
		lines(
			"Read the following passage then choose the best answer to each question below.",
			"Most people in Britain do not wear formal clothes.",
			"Question 6 Who doesn’t usually wear suits and ties?",
			"A. lawyers",
			"B. drivers",
			"Question 7 Complete the sentence by changing the form of the word in capitals.",
			"My grandfather can speak English Q7.1.................... so I admire",
			"him. (FLUENCY)",
		),
	)
}

func questionKey() domain.EvidenceDocument {
	return key(lines(
		"Đáp án đề số 1 ôn thi môn Tiếng Anh",
		"Question 1. B",
		"Đáp án đề số 2 ôn thi môn Tiếng Anh",
		"Question 1. A",
		"Question 2. A",
		"Question 3. A",
		"Question 4. C",
		"Question 5.1 A",
		"5.2 A",
		"Question 6. B",
		"Question 7.1 fluently",
	))
}

func TestPerQuestionInstructionsGroupIntoInferredSections(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	var titles []string
	for _, s := range d.Sections {
		titles = append(titles, s.Title)
		if s.Origin != domain.InferredStructure {
			t.Errorf("section %q origin %s", s.Title, s.Origin)
		}
	}
	want := []string{
		"Choose the word which has the underlined part pronounced differently from the others.",
		"Choose the sentence that is CLOSEST in meaning to the sentence given.",
		"Question 4",
		"Read the text and choose the best answer to fill in the blanks.",
		"Read the following passage then choose the best answer to each question below.",
		"Complete the sentence by changing the form of the word in capitals.",
	}
	if !slices.Equal(titles, want) {
		t.Fatalf("sections %q", titles)
	}
	if got := text(t, question(t, d, "3").Prompt); got != "I will take up golf this year." {
		t.Fatalf("prompt %q", got)
	}
}

func TestAQuestionWhoseSubLabelsFollowBecomesAGroupWithBoundGaps(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	g := groups(d)[0]
	if g.Label != "5" || len(g.Questions) != 2 || len(g.Gaps) != 2 {
		t.Fatalf("group %+v", g)
	}
	if got := text(t, g.Stimulus); got != "Many students learn grammar very well [5.1] cannot have a conversation with native speakers.\n\n[5.2] will hear your mistakes." {
		t.Fatalf("stimulus %q", got)
	}
	if got := answerLabels(question(t, d, "5.2")); !slices.Equal(got, []string{"A"}) {
		t.Fatalf("non-breaking space key not read: %v", got)
	}
}

func TestALeadingBlankStaysPartOfTheQuestion(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	q := question(t, d, "4")
	if got := text(t, q.Prompt); got != "_____ I visit him, we talk about sports a lot." || !slices.Equal(answerLabels(q), []string{"C"}) {
		t.Fatalf("question 4: %q %v", got, answerLabels(q))
	}
}

func TestLabelledGapKeysFillTheirBlank(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	q := question(t, d, "7")
	if q.Type != "fill_blank" || q.Blanks[0].Label != "7.1" || q.Blanks[0].Accepted[0] != "fluently" {
		t.Fatalf("question 7: %s %+v", q.Type, q.Blanks)
	}
	if got := text(t, q.Prompt); got != "My grandfather can speak English [7.1] so I admire him. (FLUENCY)" {
		t.Fatalf("prompt %q", got)
	}
}

func TestPartialUnderlineSurvivesInOptionContent(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	option := question(t, d, "1").Options[0]
	if !strings.Contains(string(option.Content), `{"type":"text","text":"ea","marks":["underline"]}`) {
		t.Fatalf("underline lost: %s", option.Content)
	}
}

func TestTheExamPaperNumberSelectsItsKeyAmongSeveral(t *testing.T) {
	d := recognize(t, questionPaper(), questionKey())
	if got := answerLabels(question(t, d, "1")); !slices.Equal(got, []string{"A"}) {
		t.Fatalf("paper 2 key not chosen: %v", got)
	}
}

func TestWithoutAKeyNoAnswerIsInvented(t *testing.T) {
	d := recognize(t, questionPaper())
	for _, q := range d.Questions() {
		if q.Answer.State != domain.AnswerUnknown || len(q.Answer.OptionIDs) != 0 || q.Answer.Text != "" || q.Origins.Answer != domain.Defaulted {
			t.Fatalf("question %s invented %+v", q.Label, q.Answer)
		}
	}
}

func TestAKeyThatCannotChooseAPaperAsksInsteadOfGuessing(t *testing.T) {
	untitled := exam(lines("1. A. yes\tB. no"))
	d := recognize(t, untitled, questionKey())
	if len(notices(d, domain.CodeKeyPaperAmbiguous)) != 1 || question(t, d, "1").Answer.State != domain.AnswerUnknown {
		t.Fatalf("notices %+v", d.Notices)
	}
	chosen := recognizeWith(t, domain.RecognitionProfile{KeyPaper: 1}, untitled, questionKey())
	if got := answerLabels(question(t, chosen, "1")); !slices.Equal(got, []string{"B"}) {
		t.Fatalf("chosen paper ignored: %v", got)
	}
}

func TestDisagreeingKeysAreAConflictWithBothValues(t *testing.T) {
	paper := exam(lines("TEST 1", "1. Pick one. A. cat\tB. dog\tAnswer: A"))
	d := recognize(t, paper, key(lines("1.B")))
	a := question(t, d, "1").Answer
	if a.State != domain.AnswerConflict || len(a.Candidates) != 2 || len(a.OptionIDs) != 0 {
		t.Fatalf("answer %+v", a)
	}
}

func TestLettersForAQuestionWithoutOptionsAreUnsupportedNotRelabelled(t *testing.T) {
	paper := exam(lines("TEST 1", "I. Match the sentence halves", "1. I lost my money\ta. but I just could not find it", "2. She likes you a lot\tb. because she thinks you are clever"))
	d := recognize(t, paper, key(lines("I.\t1. b\t2. a")))
	q := question(t, d, "1")
	if q.Type != domain.UnsupportedType || q.Answer.State == domain.AnswerKnown {
		t.Fatalf("matching relabelled as %s %+v", q.Type, q.Answer)
	}
}

func TestTrueFalseAnswerColumnsFollowTheSectionKeyVocabulary(t *testing.T) {
	paper := exam(lines("TEST 1", "I. Read the texts and tick", "True\tFalse", "1. Farrah lives in Istanbul\t…………..\t……………..", "2. Clare loves sitcoms\t…………..\t……………."))
	d := recognize(t, paper, key(lines("I.\t1. F\t2. T")))
	q := question(t, d, "1")
	if q.Type != "true_false" || !slices.Equal(answerLabels(q), []string{"F"}) || strings.Contains(text(t, q.Prompt), "…") {
		t.Fatalf("question 1: %s %v %q", q.Type, answerLabels(q), text(t, q.Prompt))
	}
}

func TestAMisprintedNumberIsKeptAndFlaggedRatherThanMerged(t *testing.T) {
	paper := exam(lines("TEST 1", "I. Find and correct the mistakes", "1. The collect of stamps made him famous  ……", "5. Let’s practice listening in the radio  ……"))
	d := recognize(t, paper)
	if got := labels(d); !slices.Equal(got, []string{"1", "5"}) {
		t.Fatalf("labels %v", got)
	}
	flagged := notices(d, domain.CodeNumberingIrregular)
	if len(flagged) != 1 || flagged[0].Target != question(t, d, "5").ID {
		t.Fatalf("notices %+v", d.Notices)
	}
}

func TestColouredTextInsideAQuestionIsFlaggedAsPossibleAnswerLeak(t *testing.T) {
	paper := exam(lines("TEST 1", "I. Complete the sentences"), []block{p(plain("1. They "), red("are"), plain("___ (be) on vacation."))})
	d := recognize(t, paper)
	flagged := notices(d, domain.CodeColoredText)
	if len(flagged) != 1 || flagged[0].Target != question(t, d, "1").ID || flagged[0].Severity != domain.ReviewRequired {
		t.Fatalf("notices %+v", d.Notices)
	}
}

func TestTextNoRuleCanPlaceIsReportedOnceAsUnassigned(t *testing.T) {
	paper := exam(lines("TEST 1", "I. Choose the best answer", "1. Pick. A. yes\tB. no", "Copyright notice for the school.", "Printed in 2024."))
	d := recognize(t, paper)
	unassigned := notices(d, domain.CodeUnassignedText)
	if len(unassigned) != 1 || unassigned[0].Count != 2 || len(unassigned[0].Evidence) != 2 {
		t.Fatalf("unassigned %+v", unassigned)
	}
}

func TestAnExamWithoutReadableTextIsUnsupported(t *testing.T) {
	_, err := recognizeErr(exam(lines("   ")))
	if err != domain.ErrUnsupported {
		t.Fatalf("err %v", err)
	}
}
