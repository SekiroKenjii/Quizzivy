package domain

import (
	"bytes"
	"encoding/json"

	"quizzivy/internal/shared/content"
)

// MaxTestTitle and MaxChangeNote are the most characters a test's title and a version's change note hold, as the contract
// and the tests and test_versions tables bound them.
const (
	MaxTestTitle  = 200
	MaxChangeNote = 200
)

// Composed returns the outline write with the text it carries composed to NFC: the title, the description and every
// section's title and instructions. A text that composing leaves over its limit comes back as a *ValidationError naming it.
func (in UpdateInput) Composed() (UpdateInput, error) {
	var c content.Composer
	out := in
	out.Title = c.Optional("title", in.Title, MaxTestTitle)
	out.Description = c.Optional("description", in.Description, 0)
	if in.Sections != nil {
		out.Sections = make([]SectionInput, len(in.Sections))
		for i, section := range in.Sections {
			section.Title = c.Text(sectionField(i, "title"), section.Title, 0)
			section.Instructions = c.Optional(sectionField(i, "instructions"), section.Instructions, 0)
			out.Sections[i] = section
		}
	}
	if err := c.Err(); err != nil {
		return in, err
	}
	return out, nil
}

// Composed returns the bundle with every text of the group and of its member questions composed to NFC: the group's
// title and instructions, each stimulus's title and material, each recording's transcript and each question's input.
// Identities, bindings and policies are left as they are. The limits of the group's own text are checked by Validate on
// the composed bundle; a document that composing makes invalid, or a member whose text composing leaves over its limit,
// comes back as a *GroupError with the rule group_content.
func (b GroupBundle) Composed() (GroupBundle, error) {
	group := b.Group
	group.Title = content.NFC(group.Title)
	instructions, err := composeMaterial(group.Instructions)
	if err != nil {
		return b, &GroupError{Rule: groupContent}
	}
	group.Instructions = instructions
	if group.Stimuli, err = composeStimuli(b.Group.Stimuli); err != nil {
		return b, err
	}
	group.Recordings = composeRecordings(b.Group.Recordings)
	questions, err := composeMembers(b.Questions)
	if err != nil {
		return b, err
	}
	return GroupBundle{Group: group, Questions: questions}, nil
}

func composeStimuli(stimuli []GroupStimulus) ([]GroupStimulus, error) {
	if stimuli == nil {
		return nil, nil
	}
	out := make([]GroupStimulus, len(stimuli))
	for i, stimulus := range stimuli {
		stimulus.Title = content.NFC(stimulus.Title)
		material, err := composeMaterial(stimulus.Content)
		if err != nil {
			return nil, &GroupError{Rule: groupContent, StimulusID: stimulus.ID}
		}
		stimulus.Content = material
		out[i] = stimulus
	}
	return out, nil
}

func composeRecordings(recordings []GroupRecording) []GroupRecording {
	if recordings == nil {
		return nil
	}
	out := make([]GroupRecording, len(recordings))
	for i, recording := range recordings {
		recording.Transcript = content.NFCPtr(recording.Transcript)
		out[i] = recording
	}
	return out
}

func composeMembers(members []GroupQuestion) ([]GroupQuestion, error) {
	if members == nil {
		return nil, nil
	}
	out := make([]GroupQuestion, len(members))
	for i, member := range members {
		input, err := member.Input.Composed()
		if err != nil {
			return nil, &GroupError{Rule: groupContent, QuestionID: member.ID}
		}
		member.Input = input
		out[i] = member
	}
	return out, nil
}

func composeMaterial(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw, nil
	}
	document, err := content.Parse(raw)
	if err != nil {
		return raw, nil
	}
	composed, err := content.Normalize(document)
	if err != nil {
		return raw, err
	}
	return composed.MarshalJSON()
}
