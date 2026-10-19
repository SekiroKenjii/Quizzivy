package domain

// Passage is the shared material of one stimulus of a test's draft, as plain text.
type Passage struct {
	Title string
	Text  string
}

// SectionQuestion is the type and prompt of one question already in a section, standalone or a group's member.
type SectionQuestion struct {
	Type   string
	Prompt string
}

// Section is one section of a test's draft as the assistant reads it to write more questions for it: its title, its instructions, and the questions already in it in the order the builder shows them, without their keys.
type Section struct {
	Title        string
	Instructions string
	Questions    []SectionQuestion
}

// Option is one option of a choice question.
type Option struct {
	ID      string
	Text    string
	Correct bool
}

// Blank is one blank of a fill-in question and the answers it accepts.
type Blank struct {
	ID       string
	Accepted []string
}

// PaperQuestion is one question of a test's draft with everything the assistant may read of it and nothing about a student. Number is its place in the draft, from 1, counting a group's members one by one in the order the builder shows them. GroupID names the group the question is a member of, and is empty for a standalone question. Passage is the plain text of the shared material of its group, empty for a question with none. OrderFixed says its options may not be reordered because a group's labels depend on them. Gapped says it answers a gap in its group's material, so its prompt belongs to the gap and is not reworded. Rich says the prompt, an option or the explanation holds more than plain paragraphs, so a rewrite of its text would lose what the text carries and is not proposed.
type PaperQuestion struct {
	ID          string
	Number      int
	GroupID     string
	Type        string
	Prompt      string
	Explanation string
	Sample      string
	Passage     string
	OrderFixed  bool
	Gapped      bool
	Rich        bool
	Options     []Option
	Blanks      []Blank
}

// Paper is a test's draft read for the assistant.
type Paper struct {
	Questions []PaperQuestion
}

// Question is one stored bank question, read for an explanation or for wrong options.
type Question struct {
	ID          string
	Type        string
	Prompt      string
	Explanation string
	Sample      string
	Options     []Option
	Blanks      []Blank
}

// BankQuestion is one of the caller's own bank questions, read for the clean-up. UsedInTests counts the draft outlines that use it.
type BankQuestion struct {
	ID          string
	Type        string
	Prompt      string
	Level       string
	Tags        []string
	UsedInTests int
}

// Bank is the caller's own questions, newest first, at most the number asked for; Truncated says the bank holds more.
type Bank struct {
	Questions []BankQuestion
	Truncated bool
}

// GradedAnswer is one student answer and the question it answers, without anything that says who gave it: no name, no id, no class, no attempt. Points is the question's points, Accepted the answers it accepts or the sample answer, Answer the student's text, and Manual whether the answer waits for a teacher's mark; an answer the grader marked itself does not.
type GradedAnswer struct {
	Type     string
	Prompt   string
	Points   float64
	Accepted []string
	Answer   string
	Manual   bool
}

// WrongAnswer is a distinct wrong answer text and how many papers gave it.
type WrongAnswer struct {
	Text  string
	Count int
}

// ResultQuestion is one question of an assignment's paper with how the papers did on it. CorrectRate is nil when no paper has a mark for it. Key is the answer key in words, or option letters for a choice question.
type ResultQuestion struct {
	Number      int
	Type        string
	Prompt      string
	Key         string
	CorrectRate *float64
	Wrong       []WrongAnswer
}

// Results is how an assignment's handed-in papers did, by question in paper order.
type Results struct {
	HandedIn  int
	Questions []ResultQuestion
}

// Preferences is a teacher's choices about the assistant. Language is the language they chose, or the language of their interface when they chose none; ReadStudentAnswers is on unless they turned it off.
type Preferences struct {
	Language           Language
	ReadStudentAnswers bool
}
