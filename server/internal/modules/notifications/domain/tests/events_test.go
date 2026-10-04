package domain_test

import (
	"errors"
	"slices"
	"testing"

	"quizzivy/internal/modules/notifications/domain"
)

func theFive() []domain.Preference {
	return []domain.Preference{
		{Event: domain.EventAttemptSubmitted, InApp: true},
		{Event: domain.EventAttemptFlagged, InApp: true},
		{Event: domain.EventAssignmentClosing, InApp: true},
		{Event: domain.EventAssignmentDueSoon, InApp: true},
		{Event: domain.EventResultReady, InApp: true},
	}
}

func TestTheSwitchesAreTheContractsEventsInItsOrder(t *testing.T) {
	events := schema(t, "NotificationEvent")
	declared := make([]domain.Event, 0, len(events.Enum))
	for _, v := range events.Enum {
		declared = append(declared, domain.Event(v.(string)))
	}
	if !slices.Equal(domain.Events(), declared) {
		t.Errorf("the switches are %v, the contract's events %v", domain.Events(), declared)
	}
	set := schema(t, "NotificationPreferences")
	if set.MaxItems == nil {
		t.Fatal("the contract puts no cap on the preferences")
	}
	if set.MinItems != uint64(len(declared)) || *set.MaxItems != uint64(len(declared)) {
		t.Errorf("the contract takes %d to %d preferences, want exactly %d", set.MinItems, *set.MaxItems, len(declared))
	}
	domain.Events()[0] = "changed"
	if domain.Events()[0] != domain.EventAttemptSubmitted {
		t.Error("a caller can change the list of switches")
	}
}

func TestASwitchNeverSavedIsInTheAppAndNotByEmail(t *testing.T) {
	got := domain.WithDefaults(nil)
	if !slices.Equal(got, theFive()) {
		t.Errorf("the defaults are %+v, want every switch in the app and none by email", got)
	}
	for _, event := range domain.Events() {
		if p := domain.DefaultPreference(event); p != (domain.Preference{Event: event, InApp: true}) {
			t.Errorf("the default for %s is %+v", event, p)
		}
	}
}

func TestASavedSwitchWinsOverItsDefault(t *testing.T) {
	stored := []domain.Preference{
		{Event: domain.EventResultReady, InApp: false, Email: true},
		{Event: domain.EventAttemptFlagged, InApp: true, Email: true},
		{Event: "summary.weekly", InApp: true, Email: true},
	}
	want := theFive()
	want[1].Email = true
	want[4] = domain.Preference{Event: domain.EventResultReady, InApp: false, Email: true}
	if got := domain.WithDefaults(stored); !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAWholeSetHoldsEverySwitchExactlyOnce(t *testing.T) {
	given := theFive()
	given[0].InApp, given[3].Email = false, true
	want := slices.Clone(given)
	slices.Reverse(given)
	got, err := domain.WholeSet(given)
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("a reversed set gives %+v (%v), want it in the switches' order: %+v", got, err, want)
	}

	repeated := theFive()
	repeated[4] = domain.Preference{Event: domain.EventAttemptSubmitted, InApp: false}
	if _, err := domain.WholeSet(repeated); !errors.Is(err, domain.ErrEventsNotOnceEach) {
		t.Errorf("a repeated event: err = %v, want ErrEventsNotOnceEach", err)
	}
	if _, err := domain.WholeSet(append(theFive(), theFive()[2])); !errors.Is(err, domain.ErrEventsNotOnceEach) {
		t.Errorf("a sixth switch: err = %v, want ErrEventsNotOnceEach", err)
	}
	if _, err := domain.WholeSet(theFive()[:4]); !errors.Is(err, domain.ErrEventsNotOnceEach) {
		t.Errorf("a missing event: err = %v, want ErrEventsNotOnceEach", err)
	}
	if _, err := domain.WholeSet(nil); !errors.Is(err, domain.ErrEventsNotOnceEach) {
		t.Errorf("no switch at all: err = %v, want ErrEventsNotOnceEach", err)
	}
	unknown := theFive()
	unknown[2].Event = "class.joined"
	if _, err := domain.WholeSet(unknown); !errors.Is(err, domain.ErrUnknownEvent) {
		t.Errorf("an event that is not a switch: err = %v, want ErrUnknownEvent", err)
	}
}

func TestAPageIsSizedWithinTheContractsLimits(t *testing.T) {
	var limit *struct{ def, max, min float64 }
	for _, p := range contract(t).Paths.Find("/me/notifications").Get.Parameters {
		if p.Value.Name == "limit" {
			s := p.Value.Schema.Value
			limit = &struct{ def, max, min float64 }{s.Default.(float64), *s.Max, *s.Min}
		}
	}
	if limit == nil {
		t.Fatal("listNotifications declares no limit")
	}
	if int(limit.def) != domain.DefaultLimit || int(limit.max) != domain.MaxLimit || limit.min != 1 {
		t.Errorf("the contract's limit is %+v, the domain's default %d and maximum %d", *limit, domain.DefaultLimit, domain.MaxLimit)
	}
	for given, want := range map[int]int{
		-1: domain.DefaultLimit, 0: domain.DefaultLimit, 1: 1, 20: 20,
		domain.MaxLimit: domain.MaxLimit, domain.MaxLimit + 1: domain.MaxLimit, 100000: domain.MaxLimit,
	} {
		if got := (domain.ListQuery{Limit: given}).Size(); got != want {
			t.Errorf("a limit of %d gives a page of %d, want %d", given, got, want)
		}
	}
	if domain.DefaultLimit != 20 || domain.MaxLimit != 50 {
		t.Errorf("the page is %d by default and %d at most, want 20 and 50", domain.DefaultLimit, domain.MaxLimit)
	}
}

func TestTheCapOnMarkedIdsIsTheContracts(t *testing.T) {
	body := contract(t).Paths.Find("/me/notifications/read").Post.RequestBody.Value.Content.Get("application/json").Schema.Value
	ids := body.Properties["ids"].Value
	if ids.MaxItems == nil {
		t.Fatal("the contract puts no cap on ids")
	}
	if int(*ids.MaxItems) != domain.MaxMarkedIDs || ids.MinItems != 1 {
		t.Errorf("the contract takes %d to %d ids, the domain 1 to %d", ids.MinItems, *ids.MaxItems, domain.MaxMarkedIDs)
	}
	if domain.MaxMarkedIDs != 100 {
		t.Errorf("the cap is %d, want 100", domain.MaxMarkedIDs)
	}
	if slices.Contains(body.Required, "ids") {
		t.Error("the contract requires ids, so nothing can ask for every unread notification")
	}
}
