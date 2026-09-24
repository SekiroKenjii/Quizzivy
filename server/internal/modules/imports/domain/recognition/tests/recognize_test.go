package recognition_test

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/shared/content"
	"strings"
	"testing"
	"unicode/utf8"
)

func doc(role string, lines ...string) domain.EvidenceDocument {
	d := domain.EvidenceDocument{SourceID: role, Role: role, Version: "ooxml-blocks-v1"}
	for i, line := range lines {
		d.Blocks = append(d.Blocks, domain.EvidenceBlock{ID: fmt.Sprintf("block-%d", i), Kind: "paragraph", Main: true, Safe: true, Meaningful: true, Text: line, Spans: []domain.EvidenceSpan{{Start: 0, End: utf8.RuneCountInString(line), Marks: []string{}}}, Reasons: []string{}})
	}
	return d
}
func recognize(t *testing.T, docs ...domain.EvidenceDocument) domain.Candidate {
	t.Helper()
	result, err := recognition.Recognize(context.Background(), docs, domain.RecognitionProfile{Version: "auto-v1"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func hasIssue(c domain.Candidate, code string) bool {
	for _, i := range c.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
func plain(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	d, err := content.ParseQuestion(raw)
	if err != nil {
		t.Fatal(err)
	}
	return d.PlainText()
}

func TestInlineVietnameseOptionsKeepUnicodeEvidenceAndStripExplicitKeyFromLearnerProse(t *testing.T) {
	d := doc("exam", "Phần I: Ngữ pháp", "Câu 1. Chọn từ phù hợp: A. tiếng Việt B. English Đáp án: B")
	c := recognize(t, d)
	if len(c.Questions) != 1 {
		t.Fatalf("questions: %d", len(c.Questions))
	}
	q := c.Questions[0]
	if q.Answer.State != "known" || len(q.Options) != 2 || q.Answer.OptionIDs[0] != q.Options[1].ID {
		t.Fatalf("wrong explicit answer: %+v", q.Answer)
	}
	if got := plain(t, q.Prompt); got != "Chọn từ phù hợp:" {
		t.Fatalf("prompt: %q", got)
	}
	if got := plain(t, q.Options[0].Content); got != "tiếng Việt" {
		t.Fatalf("option: %q", got)
	}
	if got := plain(t, q.Options[1].Content); got != "English" {
		t.Fatalf("key leaked: %q", got)
	}
	if hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("complete inline source has coverage hole")
	}
	for _, field := range q.Fields {
		for _, ref := range field.Refs {
			if ref.End > utf8.RuneCountInString(d.Blocks[1].Text) {
				t.Fatal("byte offsets used instead of code points")
			}
		}
	}
}

func TestRestartedQuestionNumbersDoNotResolveUnscopedCompanionKeys(t *testing.T) {
	exam := doc("exam", "Part I: Grammar", "1. First A. yes B. no", "1. Second A. yes B. no")
	for _, key := range []domain.EvidenceDocument{doc("answer_key", "1. A"), doc("answer_key", "Part I: Grammar", "1. A")} {
		c := recognize(t, exam, key)
		if len(c.Questions) != 2 || c.Questions[0].SectionID == c.Questions[1].SectionID {
			t.Fatal("restarted structure lost")
		}
		if !hasIssue(c, "AMBIGUOUS_ANSWER_MAPPING") {
			t.Fatal("ambiguous printed number assigned")
		}
		for _, q := range c.Questions {
			if q.Answer.State != "unknown" {
				t.Fatal("guessed answer across restarts")
			}
		}
	}
}

func TestCompanionKeysUseSectionIdentityAndRetainContradictions(t *testing.T) {
	exam := doc("exam", "Part I: Grammar", "1. First A. yes B. no Answer: B", "Part II: Reading", "1. Second A. yes B. no")
	key := doc("answer_key", "Part I: Grammar", "1. A", "Part II: Reading", "1. B")
	c := recognize(t, exam, key)
	if c.Questions[0].Answer.State != "conflicting" || len(c.Questions[0].Answer.OptionIDs) != 0 {
		t.Fatal("conflicting keys silently picked one")
	}
	if c.Questions[1].Answer.State != "known" || c.Questions[1].Answer.OptionIDs[0] != c.Questions[1].Options[1].ID {
		t.Fatal("section-scoped key not matched")
	}
	if !hasIssue(c, "CONFLICTING_ANSWER_KEYS") {
		t.Fatal("missing conflict finding")
	}
}

func TestFinalKeyLineSupportsMultipleEntriesWithoutSolvingMissingAnswers(t *testing.T) {
	c := recognize(t, doc("exam", "1. Same A. yes B. no", "2. Same A. yes B. no", "3. Same A. yes B. no", "ANSWER KEY", "1. B 2. A"))
	if c.Questions[0].Answer.State != "known" || c.Questions[1].Answer.State != "known" || c.Questions[2].Answer.State != "unknown" {
		t.Fatalf("key association: %+v", c.Keys)
	}
	if c.Questions[0].ID == c.Questions[1].ID {
		t.Fatal("duplicate text deduplicated distinct questions")
	}
	if !hasIssue(c, "MISSING_ANSWER") {
		t.Fatal("missing key was invented")
	}
}

func TestFormattingIsNotAnAnswerWithoutConfirmedConvention(t *testing.T) {
	d := doc("exam", "1. Sound A. apple B. banana")
	text := []rune(d.Blocks[0].Text)
	start := strings.Index(string(text), "apple")
	d.Blocks[0].Spans = []domain.EvidenceSpan{{Start: 0, End: start, Marks: []string{}}, {Start: start, End: start + 5, Marks: []string{"underline"}}, {Start: start + 5, End: len(text), Marks: []string{}}}
	c := recognize(t, d)
	if c.Questions[0].Answer.State != "unknown" || !strings.Contains(string(c.Questions[0].Options[0].Content), "underline") {
		t.Fatal("pronunciation underline treated as key or lost")
	}
	c, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{d}, domain.RecognitionProfile{Version: "auto-v1", AnswerMark: "underline", ConfirmedBy: "teacher"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Questions[0].Answer.State != "known" || strings.Contains(string(c.Questions[0].Options[0].Content), "underline") {
		t.Fatal("confirmed key-only mark leaked into learner content")
	}
}

func TestGeneratedLabelsSeparateFromSourceOffsetsAndUnassignedTextRemainsVisible(t *testing.T) {
	d := doc("exam", "Choose", "yes", "no", "passage without a confirmed attachment")
	d.Blocks[0].Numbering = "1."
	d.Blocks[1].Numbering = "A."
	d.Blocks[2].Numbering = "B."
	c := recognize(t, d)
	if len(c.Questions) != 1 || len(c.Questions[0].Options) != 2 || plain(t, c.Questions[0].Prompt) != "Choose" {
		t.Fatal("automatic numbering not recognized")
	}
	if !hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("loose passage silently attached to last option")
	}
}

func TestUnsafeTextAndUnsupportedInteractionsNeverBecomeOrdinaryChoiceContent(t *testing.T) {
	d := doc("exam", "1. Match the columns", "private teacher note")
	d.Blocks[1].Safe = false
	d.Blocks[1].Reasons = []string{"HIDDEN_TEXT_REQUIRES_REVIEW"}
	c := recognize(t, d)
	if c.Questions[0].Type != "unknown" || !hasIssue(c, "UNRECOGNIZED_INTERACTION") || !hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("unsupported or private content accepted")
	}
	if strings.Contains(string(c.Questions[0].Prompt), "private teacher") {
		t.Fatal("private source text leaked")
	}
}

func TestAnswerTablesUseExplicitGridPairs(t *testing.T) {
	exam := doc("exam", "1. First A. yes B. no", "2. Second A. yes B. no")
	key := doc("answer_key", "1", "2", "B", "A")
	for i := range key.Blocks {
		key.Blocks[i].TableID = "table"
		key.Blocks[i].Row = i / 2
		key.Blocks[i].Column = i % 2
	}
	c := recognize(t, exam, key)
	if c.Questions[0].Answer.State != "known" || c.Questions[1].Answer.State != "known" {
		t.Fatalf("table key failed: %+v", c.Keys)
	}
	if hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("answer-table evidence unaccounted")
	}
}

func TestAnswerTextMustBeALabelNotTheFirstLetterOfAWord(t *testing.T) {
	c := recognize(t, doc("exam", "1. First A. yes B. no"), doc("answer_key", "1. Banana"))
	if len(c.Keys) != 0 || c.Questions[0].Answer.State != "unknown" {
		t.Fatal("ordinary word became answer B")
	}
}

func TestAmbiguousTableAnswerCannotServeTwoNumbers(t *testing.T) {
	exam := doc("exam", "1. First A. yes B. no", "2. Second A. yes B. no")
	key := doc("answer_key", "1", "2", "B")
	positions := [][2]int{{0, 1}, {1, 0}, {1, 1}}
	for i := range key.Blocks {
		key.Blocks[i].TableID = "table"
		key.Blocks[i].Row = positions[i][0]
		key.Blocks[i].Column = positions[i][1]
	}
	c := recognize(t, exam, key)
	if len(c.Keys) != 0 || !hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("ambiguous table geometry guessed answer sharing")
	}
}

func TestUndelimitedQuestionHeadingAndContinuationAreReviewable(t *testing.T) {
	c := recognize(t, doc("exam", "Question 1 Choose the closest meaning.", "I will take up golf.", "A. I will begin to play.", "B. I will stop playing."))
	if len(c.Questions) != 1 || len(c.Questions[0].Options) != 2 || !strings.Contains(plain(t, c.Questions[0].Prompt), "I will take up golf.") {
		t.Fatal("common question heading or continuation lost")
	}
	if !hasIssue(c, "CONTEXT_ATTACHMENT_INFERRED") {
		t.Fatal("inferred continuation presented as certain")
	}
}

func TestCollectedPapersDoNotCrossMatchTheirCompanionAnswers(t *testing.T) {
	exam := doc("exam", "Đề số 1 ôn tập", "Question 1 Choose A. one B. two", "Đề số 2 ôn tập", "Question 1 Choose A. one B. two")
	keys := doc("answer_key", "Đáp án đề số 1 ôn tập", "Question 1. A", "Đáp án đề số 2 ôn tập", "Question 1. B")
	c := recognize(t, exam, keys)
	if len(c.Questions) != 2 || len(c.Keys) != 2 || c.Questions[0].Answer.State != "known" || c.Questions[1].Answer.State != "known" {
		t.Fatalf("collected paper association: %+v", c.Keys)
	}
	if c.Questions[0].Answer.OptionIDs[0] != c.Questions[0].Options[0].ID || c.Questions[1].Answer.OptionIDs[0] != c.Questions[1].Options[1].ID {
		t.Fatal("cross-paper key selected")
	}
}

func TestNonbreakingWordSpacesKeepAnswerLabelsAndSourceCoordinates(t *testing.T) {
	exam := doc("exam", "Question\u00a01\u00a0Choose A. có B. không")
	keys := doc("answer_key", "Question\u00a01.\u00a0B")
	c := recognize(t, exam, keys)
	if len(c.Questions) != 1 || c.Questions[0].Answer.State != "known" || c.Questions[0].Answer.OptionIDs[0] != c.Questions[0].Options[1].ID {
		t.Fatal("Word nonbreaking spaces lost key association")
	}
	if hasIssue(c, "UNASSIGNED_SOURCE_BLOCK") {
		t.Fatal("nonbreaking key label prefix not covered")
	}
}

func TestRepeatedOptionMarkersAreBoundedBeforeQuadraticDuplicateHandling(t *testing.T) {
	_, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{doc("exam", "1. Choose "+strings.Repeat("A. same B. same ", 200))}, domain.RecognitionProfile{Version: "auto-v1"})
	if err == nil {
		t.Fatal("unbounded option list accepted")
	}
}

func FuzzRecognizerKeepsAllMeaningfulBlocksAndSourceBoundRanges(f *testing.F) {
	for _, s := range []string{"Câu 1. nghé A. đầu B. cuối Đáp án: B", "1. Same A. yes B. no\nANSWER KEY\n1. B", "Part I\n1. Same\nA. yes\nB. no", "A. 🙂 B. α"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 64000 || !utf8.ValidString(text) {
			return
		}
		d := doc("exam", strings.Split(text, "\n")...)
		c, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{d}, domain.RecognitionProfile{Version: "auto-v1"})
		if err != nil {
			return
		}
		if len(c.Coverage) != len(d.Blocks) {
			t.Fatal("source block omitted")
		}
		blocks := map[string]domain.EvidenceBlock{}
		for _, b := range d.Blocks {
			blocks[b.ID] = b
		}
		for _, entry := range c.Coverage {
			b := blocks[entry.BlockID]
			for _, use := range entry.Uses {
				if use.Start < 0 || use.End < use.Start || use.End > utf8.RuneCountInString(b.Text) {
					t.Fatal("invalid source coordinates")
				}
			}
		}
		for _, q := range c.Questions {
			plain(t, q.Prompt)
			for _, o := range q.Options {
				plain(t, o.Content)
			}
		}
	})
}
