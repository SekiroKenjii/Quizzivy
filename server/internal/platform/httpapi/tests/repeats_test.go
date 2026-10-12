package httpapi_test

import (
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/internal/platform/httpapi"
)

func TestAnIdIsRepeatedWhenTwoSpellingsParseToTheSameUUID(t *testing.T) {
	const lower = "019535d9-3df7-79fb-b466-fa907fa17f9e"
	const upper = "019535D9-3DF7-79FB-B466-FA907FA17F9E"
	parse := func(spelled ...string) []openapi_types.UUID {
		ids := make([]openapi_types.UUID, len(spelled))
		for i, s := range spelled {
			ids[i] = uuid.MustParse(s)
		}
		return ids
	}
	other := uuid.NewString()

	for _, c := range []struct {
		name string
		ids  []openapi_types.UUID
		want bool
	}{
		{"none", nil, false},
		{"one", parse(lower), false},
		{"two different", parse(lower, other), false},
		{"the same text twice", parse(lower, lower), true},
		{"the id, then in capitals", parse(lower, upper), true},
		{"capitals, then the id", parse(upper, lower), true},
		{"a repeat after another id", parse(lower, other, upper), true},
	} {
		if got := httpapi.RepeatsAnID(c.ids); got != c.want {
			t.Errorf("%s: RepeatsAnID = %v, want %v", c.name, got, c.want)
		}
	}
}
