package domain

import "quizzivy/internal/shared/content"

// RebindGaps gives a copied question new local gap identities without changing labels or answers.
func (in *Input) RebindGaps(nextID func() string) error {
	if !hasProse(in.PromptContent) {
		return nil
	}
	document, err := content.ParseQuestionPrompt(in.PromptContent)
	if err != nil {
		return err
	}
	if len(document.GapIDs()) == 0 {
		return nil
	}
	ids := make(map[string]string)
	for _, id := range document.GapIDs() {
		ids[id] = nextID()
	}
	copied, err := document.WithGapIDs(ids)
	if err != nil {
		return err
	}
	for i := range in.Blanks {
		if in.Blanks[i].GapID == nil {
			return content.ErrInvalidDocument
		}
		id, ok := ids[*in.Blanks[i].GapID]
		if !ok {
			return content.ErrInvalidDocument
		}
		in.Blanks[i].GapID = &id
	}
	in.PromptContent, err = copied.MarshalJSON()
	return err
}
