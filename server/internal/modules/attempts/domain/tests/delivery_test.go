package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"quizzivy/internal/modules/attempts/domain"
	testsdomain "quizzivy/internal/modules/tests/domain"
)

func TestDeliveryVersionsPreserveStandaloneDeals(t *testing.T) {
	qs := questions(12)
	sections := []domain.Section{{ID: "second"}, {ID: "first"}}
	for i := range qs {
		qs[i].SectionID = sections[i%2].ID
	}
	golden, err := domain.Deal.PresentVersion(testsdomain.DeliverySectionV1, 0x5eed, true, true, sections, qs)
	if err != nil || order(golden) != "q-04,q-10,q-02,q-00,q-06,q-08,q-09,q-05,q-07,q-11,q-03,q-01" {
		t.Fatalf("historical section deal changed: %v, %v", order(golden), err)
	}
	if golden[3].Options[2].ID != "q-00-d" || golden[3].Options[3].ID != "q-00-c" {
		t.Fatal("historical option order changed")
	}
	for seed := int64(0); seed < 100; seed++ {
		for _, shuffleQuestions := range []bool{false, true} {
			for _, shuffleOptions := range []bool{false, true} {
				legacy, err := domain.Deal.PresentVersion(testsdomain.DeliverySectionV1, seed, shuffleQuestions, shuffleOptions, sections, qs)
				if err != nil {
					t.Fatal(err)
				}
				current, err := domain.Deal.PresentVersion(testsdomain.DeliveryGroupV1, seed, shuffleQuestions, shuffleOptions, sections, qs)
				if err != nil || !reflect.DeepEqual(legacy, current) {
					t.Fatalf("standalone deal changed for seed %d, flags %v/%v: %v", seed, shuffleQuestions, shuffleOptions, err)
				}
			}
		}
	}
}

func TestDeliveryRefusesUnknownOrInconsistentVersion(t *testing.T) {
	for _, version := range []testsdomain.DeliveryVersion{"", "group_v2"} {
		got, err := domain.Deal.PresentVersion(version, 1, false, false, nil, questions(2))
		if !errors.Is(err, domain.ErrUnsupportedDeliveryVersion) || got != nil {
			t.Fatalf("unsupported version %q returned a paper: %v", version, err)
		}
	}
	qs := questions(3)
	qs[0].GroupID, qs[1].GroupID = "group", "group"
	qs[1].GroupOrdinal = 1
	got, err := domain.Deal.PresentVersion(testsdomain.DeliverySectionV1, 1, true, true, nil, qs)
	if !errors.Is(err, domain.ErrUnsupportedDeliveryVersion) || got != nil {
		t.Fatalf("legacy algorithm accepted group metadata: %v", err)
	}
	got, err = domain.Deal.PresentVersion(testsdomain.DeliveryGroupV1, 1, true, true, nil, qs)
	if err != nil || !reflect.DeepEqual(got, domain.Deal.Present(1, true, true, nil, qs)) {
		t.Fatalf("versioned group deal changed: %v", err)
	}
}
