package domain

import (
	"quizzivy/internal/shared/validation"
	"strconv"
	"strings"
	"time"
)

// ListInput selects a page of tests.
type ListInput struct {
	Status *Status
	// Tags filter by the tags of the questions a test CONTAINS.
	Tags  []string
	Query string
	Page  int
	Limit int
}

// CreateInput is a new empty draft.
type CreateInput struct {
	Title       string
	Description *string
	ActorID     string
	Now         time.Time
	IP          string
	UserAgent   string
}

// UpdateRequest is one autosave.
type UpdateRequest struct {
	ID        string
	Input     UpdateInput
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
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
	GroupOutline      bool
}

// SectionInput is one section of a whole-outline write. An empty ID creates.
type SectionInput struct {
	ID           string
	Title        string
	Instructions *string
	QuestionIDs  []string
	Units        []SectionUnit
	SetUnits     bool
}

// DuplicateInput copies a test's draft structure.
type DuplicateInput struct {
	ID        string
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}

type Request struct {
	ID        string
	ActorID   string
	IP        string
	UserAgent string
}

func sectionField(i int, field string) string {
	return "sections[" + strconv.Itoa(i) + "]." + field
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
				add(sectionField(i, "questionIds"), "Một câu hỏi chỉ được xuất hiện một lần trong phần.")
				break
			}
			seen[id] = true
		}
	}
	errs = append(errs, validateMixedOutline(in)...)

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

type (
	FieldError      = validation.Field
	ValidationError = validation.Error
)

// VersionRequest identifies a snapshot and the test revision observed by the teacher.
type VersionRequest struct {
	Request
	Version           int
	ExpectedUpdatedAt time.Time
}
