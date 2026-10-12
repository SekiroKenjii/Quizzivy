package domain

import (
	"bytes"
	"encoding/json"

	"quizzivy/internal/shared/content"

	"golang.org/x/text/unicode/norm"
)

// MaxTeacherNote is the longest private note on a paper, in characters.
const MaxTeacherNote = 2000

// Composed returns the answer with the text a student typed composed to NFC: the value of a text answer and each value of
// a fill-in answer. Option ids, booleans and the ids that key a fill-in answer are left as they are, and so is a payload that
// is not JSON, which the write refuses as it always did. The grader folds composition on both sides, so a score never
// changes; the stored text is what the teacher reads back.
func (a Answer) Composed() Answer {
	if !bytes.Contains(a.Payload, []byte(`\u`)) && norm.NFC.IsNormal(a.Payload) {
		return a
	}
	var payload map[string]any
	if err := json.Unmarshal(a.Payload, &payload); err != nil {
		return a
	}
	if !composeTyped(payload) {
		return a
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return a
	}
	return Answer{QuestionID: a.QuestionID, Payload: bytes.TrimSpace(out.Bytes())}
}

func composeTyped(payload map[string]any) bool {
	changed := false
	switch payload["type"] {
	case "text":
		if value, ok := payload["value"].(string); ok {
			if composed := content.NFC(value); composed != value {
				payload["value"] = composed
				changed = true
			}
		}
	case "fill_blank":
		values, _ := payload["values"].(map[string]any)
		for blank, raw := range values {
			if value, ok := raw.(string); ok {
				if composed := content.NFC(value); composed != value {
					values[blank] = composed
					changed = true
				}
			}
		}
	}
	return changed
}
