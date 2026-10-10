package domain_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"quizzivy/internal/modules/attempts/domain"

	"golang.org/x/text/unicode/norm"
)

func typed(t testing.TB, payload map[string]any) domain.Answer {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Answer{QuestionID: "question-id", Payload: raw}
}

func decoded(t testing.TB, answer domain.Answer) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(answer.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestATextAnswerIsComposed(t *testing.T) {
	answer := typed(t, map[string]any{"type": "text", "value": norm.NFD.String("Hà Nội là thủ đô.")})
	got := answer.Composed()
	if value := decoded(t, got)["value"]; value != "Hà Nội là thủ đô." {
		t.Fatalf("value=%q", value)
	}
	if got.QuestionID != "question-id" || decoded(t, got)["type"] != "text" {
		t.Fatalf("answer=%+v", got)
	}
}

func TestEveryValueOfAFillInAnswerIsComposedAndItsKeysAreNot(t *testing.T) {
	blank := "0195aaaa-0000-7000-8000-000000000001"
	other := "0195aaaa-0000-7000-8000-000000000002"
	answer := typed(t, map[string]any{"type": "fill_blank", "values": map[string]any{blank: norm.NFD.String("Hà Nội"), other: "Huế"}})
	got := decoded(t, answer.Composed())["values"].(map[string]any)
	if got[blank] != "Hà Nội" || got[other] != "Huế" || len(got) != 2 {
		t.Fatalf("values=%v", got)
	}
}

func TestOtherAnswersAreReturnedByteForByte(t *testing.T) {
	for name, payload := range map[string]map[string]any{
		"choice":     {"type": "choice", "optionIds": []any{"0195aaaa-0000-7000-8000-000000000001"}},
		"true_false": {"type": "true_false", "value": true},
		"composed":   {"type": "text", "value": "Hà Nội"},
		"empty text": {"type": "text", "value": ""},
	} {
		t.Run(name, func(t *testing.T) {
			answer := typed(t, payload)
			got := answer.Composed()
			if string(got.Payload) != string(answer.Payload) {
				t.Fatalf("payload was rewritten: %s", got.Payload)
			}
		})
	}
}

func TestADecomposedAnswerWrittenWithEscapesIsStillComposed(t *testing.T) {
	answer := domain.Answer{QuestionID: "q", Payload: []byte(`{"type":"text","value":"Hè nọi"}`)}
	got := decoded(t, answer.Composed())["value"]
	if got != norm.NFC.String("Hè nọi") {
		t.Fatalf("value=%q", got)
	}
}

func TestAPayloadThatIsNotAJSONObjectIsLeftToTheWrite(t *testing.T) {
	for _, raw := range []string{`not json`, `[1,2]`, `"text"`, ``} {
		answer := domain.Answer{QuestionID: "q", Payload: []byte(raw)}
		if got := answer.Composed(); !reflect.DeepEqual(got, answer) {
			t.Fatalf("%q was rewritten: %q", raw, got.Payload)
		}
	}
}

func TestComposingAnAnswerDoesNotChangeTheOriginal(t *testing.T) {
	answer := typed(t, map[string]any{"type": "text", "value": norm.NFD.String("Hà Nội")})
	before := string(answer.Payload)
	_ = answer.Composed()
	if string(answer.Payload) != before {
		t.Fatal("the payload of the receiver was rewritten")
	}
}

func TestAnAnswerKeepsMarkupCharactersAsTyped(t *testing.T) {
	answer := typed(t, map[string]any{"type": "text", "value": norm.NFD.String("Hà <b> & \"Nội\"")})
	got := decoded(t, answer.Composed())["value"]
	if got != "Hà <b> & \"Nội\"" {
		t.Fatalf("value=%q", got)
	}
}
