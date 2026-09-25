package domain

import (
	"encoding/json"
	questions "quizzivy/internal/modules/questions/domain"
)

// MaxGroupBytes and companion limits bound one complete context graph, including its resolved questions.
const (
	MaxGroupBytes      = 4 << 20
	MaxGroupMembers    = 200
	MaxGroupStimuli    = 16
	MaxGroupRecordings = 16
	MaxGroupTitle      = 200
	MaxGroupTranscript = 100000
)

const (
	groupCopyIdentity = "group_copy_identity"
	groupAsset        = "group_asset"
	groupContent      = "group_content"
	groupLimits       = "group_limits"
	groupIdentity     = "group_identity"
	groupRecording    = "group_recording"
	fixedOptionOrder  = "fixed"
	groupAudio        = "audio"
)

// QuestionGroup is an independent ordered draft context; existing sections are not groups.
type QuestionGroup struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Instructions json.RawMessage  `json:"instructions,omitempty"`
	Members      []GroupMember    `json:"members"`
	Stimuli      []GroupStimulus  `json:"stimuli"`
	Recordings   []GroupRecording `json:"recordings"`
}

// GroupMember preserves authored question order and declares option-label dependencies.
type GroupMember struct {
	QuestionID  string `json:"questionId"`
	OptionOrder string `json:"optionOrder"`
}

// GroupStimulus owns one material and its explicit response bindings.
type GroupStimulus struct {
	ID      string            `json:"id"`
	Title   string            `json:"title"`
	Content json.RawMessage   `json:"content"`
	Gaps    []GroupGapBinding `json:"gaps"`
}

// GroupGapBinding targets a choice question or a stable rich-blank gap, never a display label or answer-row ID.
type GroupGapBinding struct {
	Kind       string  `json:"kind"`
	GapID      string  `json:"gapId"`
	QuestionID string  `json:"questionId"`
	BlankGapID *string `json:"blankGapId,omitempty"`
}

// GroupRecording identifies one shared playback allowance; it is admin-only because it holds a transcript.
type GroupRecording struct {
	ID         string                `json:"id"`
	AssetID    string                `json:"assetId"`
	Policy     questions.AudioPolicy `json:"policy"`
	Transcript *string               `json:"transcript,omitempty"`
}

// GroupQuestion resolves one member for validation and independent copying; asset kind is repository-verified.
type GroupQuestion struct {
	ID             string
	Input          questions.Input
	MediaAssetKind *string
}

// GroupBundle holds exactly the group's members, not a live dependency on its source test or bank item.
type GroupBundle struct {
	Group     QuestionGroup
	Questions []GroupQuestion
}

// GroupError identifies an invalid graph without copying source text or keys into diagnostics.
type GroupError struct {
	Rule       string
	StimulusID string
	QuestionID string
	GapID      string
}

func (e *GroupError) Error() string { return "invalid question group: " + e.Rule }
