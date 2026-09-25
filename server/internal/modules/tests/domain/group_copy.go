package domain

import (
	"encoding/json"
	"quizzivy/internal/shared/content"
	"strings"
)

type groupCopier struct {
	nextID    func() string
	reserved  map[string]bool
	questions map[string]string
	blanks    map[string]map[string]string
	failed    bool
}

// Copy creates a detached graph with fresh identities and unchanged grading/content; immutable asset IDs are reused.
func (b GroupBundle) Copy(nextID func() string) (GroupBundle, error) {
	if err := b.Validate(); err != nil {
		return GroupBundle{}, err
	}
	if nextID == nil {
		return GroupBundle{}, &GroupError{Rule: groupCopyIdentity}
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return GroupBundle{}, &GroupError{Rule: groupContent}
	}
	var copied GroupBundle
	if err := json.Unmarshal(raw, &copied); err != nil {
		return GroupBundle{}, &GroupError{Rule: groupContent}
	}
	c := groupCopier{nextID: nextID, reserved: map[string]bool{}, questions: map[string]string{}, blanks: map[string]map[string]string{}}
	c.reserve(b)
	if err := c.copyQuestions(copied.Questions); err != nil {
		return GroupBundle{}, err
	}
	copied.Group.ID = c.next()
	for i := range copied.Group.Members {
		member := &copied.Group.Members[i]
		member.QuestionID = c.questions[strings.ToLower(member.QuestionID)]
	}
	for i := range copied.Group.Stimuli {
		if err := c.copyStimulus(&copied.Group.Stimuli[i]); err != nil {
			return GroupBundle{}, err
		}
	}
	for i := range copied.Group.Recordings {
		copied.Group.Recordings[i].ID = c.next()
	}
	if c.failed {
		return GroupBundle{}, &GroupError{Rule: groupCopyIdentity}
	}
	if err := copied.Validate(); err != nil {
		return GroupBundle{}, err
	}
	return copied, nil
}

func (c *groupCopier) next() string {
	if c.failed {
		return ""
	}
	id := strings.ToLower(c.nextID())
	if !groupUUID(id) || c.reserved[id] {
		c.failed = true
		return ""
	}
	c.reserved[id] = true
	return id
}

func (c *groupCopier) reserve(bundle GroupBundle) {
	c.reserved[strings.ToLower(bundle.Group.ID)] = true
	for _, stimulus := range bundle.Group.Stimuli {
		c.reserved[strings.ToLower(stimulus.ID)] = true
		document, _ := content.Parse(stimulus.Content)
		c.reserveDocument(document)
	}
	for _, recording := range bundle.Group.Recordings {
		c.reserved[strings.ToLower(recording.ID)] = true
		c.reserved[strings.ToLower(recording.AssetID)] = true
	}
	for _, question := range bundle.Questions {
		c.reserveQuestion(question)
	}
}

func (c *groupCopier) reserveQuestion(question GroupQuestion) {
	c.reserved[strings.ToLower(question.ID)] = true
	if question.Input.MediaAssetID != nil {
		c.reserved[strings.ToLower(*question.Input.MediaAssetID)] = true
	}
	for _, option := range question.Input.Options {
		if option.ID != nil {
			c.reserved[strings.ToLower(*option.ID)] = true
		}
	}
	for _, blank := range question.Input.Blanks {
		if blank.ID != nil {
			c.reserved[strings.ToLower(*blank.ID)] = true
		}
		if blank.GapID != nil {
			c.reserved[strings.ToLower(*blank.GapID)] = true
		}
	}
}

func (c *groupCopier) reserveDocument(document content.Document) {
	for _, gap := range document.GapIDs() {
		c.reserved[strings.ToLower(gap)] = true
	}
	for _, asset := range document.Assets() {
		c.reserved[strings.ToLower(asset.ID)] = true
	}
}

func (c *groupCopier) copyQuestions(questions []GroupQuestion) error {
	for i := range questions {
		question := &questions[i]
		sourceID := strings.ToLower(question.ID)
		question.ID = c.next()
		c.questions[sourceID] = question.ID
		before := make([]string, len(question.Input.Blanks))
		for j, blank := range question.Input.Blanks {
			if blank.GapID != nil {
				before[j] = *blank.GapID
			}
		}
		if err := question.Input.RebindGaps(c.next); err != nil {
			return &GroupError{Rule: groupCopyIdentity}
		}
		c.blanks[sourceID] = map[string]string{}
		for j := range question.Input.Blanks {
			blank := &question.Input.Blanks[j]
			id := c.next()
			blank.ID = &id
			if blank.GapID != nil {
				c.blanks[sourceID][before[j]] = *blank.GapID
			}
		}
		for j := range question.Input.Options {
			id := c.next()
			question.Input.Options[j].ID = &id
		}
	}
	return nil
}

func (c *groupCopier) copyStimulus(stimulus *GroupStimulus) error {
	stimulus.ID = c.next()
	document, err := content.Parse(stimulus.Content)
	if err != nil {
		return &GroupError{Rule: groupContent}
	}
	ids := make(map[string]string)
	for _, gap := range document.GapIDs() {
		ids[gap] = c.next()
	}
	document, err = document.WithGapIDs(ids)
	if err != nil {
		return &GroupError{Rule: groupCopyIdentity}
	}
	stimulus.Content, err = document.MarshalJSON()
	if err != nil {
		return &GroupError{Rule: groupContent}
	}
	for i := range stimulus.Gaps {
		gap := &stimulus.Gaps[i]
		sourceID := strings.ToLower(gap.QuestionID)
		gap.QuestionID = c.questions[sourceID]
		gap.GapID = ids[gap.GapID]
		if gap.BlankGapID != nil {
			id := c.blanks[sourceID][*gap.BlankGapID]
			gap.BlankGapID = &id
		}
	}
	return nil
}
