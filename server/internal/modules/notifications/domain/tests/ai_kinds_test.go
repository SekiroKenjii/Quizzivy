package domain_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/modules/notifications/domain"
)

var aiKinds = []struct {
	params domain.Params
	kind   domain.Kind
	names  []string
}{
	{domain.AllowanceWarning{Used: 16_500, Allowance: 20_000}, domain.AIAllowanceWarning, []string{"allowance", "used"}},
	{domain.CreditRequest{TeacherName: "Nguyễn Thị Lan", Credits: 20_000}, domain.AICreditRequest, []string{"credits", "teacherName"}},
	{domain.CreditRequestDecided{Credits: 20_000, Decision: domain.DecisionApproved}, domain.AICreditRequestDecided, []string{"credits", "decision"}},
}

func TestTheAIKindsAreWrittenWithoutASwitchAndEncodeTheirOwnNames(t *testing.T) {
	for _, c := range aiKinds {
		if !c.kind.Known() {
			t.Errorf("%s is not a kind the domain knows", c.kind)
		}
		if event, governed := c.kind.Event(); governed {
			t.Errorf("%s is governed by the switch %s; no switch exists for it", c.kind, event)
		}
		if c.params.Kind() != c.kind {
			t.Errorf("%T is for %s, want %s", c.params, c.params.Kind(), c.kind)
		}
		if got := slices.Sorted(maps.Keys(encoded(t, c.params))); !slices.Equal(got, c.names) {
			t.Errorf("%s encodes %v, want exactly %v", c.kind, got, c.names)
		}
		notice := domain.Notice{UserID: "u-1", Kind: c.kind, Params: c.params, DedupeKey: "ai:key", Merge: domain.Replace}
		if err := notice.Validate(); err != nil {
			t.Errorf("%s: %v", c.kind, err)
		}
		if err := (domain.Notice{UserID: "u-1", Kind: c.kind, Params: domain.Ready{Title: "x"}, DedupeKey: "ai:key", Merge: domain.Replace}).Validate(); !errors.Is(err, domain.ErrParamsMismatch) {
			t.Errorf("%s with another kind's params: %v", c.kind, err)
		}
	}
}

func TestAIParamsOutsideTheirBoundsAreRefused(t *testing.T) {
	long := strings.Repeat("ệ", 201)
	for label, c := range map[string]struct {
		params domain.Params
		ok     bool
	}{
		"nothing used yet":           {domain.AllowanceWarning{Allowance: 1_000}, true},
		"a negative use":             {domain.AllowanceWarning{Used: -1, Allowance: 1_000}, false},
		"a negative allowance":       {domain.AllowanceWarning{Used: 1, Allowance: -1}, false},
		"a name at the limit":        {domain.CreditRequest{TeacherName: strings.Repeat("ệ", 200), Credits: 1_000}, true},
		"a name over the limit":      {domain.CreditRequest{TeacherName: long, Credits: 1_000}, false},
		"no name":                    {domain.CreditRequest{Credits: 1_000}, false},
		"a negative request":         {domain.CreditRequest{TeacherName: "Lan", Credits: -1}, false},
		"a decline":                  {domain.CreditRequestDecided{Credits: 1_000, Decision: domain.DecisionDeclined}, true},
		"a decision nobody can make": {domain.CreditRequestDecided{Credits: 1_000, Decision: "pending"}, false},
		"no decision":                {domain.CreditRequestDecided{Credits: 1_000}, false},
	} {
		_, err := domain.Encode(c.params)
		if c.ok && err != nil {
			t.Errorf("%s: refused with %v", label, err)
		}
		if !c.ok && !errors.Is(err, domain.ErrInvalidParams) {
			t.Errorf("%s: err = %v, want ErrInvalidParams", label, err)
		}
	}
}

func TestTheAIDedupeKeysNameWhatTheyAreAbout(t *testing.T) {
	if got := domain.AllowanceWarningKey("2026-10-01"); got != "ai_warning:2026-10-01" {
		t.Errorf("warning key %q", got)
	}
	if got := domain.CreditRequestKey("q-1"); got != "ai_request:q-1" {
		t.Errorf("request key %q", got)
	}
	if got := domain.CreditDecisionKey("q-1"); got != "ai_decision:q-1" {
		t.Errorf("decision key %q", got)
	}
}
