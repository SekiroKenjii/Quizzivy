package domain

import (
	"bytes"
	"encoding/json"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/content"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type groupValidator struct {
	ids         map[string]bool
	questions   map[string]GroupQuestion
	targets     map[string]bool
	assets      map[string]string
	sharedAudio map[string]bool
}

// Validate checks the entire context graph, including question invariants; empty draft groups are valid.
func (b GroupBundle) Validate() error {
	g := b.Group
	if len(g.Members) > MaxGroupMembers || len(g.Stimuli) > MaxGroupStimuli || len(g.Recordings) > MaxGroupRecordings || len(b.Questions) != len(g.Members) {
		return &GroupError{Rule: groupLimits}
	}
	if !validGroupText(g.Title, MaxGroupTitle, true) || !validGroupInstructions(g.Instructions) {
		return &GroupError{Rule: groupContent}
	}
	if err := validateGroupSize(b); err != nil {
		return err
	}
	v := groupValidator{ids: map[string]bool{}, questions: map[string]GroupQuestion{}, targets: map[string]bool{}, assets: map[string]string{}, sharedAudio: map[string]bool{}}
	if !v.claimID(g.ID) {
		return &GroupError{Rule: groupIdentity}
	}
	if err := v.members(g.Members, b.Questions); err != nil {
		return err
	}
	for _, stimulus := range g.Stimuli {
		if err := v.stimulus(stimulus); err != nil {
			return err
		}
	}
	return v.recordings(g.Recordings)
}

// ValidateForPublish additionally rejects empty groups and incompatible option shuffling.
func (b GroupBundle) ValidateForPublish(shuffleOptions bool) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if len(b.Group.Members) == 0 {
		return &GroupError{Rule: "group_empty"}
	}
	if shuffleOptions {
		for _, member := range b.Group.Members {
			if member.OptionOrder == fixedOptionOrder {
				return &GroupError{Rule: "group_option_order", QuestionID: member.QuestionID}
			}
		}
	}
	return nil
}

func (v *groupValidator) members(members []GroupMember, questions []GroupQuestion) error {
	for _, question := range questions {
		if err := v.memberQuestion(question); err != nil {
			return err
		}
	}
	seen := make(map[string]bool)
	for _, member := range members {
		id := strings.ToLower(member.QuestionID)
		if _, ok := v.questions[id]; !ok || seen[id] {
			return &GroupError{Rule: "group_membership", QuestionID: member.QuestionID}
		}
		if (member.OptionOrder != "shuffle" && member.OptionOrder != fixedOptionOrder) || (member.OptionOrder == fixedOptionOrder && !v.questions[id].Input.Type.IsChoice()) {
			return &GroupError{Rule: "group_option_order", QuestionID: member.QuestionID}
		}
		seen[id] = true
	}
	return nil
}

func (v *groupValidator) memberQuestion(question GroupQuestion) error {
	if !v.claimID(question.ID) {
		return &GroupError{Rule: groupIdentity, QuestionID: question.ID}
	}
	if !v.questionChildren(question.Input) {
		return &GroupError{Rule: groupIdentity, QuestionID: question.ID}
	}
	if question.Input.Validate(question.MediaAssetKind) != nil || !validGroupQuestionStrings(question.Input) {
		return &GroupError{Rule: "group_question", QuestionID: question.ID}
	}
	if !validMemberAsset(question) {
		return &GroupError{Rule: groupAsset, QuestionID: question.ID}
	}
	if question.Input.MediaAssetID != nil && !v.asset(*question.Input.MediaAssetID, *question.MediaAssetKind) {
		return &GroupError{Rule: groupAsset, QuestionID: question.ID}
	}
	v.questions[strings.ToLower(question.ID)] = question
	return nil
}

func (v *groupValidator) questionChildren(in questions.Input) bool {
	for _, option := range in.Options {
		if option.ID != nil && !v.claimID(*option.ID) {
			return false
		}
	}
	for _, blank := range in.Blanks {
		if blank.ID != nil && !v.claimID(*blank.ID) {
			return false
		}
	}
	return true
}

func (v *groupValidator) asset(id, kind string) bool {
	key := strings.ToLower(id)
	if previous, exists := v.assets[key]; exists && previous != kind {
		return false
	}
	v.assets[key] = kind
	return true
}

func validMemberAsset(question GroupQuestion) bool {
	if question.Input.MediaAssetID == nil {
		return question.MediaAssetKind == nil
	}
	return groupUUID(*question.Input.MediaAssetID) && question.MediaAssetKind != nil && (*question.MediaAssetKind == groupAudio || *question.MediaAssetKind == "image")
}

func validGroupQuestionStrings(in questions.Input) bool {
	values := []string{in.Prompt, in.Points}
	for _, value := range []*string{in.Transcript, in.Explanation, in.SampleAnswer} {
		if value != nil {
			values = append(values, *value)
		}
	}
	values = append(values, in.Tags...)
	for _, option := range in.Options {
		values = append(values, option.Text)
	}
	for _, blank := range in.Blanks {
		values = append(values, blank.AcceptedAnswers...)
	}
	for _, value := range values {
		if !validGroupText(value, MaxGroupBytes, false) {
			return false
		}
	}
	return true
}

func (v *groupValidator) stimulus(stimulus GroupStimulus) error {
	if !v.claimID(stimulus.ID) {
		return &GroupError{Rule: groupIdentity, StimulusID: stimulus.ID}
	}
	if !validGroupText(stimulus.Title, MaxGroupTitle, true) {
		return &GroupError{Rule: groupContent, StimulusID: stimulus.ID}
	}
	document, err := content.Parse(stimulus.Content)
	if err != nil {
		return &GroupError{Rule: groupContent, StimulusID: stimulus.ID}
	}
	for _, asset := range document.Assets() {
		id := strings.ToLower(asset.ID)
		if !v.asset(id, asset.Kind) {
			return &GroupError{Rule: groupAsset, StimulusID: stimulus.ID}
		}
		if asset.Kind == groupAudio {
			v.sharedAudio[id] = true
		}
	}
	expected := make(map[string]bool)
	for _, id := range document.GapIDs() {
		expected[id] = true
	}
	for _, gap := range stimulus.Gaps {
		if !expected[gap.GapID] || !v.gapTarget(gap) {
			return &GroupError{Rule: "group_gap", StimulusID: stimulus.ID, QuestionID: gap.QuestionID, GapID: gap.GapID}
		}
		delete(expected, gap.GapID)
	}
	if len(expected) != 0 {
		return &GroupError{Rule: "group_gap", StimulusID: stimulus.ID}
	}
	return nil
}

func (v *groupValidator) gapTarget(gap GroupGapBinding) bool {
	id := strings.ToLower(gap.QuestionID)
	question, exists := v.questions[id]
	if !exists {
		return false
	}
	key := id
	switch gap.Kind {
	case "question":
		if gap.BlankGapID != nil || !question.Input.Type.IsChoice() {
			return false
		}
	case "blank":
		if gap.BlankGapID == nil || !hasGroupBlank(question, *gap.BlankGapID) {
			return false
		}
		key += ":" + *gap.BlankGapID
	default:
		return false
	}
	if v.targets[key] {
		return false
	}
	v.targets[key] = true
	return true
}

func hasGroupBlank(question GroupQuestion, id string) bool {
	for _, blank := range question.Input.Blanks {
		if blank.GapID != nil && *blank.GapID == id {
			return true
		}
	}
	return false
}

func (v *groupValidator) recordings(recordings []GroupRecording) error {
	seen := make(map[string]bool)
	for _, recording := range recordings {
		id := strings.ToLower(recording.AssetID)
		if !v.claimID(recording.ID) || !groupUUID(recording.AssetID) || !v.sharedAudio[id] || seen[id] {
			return &GroupError{Rule: groupRecording}
		}
		if !validGroupRecording(recording) {
			return &GroupError{Rule: groupRecording}
		}
		seen[id] = true
	}
	for id := range v.sharedAudio {
		if !seen[id] {
			return &GroupError{Rule: groupRecording}
		}
	}
	for _, question := range v.questions {
		if question.Input.MediaAssetID != nil && seen[strings.ToLower(*question.Input.MediaAssetID)] {
			return &GroupError{Rule: "group_audio_scope", QuestionID: question.ID}
		}
	}
	return nil
}

func validGroupRecording(recording GroupRecording) bool {
	return (recording.Policy.MaxPlays == nil || *recording.Policy.MaxPlays >= 1) && (recording.Transcript == nil || validGroupText(*recording.Transcript, MaxGroupTranscript, false))
}

func (v *groupValidator) claimID(id string) bool {
	key := strings.ToLower(id)
	if !groupUUID(id) || v.ids[key] {
		return false
	}
	v.ids[key] = true
	return true
}

func groupUUID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && len(id) == 36 && strings.EqualFold(parsed.String(), id)
}

func validGroupText(text string, limit int, required bool) bool {
	return utf8.ValidString(text) && !strings.ContainsRune(text, 0) && utf8.RuneCountInString(text) <= limit && (!required || strings.TrimSpace(text) != "")
}

func validGroupInstructions(raw json.RawMessage) bool {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true
	}
	_, err := content.ParseQuestion(raw)
	return err == nil
}

func validateGroupSize(bundle GroupBundle) error {
	raw, err := json.Marshal(bundle.Group)
	if err != nil || len(raw) > MaxGroupBytes {
		return &GroupError{Rule: groupLimits}
	}
	size := len(raw)
	for _, question := range bundle.Questions {
		raw, err := json.Marshal(question)
		if err != nil {
			return &GroupError{Rule: groupContent, QuestionID: question.ID}
		}
		size += len(raw)
		if size > MaxGroupBytes {
			return &GroupError{Rule: groupLimits}
		}
	}
	return nil
}
