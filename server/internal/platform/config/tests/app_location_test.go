package config_test

import (
	"testing"
	"time"

	"quizzivy/internal/platform/config"
)

func TestTheAppZoneDefaultsToSaigonAndAnyZoneTheHostKnowsIsRead(t *testing.T) {
	for name, c := range map[string]struct {
		value  string
		offset int
	}{
		"unset":               {"", 7 * 3600},
		"only spaces":         {"  ", 7 * 3600},
		"Saigon":              {"Asia/Ho_Chi_Minh", 7 * 3600},
		"UTC":                 {"UTC", 0},
		"a zone with padding": {" America/Los_Angeles ", -8 * 3600},
	} {
		t.Setenv("APP_TIMEZONE", c.value)
		location, err := config.AppLocation()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		_, offset := time.Date(2090, 1, 15, 12, 0, 0, 0, time.UTC).In(location).Zone()
		if offset != c.offset {
			t.Errorf("%s: offset %d in January, want %d", name, offset, c.offset)
		}
	}
}

func TestAZoneNobodyHasIsRefusedByNameOfTheVariable(t *testing.T) {
	for _, value := range []string{"Mars/Olympus_Mons", "Saigon", "../../etc/passwd"} {
		t.Setenv("APP_TIMEZONE", value)
		if _, err := config.AppLocation(); err == nil {
			t.Errorf("%q was accepted", value)
		}
	}
}
