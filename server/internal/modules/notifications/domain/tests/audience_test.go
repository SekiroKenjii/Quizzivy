package domain_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/modules/notifications/domain"
)

var audienceRow = regexp.MustCompile("^\\| `([a-z_.]+)` \\| a (teacher|student) \\| .+ \\|$")

func TestEveryKindBelongsToExactlyOneAudienceAndTheContractAgrees(t *testing.T) {
	described := map[string]string{}
	for _, line := range strings.Split(schema(t, "NotificationKind").Description, "\n") {
		if row := audienceRow.FindStringSubmatch(strings.TrimSpace(line)); row != nil {
			described[row[1]] = row[2]
		}
	}
	covered := map[domain.Kind]int{}
	for _, audience := range []domain.Audience{domain.AudienceTeacher, domain.AudienceStudent} {
		for _, kind := range domain.KindsOf(audience) {
			covered[kind]++
			if kind.Audience() != audience {
				t.Errorf("%s is listed for %s but says it is for %q", kind, audience, kind.Audience())
			}
		}
	}
	for _, c := range everyKind {
		if covered[c.kind] != 1 {
			t.Errorf("%s is in %d audiences, want exactly one", c.kind, covered[c.kind])
		}
		if want := described[string(c.kind)]; string(c.kind.Audience()) != want {
			t.Errorf("the contract says %s is for a %s, the domain says %q", c.kind, want, c.kind.Audience())
		}
	}
	if len(covered) != len(everyKind) || len(described) != len(everyKind) {
		t.Errorf("%d kinds are in an audience and %d in the contract's table, want %d", len(covered), len(described), len(everyKind))
	}
	if domain.Kind("content.shared").Audience() != "" {
		t.Error("a kind outside the contract has an audience")
	}
}

func TestTheAudiencesAreTheContractsAndTheEmptyOneRestrictsNothing(t *testing.T) {
	if got, want := enum(t, schema(t, "NotificationAudience")), []string{"student", "teacher"}; !slices.Equal(got, want) {
		t.Errorf("the contract's audiences are %v, want %v", got, want)
	}
	for _, valid := range []domain.Audience{"", domain.AudienceTeacher, domain.AudienceStudent} {
		if !valid.Valid() {
			t.Errorf("%q is refused", valid)
		}
	}
	for _, invalid := range []domain.Audience{"admin", "Teacher", " teacher", "all"} {
		if invalid.Valid() {
			t.Errorf("%q is accepted", invalid)
		}
		if domain.KindsOf(invalid) != nil {
			t.Errorf("%q names kinds", invalid)
		}
	}
	if kinds := domain.KindsOf(""); kinds != nil {
		t.Errorf("the empty audience restricts to %v, want no restriction", kinds)
	}
}

func TestKindsOfGivesACopyTheCallerMayChange(t *testing.T) {
	first := domain.KindsOf(domain.AudienceStudent)
	first[0] = domain.ClassJoined
	if again := domain.KindsOf(domain.AudienceStudent); again[0] != domain.AssignmentOpened {
		t.Errorf("changing a result changed the next one: %v", again)
	}
}
