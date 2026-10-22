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

// QC-R6LAY-7, QC-R6LAY-10: a passage that opens like a command ("Use of…", "Mark Twain…", "Look … and you will see",
// "Make no mistake", "Đọc sách là…") is read by its grammar, so it stays the stimulus under a heading, with or
// without the instruction's full stop.
func TestAPassageThatOpensLikeACommandStaysTheStimulus(t *testing.T) {
	for _, passage := range []string{
		"Use of mobile phones has grown fast since 1990 in every country.",
		"Mark Twain described this passage of the river as the finest stretch in America.",
		"Look at the river below the old bridge and you will see where the museum began in 1990.",
		"Read any newspaper today and you'll find the following words: crisis, climate, change.",
		"Make no mistake: the text of the 1990 charter still governs the museum today.",
		"Đọc sách là thói quen tốt của người Việt từ lâu đời.",
		"Tìm kiếm thông tin trên mạng ngày càng phổ biến với học sinh.",
		"Viết nhật ký mỗi ngày giúp học sinh nhớ bài lâu hơn.",
		"Chọn nghề khiến nhiều học sinh lo lắng.",
	} {
		for _, instruction := range []string{"Write TRUE, FALSE or NOT GIVEN", "Write TRUE, FALSE or NOT GIVEN."} {
			t.Run(passage+" / "+instruction, func(t *testing.T) {
				d := recognize(t, exam(lines("Questions 1-2", instruction, passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
				g := d.Sections[0].Items[0].Group
				if g == nil || g.Stimulus == nil || text(t, g.Stimulus) != passage {
					t.Fatalf("the passage is the stimulus: %+v", d.Sections)
				}
				if d.Sections[0].Title != "Questions 1-2 "+instruction {
					t.Fatalf("title: %q", d.Sections[0].Title)
				}
				for _, label := range []string{"1", "2"} {
					if q := question(t, d, label); q.Type != "tfng" || q.Task != "" {
						t.Fatalf("question %s takes nothing from the passage: %+v", label, q)
					}
				}
			})
		}
	}
}

// QC-R6LAY-11, QC-R6LAY-14: the grammar never asks an instruction for a task word or a length, so a short command
// with no answer format and a long one with a word limit both stay the instruction, and a line that names an answer
// format is one whatever its first sentence reads like.
func TestAnInstructionStaysTheInstructionWhateverItNames(t *testing.T) {
	passage := "The museum opened in 1990 after a long campaign by local residents, who raised money for more than a decade."
	for _, instruction := range []string{
		"Chọn phương án đúng nhất.",
		"Chọn phương án đúng",
		"Choose TWO.",
		"Circle the odd one out.",
		"Complete the flow-chart",
		"Complete Table 1 below.",
		"Fill In The Blanks",
		"Choose One answer",
		"Use no more than one letter in each box.",
		"Look at the picture and answer the questions.",
		"Đọc đoạn văn sau và cho biết câu nào là đúng.",
		"Use of a dictionary is not allowed. Choose the correct letter, A, B or C.",
	} {
		t.Run(instruction, func(t *testing.T) {
			d := recognize(t, exam(lines("Questions 1-2", instruction, passage, "1. When did it open?", "A. 1980\tB. 1990\tC. 2000", "2. Who raised money?", "A. residents\tB. the city\tC. a bank")), key(lines("1. B\t2. A")))
			if d.Sections[0].Title != "Questions 1-2 "+instruction {
				t.Fatalf("title: %q", d.Sections[0].Title)
			}
			if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil || text(t, g.Stimulus) != passage {
				t.Fatalf("the passage is the stimulus: %+v", d.Sections)
			}
		})
	}
	long := "Complete the sentences below using information from Reading Passage 1. Write NO MORE THAN TWO WORDS AND/OR A NUMBER from the passage for each answer. Write your answers in boxes 1-2 on your answer sheet. Spelling must be correct."
	d := recognize(t, exam(lines("Questions 1-2", long, passage, "1. When did it open?", "2. Who raised money?")), key(lines("1. 1990\t2. residents")))
	for _, label := range []string{"1", "2"} {
		if q := question(t, d, label); q.MaxWords == nil || *q.MaxWords != 2 {
			t.Fatalf("question %s keeps the word limit of a long instruction: %+v", label, q)
		}
	}
	if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil || text(t, g.Stimulus) != passage {
		t.Fatalf("the passage is the stimulus: %+v", d.Sections)
	}
}

// QC-R6LAY-8: text copied from a PDF breaks the legend's "write" onto its own lower-case line; it carries on the
// unfinished line before it, so the legend still reaches the questions.
func TestALowerCaseLineCarriesOnTheHeadingInstruction(t *testing.T) {
	d := recognize(t, exam(lines(append([]string{
		"Questions 1-4",
		"Do the following statements agree with the information given in Reading Passage 1?",
		"In boxes 1-4 on your answer sheet,",
		"write",
		"TRUE if the statement agrees with the information",
		"FALSE if the statement contradicts the information",
		"NOT GIVEN if there is no information on this",
	}, statements...)...)), key(lines("1. TRUE\t2. NOT GIVEN\t3. FALSE\t4. NG")))
	for label, want := range map[string]string{"1": "T", "2": "NG", "3": "F", "4": "NG"} {
		if q := question(t, d, label); q.Type != "tfng" || !slices.Equal(answerLabels(q), []string{want}) {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
	d = recognize(t, exam(lines("Questions 1-2", "write no more than two words for each answer", "1. When did it open?", "2. Who raised money?")), key(lines("1. 1990\t2. residents")))
	if q := question(t, d, "1"); q.MaxWords == nil || *q.MaxWords != 2 {
		t.Fatalf("a lower-case instruction straight under the heading: %+v", q)
	}
	d = recognize(t, exam(lines("Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "the passage below was found in 1990.", "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
	if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil {
		t.Fatalf("nothing carries on a finished sentence: %+v", d.Sections)
	}
}

// QC-R6LAY-9, QC-R6LAY-12, QC-R6LAY-13: an upper-case tail after a soft break is part of the instruction when it
// names an answer format in either language, and a title when it only holds such words.
func TestAnAnswerFormatAfterASoftBreakIsPartOfTheInstruction(t *testing.T) {
	for _, c := range []struct{ head, tail string }{
		{"I. Chọn", "ĐÚNG, SAI HOẶC KHÔNG CÓ THÔNG TIN"},
		{"I. Viết câu trả lời", "KHÔNG QUÁ HAI TỪ"},
		{"I. Choose the correct letter", "A, B, C OR D"},
	} {
		d := recognize(t, exam(lines(c.head+"\n"+c.tail, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
		if d.Sections[0].Title != c.head+" "+c.tail {
			t.Fatalf("the tail joins: %q", d.Sections[0].Title)
		}
	}
	d := recognize(t, exam(lines("I. Chọn\nĐÚNG, SAI HOẶC KHÔNG CÓ THÔNG TIN", "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
	for _, label := range []string{"1", "2"} {
		if q := question(t, d, label); q.Type != "tfng" {
			t.Fatalf("question %s: %+v", label, q)
		}
	}
	for _, title := range []string{"ĐÚNG HAY SAI?", "PLAN A OR B", "KHÔNG GIAN XANH", "SAI LẦM LỚN NHẤT", "TỪ ĐIỂN TIẾNG VIỆT", "THÔNG TIN MỚI"} {
		d := recognize(t, exam(lines("I. Đọc đoạn văn và trả lời câu hỏi\n"+title, "The museum opened in 1990.", "1. When did it open?", "2. Who raised money?")), key(lines("1. 1990\t2. residents")))
		if d.Sections[0].Title != "I. Đọc đoạn văn và trả lời câu hỏi" {
			t.Fatalf("%s is a title: %q", title, d.Sections[0].Title)
		}
		if q := question(t, d, "1"); q.Type != "short_answer" {
			t.Fatalf("%s sets no type: %+v", title, q)
		}
	}
}

// QC-R6LAY-5: headings with nothing between them share the instruction under the last one, and hold the questions
// either names.
func TestConsecutiveHeadingsShareTheInstruction(t *testing.T) {
	d := recognize(t, exam(lines(append([]string{"Questions 1-2", "Questions 3-4", "Write TRUE, FALSE or NOT GIVEN."}, statements...)...)), key(lines("1. T\t2. F\t3. T\t4. F")))
	if len(d.Sections) != 1 || d.Sections[0].Title != "Questions 1-2 Questions 3-4 Write TRUE, FALSE or NOT GIVEN." {
		t.Fatalf("one part: %+v", d.Sections)
	}
	if g := d.Sections[0].Items[0].Group; g == nil || len(g.Questions) != 4 {
		t.Fatalf("one group holds all four: %+v", d.Sections[0].Items)
	}
	if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
		t.Fatalf("neither heading is left over: %+v", got)
	}
}

// QC-R6GRM-1: the grammar that tells a passage from a second instruction line never takes a real instruction. It is
// asked only under a heading that already has its instruction, and an instruction that asks, glosses or promises the
// answers ("là từ nào", "có nghĩa là", "and you will find the answers") is not a statement.
func TestTheGrammarNeverTakesARealInstruction(t *testing.T) {
	passage := "The museum opened in 1990 after a long campaign by local residents, who raised money for more than a decade."
	questions := []string{"1. When did it open?", "A. 1980\tB. 1990\tC. 2000", "2. Who raised money?", "A. residents\tB. the city\tC. a bank"}
	for _, instruction := range []string{
		"Chọn từ có nghĩa là 'vui vẻ'.",
		"Tìm từ khác loại là từ nào trong mỗi câu sau.",
		"Read the passage carefully and you will find the answers.",
		"Look at the pictures and you can answer the questions.",
		"Write Yes if the statement agrees with the writer.",
		"Use of English: choose the correct answer.",
		"Use no more than one letter in each box.",
		"Use of a dictionary is not allowed. Choose the correct letter, A, B or C.",
	} {
		t.Run(instruction, func(t *testing.T) {
			for name, head := range map[string][]string{
				"under a heading":   {"Questions 1-2", instruction},
				"second line":       {"Questions 1-2", "Answer the questions below.", instruction},
				"with no structure": {instruction},
			} {
				d := recognize(t, exam(lines(append(append(head, passage), questions...)...)), key(lines("1. B\t2. A")))
				if !strings.Contains(d.Sections[0].Title, instruction) {
					t.Fatalf("%s: the instruction is the title: %q", name, d.Sections[0].Title)
				}
				if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil || text(t, g.Stimulus) != passage {
					t.Fatalf("%s: the passage is the stimulus: %+v", name, d.Sections)
				}
				if got := notices(d, domain.CodeUnassignedText); len(got) != 0 {
					t.Fatalf("%s: nothing is left over: %+v", name, got)
				}
			}
		})
	}
}

// The grammar is a tie-break for a second line only: straight under a heading, even a line it reads as a statement
// ("Make no …") is the instruction.
func TestTheFirstLineUnderAHeadingIsTheInstruction(t *testing.T) {
	d := recognize(t, exam(lines("Questions 1-2", "Make no changes to the words given.", "1. Rewrite: She is tall.", "2. Rewrite: He is old.")), key(lines("1. Tall she is.\t2. Old he is.")))
	if d.Sections[0].Title != "Questions 1-2 Make no changes to the words given." {
		t.Fatalf("title: %q", d.Sections[0].Title)
	}
}

// QC-R6GRM-3, QC-R6GRM-4: an instruction broken over hard lines carries on when the next line opens with a
// preposition or completes the answer format the line began; a passage that opens in lower case does not.
func TestAnInstructionBrokenOverHardLinesStaysWhole(t *testing.T) {
	passage := "The museum opened in 1990 after a long campaign by local residents, who raised money for more than a decade."
	d := recognize(t, exam(lines("Questions 1-2", "Choose", "TWO letters, A-E.", passage, "1. Which two?", "A. a\tB. b\tC. c\tD. d\tE. e")), key(lines("1. B, D")))
	if q := question(t, d, "1"); q.SelectCount == nil || *q.SelectCount != 2 {
		t.Fatalf("Choose / TWO: %+v", q)
	}
	d = recognize(t, exam(lines("Câu 1-2", "Chọn", "Đúng, Sai hoặc Không có thông tin", passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. Đúng\t2. Không có thông tin")))
	for _, label := range []string{"1", "2"} {
		if q := question(t, d, label); q.Type != "tfng" {
			t.Fatalf("Chọn / Đúng, Sai…: question %s: %+v", label, q)
		}
	}
	d = recognize(t, exam(lines("Questions 1-2", "Write the correct letter,", "A, B or C, in boxes 1-2 on your answer sheet.", passage, "1. When?", "A. 1980\tB. 1990\tC. 2000")), key(lines("1. B")))
	if d.Sections[0].Title != "Questions 1-2 Write the correct letter, A, B or C, in boxes 1-2 on your answer sheet." {
		t.Fatalf("the letters join: %q", d.Sections[0].Title)
	}
	for _, c := range []struct{ head, tail string }{
		{"Write NO MORE THAN TWO WORDS", "from the passage for each answer."},
		{"Viết KHÔNG QUÁ HAI TỪ", "cho mỗi câu trả lời."},
	} {
		d = recognize(t, exam(lines("Câu 1-2", "Complete the sentences below.", c.head, c.tail, passage, "1. When did it open?", "2. Who raised money?")), key(lines("1. 1990\t2. residents")))
		if want := "Câu 1-2 Complete the sentences below. " + c.head + " " + c.tail; d.Sections[0].Title != want {
			t.Fatalf("the phrase carries on: %q", d.Sections[0].Title)
		}
	}
	d = recognize(t, exam(lines("Questions 1-2", "Write TRUE, FALSE or NOT GIVEN", "iPads and phones changed the museum after 2010.", "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
	if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil || d.Sections[0].Title != "Questions 1-2 Write TRUE, FALSE or NOT GIVEN" {
		t.Fatalf("a lower-case passage stays the stimulus: %q %+v", d.Sections[0].Title, d.Sections)
	}
}

// QC-R6GRM-2, D-073: no grammar tells "Choose your words carefully, the old saying goes…" from an instruction, so a
// heading's instruction that took a sentence after its first line, with questions straight after it and no passage,
// carries one informational finding on its section. An instruction whose later lines name the answer format, or a
// section with a passage, carries none.
func TestALongInstructionPointsAtAPossiblePassage(t *testing.T) {
	for _, c := range []struct{ heading, instruction, passage string }{
		{"Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "Choose your words carefully, the old saying goes, because they cannot be taken back."},
		{"Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "Complete silence filled the hall when the museum opened in 1990."},
		{"Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "Decide for yourself whether the museum succeeded."},
		{"Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "Look: the museum opened in 1990."},
		{"Câu 1-2", "Chọn Đúng, Sai hoặc Không có thông tin.", "Viết văn không dễ chút nào với học sinh lớp sáu."},
		{"Câu 1-2", "Chọn Đúng, Sai hoặc Không có thông tin.", "Tìm việc làm ở thành phố ngày nay rất khó."},
	} {
		t.Run(c.passage, func(t *testing.T) {
			d := recognize(t, exam(lines(c.heading, c.instruction, c.passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. NOT GIVEN")))
			got := notices(d, domain.CodeLongInstruction)
			if len(got) != 1 || got[0].Severity != domain.Informational || got[0].Target != d.Sections[0].ID {
				t.Fatalf("one informational finding on the section: %+v", got)
			}
		})
	}
	passage := "The museum opened in 1990 after a long campaign by local residents, who raised money for more than a decade."
	for name, doc := range map[string]domain.EvidenceDocument{
		"a limit on the later line":  exam(lines("Questions 1-2", "Answer the questions below.", "Write NO MORE THAN THREE WORDS AND/OR A NUMBER for each answer.", "1. Where does the guide live?", "2. Who made the rule?")),
		"the IELTS legend":           exam(lines(append([]string{"Questions 1-4", "Do the following statements agree with the information given in the passage?", "In boxes 1-4 on your answer sheet, write", "TRUE if the statement agrees with the information", "FALSE if the statement contradicts the information", "NOT GIVEN if there is no information on this"}, statements...)...)),
		"a passage of its own":       exam(lines("Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", "Decide for yourself whether the museum succeeded.", passage, "1. The museum opened in 1990.", "2. Entry is free.")),
		"a one-line instruction":     exam(lines("Questions 1-2", "Write TRUE, FALSE or NOT GIVEN.", passage, "1. The museum opened in 1990.", "2. Entry is free.")),
		"a first line of any length": exam(lines("Questions 1-2", "Choose the best answer for each question.", "1. When did it open?", "A. 1980\tB. 1990", "2. Who paid?", "A. city\tB. residents")),
		"a broken line carried on":   exam(lines("Questions 1-2", "Complete the sentences below.", "Write NO MORE THAN TWO WORDS", "from the passage for each answer.", "1. When did it open?", "2. Who paid?")),
		"a short later line":         exam(lines("Questions 1-2", "Answer the questions below.", "Read carefully.", "1. When did it open?", "2. Who paid?")),
	} {
		t.Run(name, func(t *testing.T) {
			if got := notices(recognize(t, doc), domain.CodeLongInstruction); len(got) != 0 {
				t.Fatalf("no finding: %+v", got)
			}
		})
	}
}

// QC-R6GRM-5: a passage that opens like a command, after the instruction of a Roman section or of a plain one, is
// the stimulus; straight under a bare heading it is still taken as the instruction, but pointed at (D-073).
func TestACommandLikePassageAfterAnyInstructionStaysTheStimulus(t *testing.T) {
	instruction := "Read the passage and decide if the statements are true or false."
	for _, passage := range []string{
		"Mark Twain once wrote that the museum was the finest in the state, and visitors still quote him today.",
		"Make no mistake: the museum changed the town for ever when it opened in 1990.",
		"Đọc sách là thói quen tốt giúp học sinh mở rộng hiểu biết về thế giới xung quanh.",
	} {
		for name, head := range map[string][]string{"roman": {"I. " + instruction}, "plain": {instruction}} {
			t.Run(name+"/"+passage, func(t *testing.T) {
				d := recognize(t, exam(lines(append(head, passage, "1. The museum opened in 1990.", "2. Entry is free.")...)), key(lines("1. TRUE\t2. FALSE")))
				g := d.Sections[0].Items[0].Group
				if g == nil || g.Stimulus == nil || strings.Contains(d.Sections[0].Title+g.Instructions, passage[:10]) {
					t.Fatalf("the passage is the stimulus: %+v", d.Sections)
				}
				if q := question(t, d, "1"); q.Task != "" {
					t.Fatalf("the passage sets no task: %+v", q)
				}
			})
		}
		t.Run("bare heading/"+passage, func(t *testing.T) {
			d := recognize(t, exam(lines("Questions 1-2", passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. FALSE")))
			if got := notices(d, domain.CodeLongInstruction); len(got) != 1 || got[0].Severity != domain.Informational {
				t.Fatalf("the line is pointed at: %+v", got)
			}
		})
	}
	d := recognize(t, exam(lines("Questions 1-2", "Choose the best answer for each question.", "1. When did it open?", "A. 1980\tB. 1990", "2. Who paid?", "A. city\tB. residents")), key(lines("1. B\t2. B")))
	if got := notices(d, domain.CodeLongInstruction); len(got) != 0 {
		t.Fatalf("an instruction straight under a heading is not pointed at: %+v", got)
	}
}

// QC-R6GRM-6: a Vietnamese predicate reads as a statement only in the first clause, so a second instruction line
// that glosses its task after a comma or semicolon stays the instruction.
func TestAVietnamesePredicateInALaterClauseKeepsTheInstruction(t *testing.T) {
	for _, second := range []string{
		"Chọn đáp án đúng, mỗi câu chỉ có một đáp án là đúng.",
		"Điền vào chỗ trống, mỗi chỗ trống là một từ.",
		"Chọn đáp án đúng; lưu ý câu 2 là câu khó.",
		"Sắp xếp các từ sau thành câu hoàn chỉnh, từ đầu tiên đã được viết hoa giúp em.",
	} {
		d := recognize(t, exam(lines("Câu 1-2", "Đọc đoạn văn sau.", second, "Bảo tàng mở cửa năm 1990 sau nhiều năm vận động.", "1. Bảo tàng mở cửa năm nào?", "2. Ai góp tiền?")), key(lines("1. 1990\t2. người dân")))
		if want := "Câu 1-2 Đọc đoạn văn sau. " + second; d.Sections[0].Title != want {
			t.Fatalf("the line joins: %q", d.Sections[0].Title)
		}
	}
}

// QC-R6GRM-7, QC-R6GRM-8: a line that completes the answer format the instruction line before it began joins it in
// every layout, unless what follows the format reads as a sentence of its own.
func TestABrokenAnswerFormatJoinsInEveryLayout(t *testing.T) {
	passage := "The museum opened in 1990 after a long campaign by local residents."
	for name, prefix := range map[string]string{"roman": "I. ", "plain": ""} {
		d := recognize(t, exam(lines(prefix+"Choose", "TWO letters, A-E.", passage, "1. Which TWO are true?", "A. a\tB. b\tC. c\tD. d\tE. e")), key(lines("1. B, D")))
		if q := question(t, d, "1"); q.SelectCount == nil || *q.SelectCount != 2 {
			t.Fatalf("%s: Choose / TWO: %+v", name, q)
		}
		d = recognize(t, exam(lines(prefix+"Chọn", "Đúng, Sai hoặc Không có thông tin", passage, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. Đúng\t2. Không có thông tin")))
		if q := question(t, d, "2"); q.Type != "tfng" {
			t.Fatalf("%s: Chọn / Đúng, Sai…: %+v", name, q)
		}
		d = recognize(t, exam(lines(prefix+"Write the correct letter,", "A, B or C, in boxes 1-2 on your answer sheet.", passage, "1. When?", "A. 1980\tB. 1990\tC. 2000")), key(lines("1. B")))
		if text := d.Sections[0].Title + " " + instructionsOf(d); !strings.Contains(text, "Write the correct letter, A, B or C, in boxes 1-2") {
			t.Fatalf("%s: the letters join: %q", name, text)
		}
	}
	for _, c := range []struct{ head, line string }{
		{"Choose", "A, B and C were the three rooms of the museum when it opened in 1990."},
		{"Read the passage", "True or false, the story of the museum is a long one that began in 1980."},
		{"Read the passage and answer", "no more than two words were spoken by the mayor when the museum opened in 1990."},
	} {
		d := recognize(t, exam(lines("Questions 1-2", c.head, c.line, "1. The museum opened in 1990.", "2. Entry is free.")), key(lines("1. TRUE\t2. FALSE")))
		if g := d.Sections[0].Items[0].Group; g == nil || g.Stimulus == nil || d.Sections[0].Title != "Questions 1-2 "+c.head {
			t.Fatalf("%s: the sentence is the stimulus: %q", c.line, d.Sections[0].Title)
		}
	}
}

// instructionsOf joins the instructions of the first section's groups.
func instructionsOf(d domain.Draft) string {
	var text []string
	for _, it := range d.Sections[0].Items {
		if it.Group != nil {
			text = append(text, it.Group.Instructions)
		}
	}
	return strings.Join(text, " ")
}
