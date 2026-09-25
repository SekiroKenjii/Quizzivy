package domain

import (
	"encoding/json"
	questions "quizzivy/internal/modules/questions/domain"
)

// PreviewPaper is the complete frozen learner-safe projection of a selected version.
type PreviewPaper struct {
	Version   int
	Sections  []PreviewSection
	Questions []PreviewQuestion
	Groups    []PreviewGroup
}

// PreviewSection preserves the authored section order and instructions.
type PreviewSection struct {
	ID           string
	Title        string
	Instructions *string
}

// PreviewGroup exposes shared context without grading data or recording transcripts.
type PreviewGroup struct {
	ID           string
	SectionID    string
	Title        string
	Instructions json.RawMessage
	QuestionIDs  []string
	Stimuli      []GroupStimulus
	Recordings   []PreviewRecording
	AssetIDs     []string
}

// PreviewRecording identifies one shared playback scope without its transcript.
type PreviewRecording struct {
	ID      string
	AssetID string
	Policy  questions.AudioPolicy
}
