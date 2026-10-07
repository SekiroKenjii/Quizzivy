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

// Preferences is the account's stored top-level preferences without materialized defaults.
type Preferences struct {
	AssignmentDefaults *AssignmentDefaults `json:"assignmentDefaults,omitempty"`
	CompactTables      *bool               `json:"compactTables,omitempty"`
	LargerTestText     *bool               `json:"largerTestText,omitempty"`
	Theme              *string             `json:"theme,omitempty"`
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
