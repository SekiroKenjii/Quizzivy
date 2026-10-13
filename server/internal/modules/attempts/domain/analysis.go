package domain

import (
	"errors"
	"time"
)

// MaxExportRows is the most rows one results export holds.
const MaxExportRows = 20000

// MaxExportAssignments is the most assignments one results export names.
const MaxExportAssignments = 50

// ErrExportTooLarge means the assignments named hold more than MaxExportRows rows.
var ErrExportTooLarge = errors.New("attempts: the export holds too many rows")

// ItemAnalysis is how the students did on each question of an assignment:
// HandedIn papers, one per student the reader reaches (their latest handed-in
// attempt that is not voided), and a figure per question, hardest first.
type ItemAnalysis struct {
	HandedIn int
	Items    []AnalysisItem
}

// AnalysisItem is one question's figures. Answered counts the papers with an
// answer that says something. CorrectRate is the papers that earned the full
// points over the papers with a mark for the question: a question left
// unanswered counts against it, a manually marked answer not yet marked counts
// for neither side, and it is nil when no paper has a mark.
type AnalysisItem struct {
	QuestionID    string
	Number        int
	Type          string
	PromptExcerpt string
	Answered      int
	CorrectRate   *float64
}

// ResultRow is one student's line of a results export: the attempt that stands
// for them on the assignment's monitor, or none when they have not started.
// Earned and Total are nil until every manually marked answer has a mark.
type ResultRow struct {
	AssignmentTitle string
	Version         int
	Classes         string
	StudentName     string
	Email           string
	State           string
	Earned          *float64
	Total           *float64
	SubmittedAt     *time.Time
	FocusLoss       *int
	Flagged         bool
}

// ResultsExport is the rows of a results export and the calendar zone its
// times are written in.
type ResultsExport struct {
	Zone string
	Rows []ResultRow
}

// MaxWrongAnswers is how many distinct wrong answer texts a question reports.
const MaxWrongAnswers = 8

// WrongAnswers is, for each question of an assignment's pinned version in paper order, its key and the distinct answer texts that did not earn full points with how many papers gave each: HandedIn papers, one per student the reader reaches (their latest handed-in attempt that is not voided), as in ItemAnalysis. An answer that says nothing, and one not yet marked, is not a wrong answer. The text of a text answer is lower case with its spaces collapsed; a choice answer is its option letters; a true/false answer is True or False; a fill-in is its blanks' values in order. Nothing names a student or an attempt.
type WrongAnswers struct {
	HandedIn  int
	Questions []QuestionWrongAnswers
}

// QuestionWrongAnswers is one question's key in words (option letters for a choice question) and its most frequent wrong answers, the most frequent first.
type QuestionWrongAnswers struct {
	QuestionID string
	Number     int
	Type       string
	Key        string
	Wrong      []WrongAnswer
}

// WrongAnswer is a distinct wrong answer text and the papers that gave it.
type WrongAnswer struct {
	Text  string
	Count int
}
