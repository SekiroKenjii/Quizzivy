package domain_test

import (
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/notifications/domain"
)

var closes = time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC)

var everyKind = []struct {
	params domain.Params
	kind   domain.Kind
	names  []string
}{
	{domain.Submitted{Title: "Đề giữa kỳ", Count: 6, ToGrade: 2}, domain.AttemptSubmitted, []string{"count", "title", "toGrade"}},
	{domain.Flagged{StudentName: "Lê Hoàng Nam", Title: "Đề giữa kỳ", FocusLost: 3}, domain.AttemptFlagged, []string{"focusLost", "studentName", "title"}},
	{domain.Closing{Title: "Đề giữa kỳ", NotSubmitted: 4}, domain.AssignmentClosing, []string{"notSubmitted", "title"}},
	{domain.Joined{StudentName: "Nguyễn Gia Bảo", ClassName: "IELTS Foundation A"}, domain.ClassJoined, []string{"className", "studentName"}},
	{domain.CodesRotated{Count: 7, ClassNames: []string{"Lớp A", "Lớp B"}}, domain.JoinCodesRotated, []string{"classNames", "count"}},
	{domain.Opened{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.AssignmentOpened, []string{"closesAt", "title"}},
	{domain.DueSoon{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.AssignmentDueSoon, []string{"closesAt", "title"}},
	{domain.Extended{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.AssignmentExtended, []string{"closesAt", "title"}},
	{domain.Ready{Title: "Đề giữa kỳ"}, domain.ResultReady, []string{"title"}},
}

func contract(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func schema(t *testing.T, name string) *openapi3.Schema {
	t.Helper()
	ref := contract(t).Components.Schemas[name]
	if ref == nil || ref.Value == nil {
		t.Fatalf("the contract has no %s", name)
	}
	return ref.Value
}

func encoded(t *testing.T, p domain.Params) map[string]any {
	t.Helper()
	raw, err := domain.Encode(p)
	if err != nil {
		t.Fatalf("%T: %v", p, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%T encodes %s: %v", p, raw, err)
	}
	return out
}

func enum(t *testing.T, s *openapi3.Schema) []string {
	t.Helper()
	out := make([]string, 0, len(s.Enum))
	for _, v := range s.Enum {
		out = append(out, v.(string))
	}
	slices.Sort(out)
	return out
}

func TestEachKindEncodesExactlyItsOwnNames(t *testing.T) {
	for _, c := range everyKind {
		if got := c.params.Kind(); got != c.kind {
			t.Errorf("%T is for %s, want %s", c.params, got, c.kind)
		}
		if got := slices.Sorted(maps.Keys(encoded(t, c.params))); !slices.Equal(got, c.names) {
			t.Errorf("%s encodes %v, want exactly %v", c.kind, got, c.names)
		}
	}
}

func TestTheEncodedParamsAreTheContracts(t *testing.T) {
	params := schema(t, "NotificationParams")
	used := map[string]bool{}
	for _, c := range everyKind {
		object := encoded(t, c.params)
		if err := params.VisitJSON(object); err != nil {
			t.Errorf("%s: the contract refuses %v: %v", c.kind, object, err)
		}
		for name := range object {
			used[name] = true
		}
	}
	declared := slices.Sorted(maps.Keys(params.Properties))
	if got := slices.Sorted(maps.Keys(used)); !slices.Equal(got, declared) {
		t.Errorf("the kinds encode %v, the contract declares %v", got, declared)
	}
	if params.AdditionalProperties.Has == nil || *params.AdditionalProperties.Has {
		t.Error("NotificationParams is not a closed object")
	}
}

var kindRow = regexp.MustCompile("^\\| `([a-z_.]+)` \\| [^|]+ \\| (.+) \\|$")

func TestTheContractsKindTableIsWhatEachKindEncodes(t *testing.T) {
	kinds := schema(t, "NotificationKind")
	described := map[string][]string{}
	for _, line := range strings.Split(kinds.Description, "\n") {
		row := kindRow.FindStringSubmatch(strings.TrimSpace(line))
		if row == nil {
			continue
		}
		names := strings.Split(strings.ReplaceAll(row[2], "`", ""), ", ")
		slices.Sort(names)
		described[row[1]] = names
	}
	var known []string
	for _, c := range everyKind {
		known = append(known, string(c.kind))
		if !c.kind.Known() {
			t.Errorf("%s is not a kind the domain knows", c.kind)
		}
		if !slices.Equal(described[string(c.kind)], c.names) {
			t.Errorf("the contract says %s carries %v, it encodes %v", c.kind, described[string(c.kind)], c.names)
		}
	}
	slices.Sort(known)
	if declared := enum(t, kinds); !slices.Equal(declared, known) {
		t.Errorf("the contract's kinds are %v, the domain's %v", declared, known)
	}
	if len(described) != len(everyKind) {
		t.Errorf("the contract's table describes %d kinds, want %d", len(described), len(everyKind))
	}
	if domain.Kind("content.shared").Known() {
		t.Error("a kind outside the contract is known")
	}
}

func TestATimeIsEncodedInUTC(t *testing.T) {
	saigon := time.FixedZone("ICT", 7*3600)
	object := encoded(t, domain.DueSoon{Title: "Đề giữa kỳ", ClosesAt: time.Date(2026, 10, 24, 21, 0, 0, 0, saigon)})
	if object["closesAt"] != "2026-10-24T14:00:00Z" {
		t.Errorf("closesAt = %v, want 2026-10-24T14:00:00Z", object["closesAt"])
	}
}

func TestParamsOutsideTheContractsBoundsAreRefused(t *testing.T) {
	params := schema(t, "NotificationParams")
	limit := func(name string) int {
		property := params.Properties[name].Value
		if property.MaxLength == nil {
			t.Fatalf("%s has no maxLength in the contract", name)
		}
		return int(*property.MaxLength)
	}
	title, name := limit("title"), limit("studentName")
	if limit("className") != name || int(*params.Properties["classNames"].Value.Items.Value.MaxLength) != name {
		t.Fatal("the contract bounds its three name fields differently")
	}
	names := int(*params.Properties["classNames"].Value.MaxItems)
	text := func(n int) string { return strings.Repeat("ệ", n) }
	classes := func(n int) []string { return slices.Repeat([]string{"Lớp"}, n) }

	for label, c := range map[string]struct {
		params domain.Params
		ok     bool
	}{
		"a title at the limit":       {domain.Ready{Title: text(title)}, true},
		"a title over the limit":     {domain.Ready{Title: text(title + 1)}, false},
		"an empty title":             {domain.Ready{}, false},
		"a name at the limit":        {domain.Joined{StudentName: text(name), ClassName: text(name)}, true},
		"a student name over":        {domain.Joined{StudentName: text(name + 1), ClassName: "Lớp"}, false},
		"a class name over":          {domain.Joined{StudentName: "Nam", ClassName: text(name + 1)}, false},
		"an empty class name":        {domain.Joined{StudentName: "Nam"}, false},
		"zero counts":                {domain.Submitted{Title: "Đề"}, true},
		"a negative count":           {domain.Submitted{Title: "Đề", Count: -1}, false},
		"a negative to-grade":        {domain.Submitted{Title: "Đề", ToGrade: -1}, false},
		"a negative focus count":     {domain.Flagged{StudentName: "Nam", Title: "Đề", FocusLost: -1}, false},
		"a negative not-submitted":   {domain.Closing{Title: "Đề", NotSubmitted: -1}, false},
		"the most class names":       {domain.CodesRotated{Count: names, ClassNames: classes(names)}, true},
		"one class name too many":    {domain.CodesRotated{Count: names + 1, ClassNames: classes(names + 1)}, false},
		"no class name":              {domain.CodesRotated{Count: 1}, false},
		"fewer classes than names":   {domain.CodesRotated{Count: 1, ClassNames: classes(2)}, false},
		"more classes than names":    {domain.CodesRotated{Count: 9, ClassNames: classes(2)}, true},
		"an overlong name in a list": {domain.CodesRotated{Count: 1, ClassNames: []string{text(name + 1)}}, false},
		"an empty name in a list":    {domain.CodesRotated{Count: 1, ClassNames: []string{""}}, false},
		"no closing time":            {domain.Opened{Title: "Đề"}, false},
	} {
		raw, err := domain.Encode(c.params)
		if c.ok && err != nil {
			t.Errorf("%s: refused with %v", label, err)
		}
		if !c.ok && !errors.Is(err, domain.ErrInvalidParams) {
			t.Errorf("%s: err = %v, want ErrInvalidParams; encoded %s", label, err, raw)
		}
		if !c.ok {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		if err := params.VisitJSON(object); err != nil {
			t.Errorf("%s: the domain accepts what the contract refuses: %v", label, err)
		}
	}
}
