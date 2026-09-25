package domain

import "encoding/json"

// DraftVersion identifies the reviewable assessment graph schema.
const DraftVersion = "word-draft-v1"

// UnsupportedType marks source content no Quizzivy interaction can represent; it must be excluded or retyped.
const UnsupportedType = "unsupported"

// RecognitionProfile carries teacher decisions that change how evidence is read; the zero value is automatic.
type RecognitionProfile struct {
	KeyPaper int `json:"keyPaper,omitempty"`
}

// Origin says where a field's value came from, so review can tell source facts from guesses and edits.
type Origin string

const (
	SourceExplicit    Origin = "source_explicit"
	InferredStructure Origin = "inferred_structure"
	Defaulted         Origin = "defaulted"
	TeacherEntered    Origin = "teacher_entered"
)

// AnswerState keeps an unknown key distinct from a key whose evidence disagrees or cannot be used.
type AnswerState string

const (
	AnswerKnown    AnswerState = "known"
	AnswerUnknown  AnswerState = "unknown"
	AnswerConflict AnswerState = "conflict"
)

// Draft is one import's reviewable exam: sections of standalone questions and shared-context groups.
// Notices are source observations a teacher must acknowledge; validation findings are derived, never stored.
type Draft struct {
	Version      string         `json:"version"`
	Title        string         `json:"title"`
	Sections     []DraftSection `json:"sections"`
	Notices      []Finding      `json:"notices"`
	Acknowledged []string       `json:"acknowledged"`
}

type DraftSection struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Instructions string      `json:"instructions,omitempty"`
	Origin       Origin      `json:"origin"`
	Items        []DraftItem `json:"items"`
	Source       []SourceRef `json:"source"`
}

// DraftItem holds exactly one of Question or Group.
type DraftItem struct {
	Question *DraftQuestion `json:"question,omitempty"`
	Group    *DraftGroup    `json:"group,omitempty"`
}

// DraftGroup is a shared passage whose gaps are bound to member questions by stable IDs, never by printed numbers.
type DraftGroup struct {
	ID           string          `json:"id"`
	Label        string          `json:"label,omitempty"`
	Instructions string          `json:"instructions,omitempty"`
	Stimulus     json.RawMessage `json:"stimulus,omitempty"`
	Gaps         []GapLink       `json:"gaps"`
	Questions    []DraftQuestion `json:"questions"`
	Source       []SourceRef     `json:"source"`
}

// GapLink binds a passage gap to a choice question, or to one blank of a fill-blank question when BlankGapID is set.
type GapLink struct {
	GapID      string `json:"gapId"`
	QuestionID string `json:"questionId"`
	BlankGapID string `json:"blankGapId,omitempty"`
}

type DraftQuestion struct {
	ID       string          `json:"id"`
	Label    string          `json:"label"`
	Task     string          `json:"task,omitempty"`
	Type     string          `json:"type"`
	Prompt   json.RawMessage `json:"prompt"`
	Options  []DraftOption   `json:"options"`
	Blanks   []DraftBlank    `json:"blanks"`
	Answer   DraftAnswer     `json:"answer"`
	Points   string          `json:"points"`
	Excluded *Exclusion      `json:"excluded,omitempty"`
	Origins  Origins         `json:"origins"`
	Source   []SourceRef     `json:"source"`
}

type DraftOption struct {
	ID      string          `json:"id"`
	Label   string          `json:"label"`
	Content json.RawMessage `json:"content"`
}

type DraftBlank struct {
	GapID         string   `json:"gapId"`
	Label         string   `json:"label,omitempty"`
	Accepted      []string `json:"accepted"`
	CaseSensitive bool     `json:"caseSensitive"`
}

// DraftAnswer holds option IDs for choice types and Text for a short answer; fill-blank keys live on blanks.
// Candidates retains every explicit source value when State is conflict.
type DraftAnswer struct {
	State      AnswerState `json:"state"`
	OptionIDs  []string    `json:"optionIds"`
	Text       string      `json:"text,omitempty"`
	Evidence   []SourceRef `json:"evidence"`
	Candidates []KeyValue  `json:"candidates,omitempty"`
}

type KeyValue struct {
	Value    string      `json:"value"`
	Evidence []SourceRef `json:"evidence"`
}

type Exclusion struct {
	Reason string `json:"reason"`
}

type Origins struct {
	Type    Origin `json:"type"`
	Prompt  Origin `json:"prompt"`
	Options Origin `json:"options"`
	Answer  Origin `json:"answer"`
	Points  Origin `json:"points"`
}

// Questions returns every question in document order, including group members.
func (d Draft) Questions() []*DraftQuestion {
	var out []*DraftQuestion
	for s := range d.Sections {
		for i := range d.Sections[s].Items {
			item := &d.Sections[s].Items[i]
			if item.Question != nil {
				out = append(out, item.Question)
			}
			if item.Group != nil {
				for q := range item.Group.Questions {
					out = append(out, &item.Group.Questions[q])
				}
			}
		}
	}
	return out
}
