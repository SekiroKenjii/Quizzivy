package recognition_test

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"strings"
	"testing"
)

// The fixtures below are written for these tests; no real paper is copied into the repo.

var statements = []string{"1. The museum opens every day.", "2. Tickets are free for children.", "3. The café closes at six.", "4. Guides speak French."}

func withStatements(head ...block) []block { return append(head, lines(statements...)...) }

func TestASoftLineBreakInsideAnInstructionKeepsItWhole(t *testing.T) {
	for _, instruction := range []string{
		"I. Write TRUE,\nFALSE or\nNOT GIVEN.",
		"I. Ghi Đúng, Sai hoặc\nKhông có thông tin.",
		"I. Do the following statements agree with the passage? Write\nTRUE, FALSE or\nNOT GIVEN.",
	} {
		t.Run(instruction, func(t *testing.T) {
			d := recognize(t, exam(withStatements(p(plain(instruction)))), key(lines("1. T\t2. F\t3. T\t4. F")))
			if len(d.Sections) != 1 || len(d.Sections[0].Items) != 4 || d.Sections[0].Items[0].Group != nil {
				t.Fatalf("the tail of the instruction is not a passage: %+v", d.Sections)
			}
			for _, label := range []string{"1", "2", "3", "4"} {
				if q := question(t, d, label); q.Type != "tfng" {
					t.Fatalf("question %s: %+v", label, q)
				}
			}
			if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
				t.Fatalf("nothing is left over: %+v", got)
			}
		})
	}
}

func TestASoftLineBreakInsideChooseNOrAWordLimitStillReadsTheCount(t *testing.T) {
	d := recognize(t, exam(
		[]block{p(plain("I. Choose\nTWO\nletters, A-E."))},
		lines("1. Which two activities does the club offer?", "A. hiking\tB. painting\tC. chess\tD. cooking\tE. rowing"),
		[]block{p(plain("II. Answer the questions. Write NO MORE THAN\nTWO WORDS for each answer."))},
		lines("2. Where does Anna work?"),
	), key(lines("1. B, D\t2. in a shop")))
	if q := question(t, d, "1"); q.SelectCount == nil || *q.SelectCount != 2 {
		t.Fatalf("Choose TWO across breaks: %+v", q)
	}
	if q := question(t, d, "2"); q.Type != "short_answer" || q.MaxWords == nil || *q.MaxWords != 2 {
		t.Fatalf("word limit across a break: %+v", q)
	}
	if d.Sections[1].Title != "II. Answer the questions. Write NO MORE THAN TWO WORDS for each answer." {
		t.Fatalf("the title is the whole instruction: %q", d.Sections[1].Title)
	}
}

func TestASoftLineBreakBeforeAQuestionOrOptionStillSplits(t *testing.T) {
	d := recognize(t, exam(
		lines("I. Choose the best answer"),
		[]block{p(plain("1. Which is red?\nA. sky\tB. apple"))},
		[]block{p(plain("Choose the word\n2. Which is blue?\nA. sky\tB. apple"))},
	), key(lines("1. B\t2. A")))
	for label, want := range map[string]string{"1": "B", "2": "A"} {
		q := question(t, d, label)
		if len(q.Options) != 2 || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s keeps its own line and options: %+v", label, q)
		}
	}
}

func TestAnInstructionUnderAQuestionsHeadingReachesItsQuestions(t *testing.T) {
	t.Run("Choose N", func(t *testing.T) {
		d := recognize(t, exam(lines("Questions 1-2", "Choose TWO letters, A-E.", "1. Which two activities does the club offer?", "A. hiking\tB. painting\tC. chess\tD. cooking\tE. rowing")), key(lines("1. B, D")))
		if q := question(t, d, "1"); q.SelectCount == nil || *q.SelectCount != 2 {
			t.Fatalf("selectCount from the instruction under the heading: %+v", q)
		}
		if d.Title != "" || d.Sections[0].Title != "Questions 1-2 Choose TWO letters, A-E." {
			t.Fatalf("the heading is not the paper's title: %q / %q", d.Title, d.Sections[0].Title)
		}
	})
	t.Run("IELTS three lines", func(t *testing.T) {
		d := recognize(t, exam(lines("Questions 1-2", "Answer the questions below.", "Write NO MORE THAN THREE WORDS AND/OR A NUMBER for each answer.", "1. Where does the guide live?", "2. Who made the rule?")), key(lines("1. in Hue\t2. the editor")))
		for _, label := range []string{"1", "2"} {
			if q := question(t, d, label); q.MaxWords == nil || *q.MaxWords != 3 {
				t.Fatalf("question %s: %+v", label, q)
			}
		}
		if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
			t.Fatalf("every instruction line is assigned: %+v", got)
		}
	})
	t.Run("one line", func(t *testing.T) {
		d := recognize(t, exam(lines(append([]string{"Questions 1-4: Write TRUE, FALSE or NOT GIVEN."}, statements...)...)))
		for _, label := range []string{"1", "2", "3", "4"} {
			if q := question(t, d, label); q.Type != "tfng" {
				t.Fatalf("question %s: %+v", label, q)
			}
		}
	})
	t.Run("PART heading", func(t *testing.T) {
		d := recognize(t, exam(lines(append([]string{"PART 1", "Write TRUE, FALSE or NOT GIVEN."}, statements...)...)), key(lines("1. T\t2. F\t3. T\t4. F")))
		for _, label := range []string{"1", "2", "3", "4"} {
			if q := question(t, d, label); q.Type != "tfng" {
				t.Fatalf("question %s: %+v", label, q)
			}
		}
	})
	t.Run("Vietnamese range", func(t *testing.T) {
		d := recognize(t, exam(lines("ĐỀ KIỂM TRA", "Câu 1-2: Chọn HAI đáp án đúng. Choose TWO answers.", "Câu 1. Which two are fruit?", "A. apple\tB. sky\tC. pear", "Câu 2. Which two are blue?", "A. sky\tB. sea\tC. pear")), key(lines("1. A, C\t2. A, B")))
		for _, label := range []string{"1", "2"} {
			if q := question(t, d, label); q.SelectCount == nil || *q.SelectCount != 2 {
				t.Fatalf("question %s: %+v", label, q)
			}
		}
	})
}

func TestTheIELTSAnswerLegendUnderAHeadingIsTheInstruction(t *testing.T) {
	d := recognize(t, exam(lines(append([]string{
		"READING TEST",
		"Questions 1-4",
		"Do the following statements agree with the information given in the passage?",
		"In boxes 1-4 on your answer sheet, write",
		"TRUE if the statement agrees with the information",
		"FALSE if the statement contradicts the information",
		"NOT GIVEN if there is no information on this",
	}, statements...)...)), key(lines("1. TRUE\t2. NOT GIVEN\t3. TRUE\t4. NG")))
	for label, want := range map[string]string{"1": "T", "2": "NG", "3": "T", "4": "NG"} {
		if q := question(t, d, label); q.Type != "tfng" || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
}

// QC-R6IMP-11: a section keyed only T and NG is tfng once its instruction is read, not true_false mixed with
// short answers.
func TestAHeadingLayoutKeyedOnlyTrueAndNotGivenIsTFNG(t *testing.T) {
	d := recognize(t, exam(lines(append([]string{"Questions 1-4", "Do the statements agree with the passage?", "Write TRUE, FALSE or NOT GIVEN."}, statements...)...)), key(lines("1. TRUE\t2. NOT GIVEN\t3. TRUE\t4. NG")))
	for label, want := range map[string]string{"1": "T", "2": "NG", "3": "T", "4": "NG"} {
		if q := question(t, d, label); q.Type != "tfng" || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
}

func TestEachQuestionsHeadingStartsItsOwnPart(t *testing.T) {
	d := recognize(t, exam(lines(append(append([]string{"READING TEST", "Questions 1-4", "Write TRUE, FALSE or NOT GIVEN."}, statements...),
		"Questions 5-6",
		"Choose TWO letters, A-E.",
		"5. Which two rooms are new?",
		"A. hall\tB. shop\tC. café\tD. library\tE. garden",
		"Questions 7-8",
		"Answer the questions below. Write NO MORE THAN TWO WORDS for each answer.",
		"7. Who built it?",
		"8. When does it close?",
	)...)), key(lines("1. T\t2. F\t3. NG\t4. T\t5. B, D\t7. the king\t8. at six")))
	if len(d.Sections) != 3 {
		t.Fatalf("one part for each heading: %+v", d.Sections)
	}
	if q := question(t, d, "4"); q.Type != "tfng" || text(t, q.Prompt) != "Guides speak French." {
		t.Fatalf("the next heading is not part of question 4: %+v %s", q, text(t, q.Prompt))
	}
	if q := question(t, d, "5"); q.SelectCount == nil || *q.SelectCount != 2 {
		t.Fatalf("question 5: %+v", q)
	}
	for _, label := range []string{"7", "8"} {
		if q := question(t, d, label); q.MaxWords == nil || *q.MaxWords != 2 {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
}

func TestAHeadingInsideARomanSectionKeepsTheSection(t *testing.T) {
	d := recognize(t, exam(lines(append(append([]string{"ENGLISH TEST", "I. Reading", "Questions 1-4", "Write TRUE, FALSE or NOT GIVEN."}, statements...), "II. Writing", "5. Write about your town.")...)), key(lines("1. T\t2. F\t3. NG\t4. T")))
	if len(d.Sections) != 2 || d.Sections[0].Title != "I. Reading" {
		t.Fatalf("the roman sections stand: %+v", d.Sections)
	}
	g := d.Sections[0].Items[0].Group
	if g == nil || g.Instructions != "Questions 1-4 Write TRUE, FALSE or NOT GIVEN." || len(g.Questions) != 4 || g.Questions[0].Type != "tfng" {
		t.Fatalf("the heading's group carries the instruction: %+v", g)
	}
}

func TestAQuestionNamingAScoreIsNotAHeading(t *testing.T) {
	d := recognize(t, exam(lines("TEST", "Question 1 - 2 points", "Which is red?", "A. sky\tB. apple")), key(lines("1. B")))
	if q := question(t, d, "1"); len(q.Options) != 2 {
		t.Fatalf("question 1 stays a question: %+v", q)
	}
}

// QC-R6LAY-1: an instruction under a heading that lacks its full stop still ends at the paragraph; the passage after
// it stays the group's stimulus.
func TestAPassageAfterAnUnpunctuatedHeadingInstructionStaysTheStimulus(t *testing.T) {
	passage := "The museum opened in 1990 after a long campaign by local residents, who raised money for more than a decade."
	for name, doc := range map[string]domain.EvidenceDocument{
		"file": exam(lines("READING TEST", "Questions 1-2", "Answer the questions below", "Write NO MORE THAN TWO WORDS for each answer", passage, "1. When did it open?", "2. Who raised money?")),
		"text": textExam("READING TEST", "", "Questions 1-2", "Write TRUE, FALSE or NOT GIVEN", passage, "1. The museum opened in 1990.", "Answer: TRUE", "2. Entry is free.", "Answer: NOT GIVEN"),
	} {
		t.Run(name, func(t *testing.T) {
			d := recognize(t, doc)
			g := d.Sections[0].Items[0].Group
			if g == nil || g.Stimulus == nil || !strings.Contains(text(t, g.Stimulus), "The museum opened in 1990") {
				t.Fatalf("the passage is the stimulus: %+v", d.Sections)
			}
			if strings.Contains(d.Sections[0].Title+d.Sections[0].Instructions, "museum") {
				t.Fatalf("the passage is not part of the instruction: %q / %q", d.Sections[0].Title, d.Sections[0].Instructions)
			}
		})
	}
	d := recognize(t, exam(lines("READING TEST", "Questions 1-2", "Answer the questions below", "Write NO MORE THAN TWO WORDS for each answer", passage, "1. When did it open?", "2. Who raised money?")), key(lines("1. in 1990\t2. residents")))
	if q := question(t, d, "1"); q.MaxWords == nil || *q.MaxWords != 2 {
		t.Fatalf("the instruction still reaches the questions: %+v", q)
	}
}

// QC-R6LAY-2: questions whose stem opens with an imperative ("Pick one") still belong to the heading that names them.
func TestAHeadingKeepsTheQuestionsItNames(t *testing.T) {
	d := recognize(t, exam(lines("Questions 1-2", "1. Pick one", "A. a\tB. b\tC. c", "2. Pick one", "A. a\tB. b\tC. c")), key(lines("1. A\t2. B")))
	if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
		t.Fatalf("the heading is assigned: %+v", got)
	}
	if g := d.Sections[0].Items[0].Group; g == nil || len(g.Questions) != 2 {
		t.Fatalf("both questions sit under the heading: %+v", d.Sections[0].Items)
	}
}

// QC-R6LAY-3: a heading straight above a section heading leaves no empty part.
func TestAHeadingAboveASectionHeadingJoinsTheSection(t *testing.T) {
	d := recognize(t, exam(withStatements(p(plain("ENGLISH TEST")), p(plain("Questions 1-4")), p(plain("I. Write TRUE, FALSE or NOT GIVEN.")))), key(lines("1. T\t2. F\t3. NG\t4. T")))
	if len(d.Sections) != 1 || d.Sections[0].Title != "I. Write TRUE, FALSE or NOT GIVEN." || len(d.Sections[0].Items) != 4 {
		t.Fatalf("one section: %+v", d.Sections)
	}
	if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
		t.Fatalf("the heading is assigned: %+v", got)
	}
}

// QC-R6LAY-4: an upper-case passage title after a soft break is not the rest of the instruction.
func TestAnUpperCaseTitleAfterASoftBreakIsNotPartOfTheInstruction(t *testing.T) {
	d := recognize(t, exam(lines("I. Read the passage and answer the questions\nTHE HISTORY OF TEA", "Tea came to Europe in the 1600s.", "1. When did tea come to Europe?")), key(lines("I.\t1. 1600s")))
	if d.Sections[0].Title != "I. Read the passage and answer the questions" {
		t.Fatalf("title: %q", d.Sections[0].Title)
	}
}

// QC-R6LAY-7: a passage that opens with a word an instruction could open with is still the passage.
func TestAPassageOpeningLikeAnInstructionStaysThePassage(t *testing.T) {
	for _, passage := range []string{
		"Look around any modern city and you will see glass towers.",
		"Use of mobile phones has grown fast since 1990 in every country.",
		"Mark Twain once wrote that the museum was the finest on the river.",
		"Make no mistake: the museum opened in 1990 after a long fight.",
	} {
		t.Run(passage, func(t *testing.T) {
			d := recognize(t, exam(lines("Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
			g := d.Sections[0].Items[0].Group
			if g == nil || g.Stimulus == nil || text(t, g.Stimulus) != passage || d.Sections[0].Title != "Questions 1-2 Write TRUE, FALSE or NOT GIVEN." {
				t.Fatalf("the passage is the stimulus: %+v", d.Sections)
			}
			for _, label := range []string{"1", "2"} {
				if q := question(t, d, label); q.Type != "tfng" {
					t.Fatalf("question %s: %+v", label, q)
				}
			}
		})
	}
}

// A passage title between the heading and its instruction stays the stimulus, and the instruction still applies.
func TestAPassageTitleBetweenTheHeadingAndItsInstruction(t *testing.T) {
	d := recognize(t, exam(withStatements(p(plain("Questions 1-4")), p(plain("THE MUSEUM")), p(plain("Write TRUE, FALSE or NOT GIVEN.")))), key(lines("1. T\t2. F\t3. T\t4. F")))
	g := d.Sections[0].Items[0].Group
	if g == nil || g.Stimulus == nil || text(t, g.Stimulus) != "THE MUSEUM" || g.Questions[0].Type != "tfng" {
		t.Fatalf("title as stimulus, instruction read: %+v", d.Sections)
	}
	if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
		t.Fatalf("nothing is left over: %+v", got)
	}
}

// QC-R6LAY-8: "write" on its own line, as text copied from a PDF breaks it, keeps the legend together.
func TestALegendWithWriteOnItsOwnLineIsTFNG(t *testing.T) {
	d := recognize(t, exam(lines("Questions 1-2", "Do the following statements agree with the information given in Reading Passage 1?", "In boxes 1-2 on your answer sheet,", "write", "TRUE if the statement agrees with the information", "FALSE if the statement contradicts the information", "NOT GIVEN if there is no information on this", "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
	for label, want := range map[string]string{"1": "T", "2": "NG"} {
		if q := question(t, d, label); q.Type != "tfng" || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
}

// QC-R6LAY-9: upper-case Vietnamese values and option letters after a soft break are the rest of the instruction.
func TestUpperCaseInstructionTailsAfterASoftBreakJoin(t *testing.T) {
	d := recognize(t, exam(lines("I. Chọn\nĐÚNG, SAI HOẶC KHÔNG CÓ THÔNG TIN", "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("I.\t1. TRUE\t2. NOT GIVEN")))
	if d.Sections[0].Title != "I. Chọn ĐÚNG, SAI HOẶC KHÔNG CÓ THÔNG TIN" || question(t, d, "1").Type != "tfng" {
		t.Fatalf("Vietnamese tail: %+v", d.Sections)
	}
	d = recognize(t, exam(lines("I. Choose the correct letter\nA, B, C OR D", "1. Which is red?", "A. sky\tB. apple\tC. sea\tD. grass")), key(lines("I.\t1. B")))
	if d.Sections[0].Title != "I. Choose the correct letter A, B, C OR D" {
		t.Fatalf("letters tail: %q", d.Sections[0].Title)
	}
}
