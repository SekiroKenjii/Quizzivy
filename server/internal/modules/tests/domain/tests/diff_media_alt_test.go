package domain_test

import (
	"quizzivy/internal/modules/tests/domain"
	"reflect"
	"testing"
)

func imageQuestion(source string, alt *string) domain.DraftQuestion {
	q := choiceQuestion(source, "Con vật nào trong ảnh?", "1.00", true, false)
	q.MediaAssetID = textOf("0195a000-0000-7000-8000-000000000001")
	q.MediaAssetKind = textOf("image")
	q.MediaAlt = alt
	return q
}

func TestAChangedAltTextIsAChangeToTheQuestionsMedia(t *testing.T) {
	cases := []struct {
		name     string
		from, to *string
		changed  bool
	}{
		{"added", nil, textOf("Một chú mèo"), true},
		{"edited", textOf("Một chú mèo"), textOf("Một chú chó"), true},
		{"cleared", textOf("Một chú mèo"), nil, true},
		{"unchanged", textOf("Một chú mèo"), textOf("Một chú mèo"), false},
		{"absent on both sides", nil, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			changes := compare(t, onePart(imageQuestion("q1", c.from)), onePart(imageQuestion("q1", c.to)))
			if !c.changed {
				if len(changes) != 0 {
					t.Fatalf("changes = %+v, want none", changes)
				}
				return
			}
			if len(changes) != 1 {
				t.Fatalf("changes = %v, want exactly one, so unpublishedChanges rises by one", kinds(changes))
			}
			changed := only(t, changes, domain.ChangeChanged)
			if !reflect.DeepEqual(changed.Fields, []domain.ChangedField{domain.FieldMedia}) || changed.Number != 1 || changed.QuestionID != "q1" {
				t.Errorf("change = %+v, want question 1 changed in its media alone", changed)
			}
		})
	}
}

func TestACopyThatKeepsTheAltTextIsTheSameQuestionAndOneThatLostItIsNot(t *testing.T) {
	alt := textOf("Một chú mèo")
	frozen := imageQuestion("", alt)
	frozen.FrozenID = "f1"
	if changes := compare(t, onePart(frozen), onePart(imageQuestion("restored-copy", alt))); len(changes) != 0 {
		t.Fatalf("a restored copy with its alt text = %+v, want the same question", changes)
	}
	lost := compare(t, onePart(frozen), onePart(imageQuestion("restored-copy", nil)))
	if !reflect.DeepEqual(kinds(lost), []domain.ChangeKind{domain.ChangeAdded, domain.ChangeRemoved}) {
		t.Fatalf("a copy that lost its alt text = %v, want it read as added and removed", kinds(lost))
	}
}
