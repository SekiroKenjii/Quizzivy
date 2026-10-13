package domain

import (
	"encoding/json"
	"time"
)

// ProfilePatch names supplied profile fields and preserves explicit nullable clears.
type ProfilePatch struct {
	FullName       *string
	DisplayNameSet bool
	DisplayName    *string
	PhoneSet       bool
	Phone          *string
	Locale         *string
	TimeZone       *string
}

// ProfileRecord is one self profile mutation and its audit metadata.
type ProfileRecord struct {
	UserID    string
	Patch     ProfilePatch
	Now       time.Time
	IP        *string
	UserAgent *string
}

// AvatarRecord is one write of the caller's photo and its audit metadata. A nil
// Key clears the photo.
type AvatarRecord struct {
	UserID    string
	Key       *string
	Now       time.Time
	IP        *string
	UserAgent *string
}

// AvatarWrite is the caller as stored after the write, with the key the photo
// had before it, which the caller of the write deletes from the object store.
type AvatarWrite struct {
	User        User
	PreviousKey *string
}

// Preferences is the account's stored top-level preferences without materialized defaults.
type Preferences struct {
	AI                 *AIPreferences      `json:"ai,omitempty"`
	AssignmentDefaults *AssignmentDefaults `json:"assignmentDefaults,omitempty"`
	CompactTables      *bool               `json:"compactTables,omitempty"`
	LargerTestText     *bool               `json:"largerTestText,omitempty"`
	Theme              *string             `json:"theme,omitempty"`
}

// AIPreferences are the account's choices about the AI assistant. An omitted field uses the client's default, and the server's where it reads one: reading student answers is on unless it is false.
type AIPreferences struct {
	Language           *string `json:"language,omitempty"`
	TagDrafts          *bool   `json:"tagDrafts,omitempty"`
	ReadStudentAnswers *bool   `json:"readStudentAnswers,omitempty"`
	ReadImports        *bool   `json:"readImports,omitempty"`
}

// AssignmentDefaults supplies optional values for future assignment forms.
type AssignmentDefaults struct {
	BlockCopyPaste    *bool `json:"blockCopyPaste,omitempty"`
	DurationMinutes   *int  `json:"durationMinutes,omitempty"`
	RequireFullscreen *bool `json:"requireFullscreen,omitempty"`
	ShowScore         *bool `json:"showScore,omitempty"`
	ShuffleQuestions  *bool `json:"shuffleQuestions,omitempty"`
}

// PreferencesRecord carries one atomic top-level merge and its audit metadata.
type PreferencesRecord struct {
	UserID    string
	Patch     json.RawMessage
	Now       time.Time
	IP        *string
	UserAgent *string
}
