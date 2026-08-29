package tests

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	Draft     Status = "draft"
	Published Status = "published"
	Archived  Status = "archived"
)

func (s Status) valid() bool {
	switch s {
	case Draft, Published, Archived:
		return true
	}
	return false
}

var (
	ErrNotFound        = errors.New("tests: not found")
	ErrStaleWrite      = errors.New("tests: edited elsewhere since the version read")
	ErrBadCursor       = errors.New("tests: malformed cursor")
	ErrUnknownQuestion = errors.New("tests: outline references a question that does not exist")
)

// Section is one part of the draft outline, with its questions in order.
type Section struct {
	ID           string
	Ordinal      int
	Title        string
	Instructions *string
	QuestionIDs  []string
}

// Test is a test with its DRAFT outline. Published content lives in versions.
type Test struct {
	ID             string
	Title          string
	Description    *string
	Status         Status
	CurrentVersion int
	TotalPoints    string
	QuestionCount  int
	Sections       []Section
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// SectionInput is one section of a whole-outline write. An empty ID creates.
type SectionInput struct {
	ID           string
	Title        string
	Instructions *string
	QuestionIDs  []string
}

// UpdateInput is the autosave body. Nil fields are left alone; a non-nil
// Sections replaces the whole outline.
type UpdateInput struct {
	ExpectedUpdatedAt time.Time
	Title             *string
	Description       *string
	SetDescription    bool
	Status            *Status
	Sections          []SectionInput
	SetSections       bool
}

// FieldError names the field a rule failed on.
type FieldError struct {
	Field   string
	Message string
}

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "tests: " + strings.Join(parts, "; ")
}

// Validate checks the parts of an outline write a schema cannot express.
func (in UpdateInput) Validate() error {
	var errs []FieldError
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }

	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		add("title", "Tên đề không được để trống.")
	}
	if in.Status != nil && !in.Status.valid() {
		add("status", "Trạng thái không hợp lệ.")
	}
	// Publishing is its own endpoint: it has to snapshot a version, and
	// current_version = 0 with status published is refused by the database.
	if in.Status != nil && *in.Status == Published {
		add("status", "Dùng thao tác xuất bản để chuyển đề sang trạng thái published.")
	}

	for i, s := range in.Sections {
		if strings.TrimSpace(s.Title) == "" {
			add(sectionField(i, "title"), "Tên phần không được để trống.")
		}
		seen := make(map[string]bool, len(s.QuestionIDs))
		for _, id := range s.QuestionIDs {
			if seen[id] {
				// The database refuses this too; saying which section it was
				// in is what the teacher needs.
				add(sectionField(i, "questionIds"), "Một câu hỏi chỉ được xuất hiện một lần trong phần.")
				break
			}
			seen[id] = true
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

func sectionField(i int, field string) string {
	return "sections[" + strconv.Itoa(i) + "]." + field
}
