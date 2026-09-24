package domain

import "encoding/json"

// CandidateVersion identifies reviewable machine output, which is never sufficient authorization to create or publish an assessment.
const CandidateVersion = "word-candidate-v1"

// Candidate is a complete machine proposal with explicit uncertainty and block coverage, separate from teacher review revisions.
type Candidate struct {
	Version           string              `json:"version"`
	RecognizerVersion string              `json:"recognizerVersion"`
	Profile           RecognitionProfile  `json:"profile"`
	Sources           []string            `json:"sources"`
	Sections          []CandidateSection  `json:"sections"`
	Questions         []CandidateQuestion `json:"questions"`
	Keys              []CandidateKey      `json:"keys"`
	Coverage          []BlockCoverage     `json:"coverage"`
	Issues            []CandidateIssue    `json:"issues"`
}

// RecognitionProfile is declarative; a marking convention is inactive unless a teacher explicitly confirmed it.
type RecognitionProfile struct {
	Version     string `json:"version"`
	AnswerMark  string `json:"answerMark,omitempty"`
	ConfirmedBy string `json:"confirmedBy,omitempty"`
}

type CandidateSection struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Title    string      `json:"title"`
	Evidence []SourceRef `json:"evidence"`
}

type CandidateQuestion struct {
	ID        string            `json:"id"`
	SectionID string            `json:"sectionId"`
	Label     string            `json:"label"`
	Type      string            `json:"type"`
	Prompt    json.RawMessage   `json:"prompt"`
	Options   []CandidateOption `json:"options"`
	Answer    CandidateAnswer   `json:"answer"`
	Points    string            `json:"points"`
	Fields    []FieldEvidence   `json:"fields"`
}

type CandidateOption struct {
	ID      string          `json:"id"`
	Label   string          `json:"label"`
	Content json.RawMessage `json:"content"`
}

// CandidateAnswer distinguishes an absent key from confirmed options and incompatible evidence; no false-option array stands in for an unknown answer.
type CandidateAnswer struct {
	State     string      `json:"state"`
	OptionIDs []string    `json:"optionIds"`
	Evidence  []SourceRef `json:"evidence"`
}

// CandidateKey retains unmatched and conflicting explicit keys for review instead of assigning them by printed number alone.
type CandidateKey struct {
	Origin        string      `json:"origin"`
	ID            string      `json:"id"`
	SectionLabel  string      `json:"sectionLabel"`
	QuestionLabel string      `json:"questionLabel"`
	QuestionID    string      `json:"questionId,omitempty"`
	OptionLabels  []string    `json:"optionLabels"`
	Evidence      []SourceRef `json:"evidence"`
}

type FieldEvidence struct {
	Field  string      `json:"field"`
	Origin string      `json:"origin"`
	Refs   []SourceRef `json:"refs"`
}

type CandidateIssue struct {
	ID       string      `json:"id"`
	Code     string      `json:"code"`
	Severity string      `json:"severity"`
	EntityID string      `json:"entityId"`
	Field    string      `json:"field"`
	Evidence []SourceRef `json:"evidence"`
}

type BlockCoverage struct {
	SourceID string        `json:"sourceId"`
	BlockID  string        `json:"blockId"`
	Uses     []CoverageUse `json:"uses"`
}

// CoverageUse accounts for an exact source range assigned to content, structure or private answer evidence; partial ranges do not cover the whole block.
type CoverageUse struct {
	EntityID       string `json:"entityId"`
	Field          string `json:"field"`
	Start          int    `json:"start"`
	End            int    `json:"end"`
	GeneratedLabel bool   `json:"generatedLabel,omitempty"`
}
