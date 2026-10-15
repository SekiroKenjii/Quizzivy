package config_test

import (
	"strings"
	"testing"
	"time"
)

var planVariables = []string{
	"PLAN_NAME", "PLAN_RENEWS_ON", "PLAN_SEAT_LIMIT", "PLAN_TEACHER_LIMIT", "PLAN_MEDIA_BYTES", "PLAN_IMPORT_BYTES",
}

func TestWithoutAPlanNothingIsNamedOrLimited(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.Plan; p.Name != "" || !p.RenewsOn.IsZero() || p.SeatLimit != 0 || p.TeacherLimit != 0 || p.MediaBytes != 0 || p.ImportBytes != 0 {
		t.Fatalf("defaults: %+v", p)
	}
}

func TestAPlanLoadsEveryFieldItStates(t *testing.T) {
	cfg, err := loadWith(t, map[string]string{
		"PLAN_NAME": " Centre plan ", "PLAN_RENEWS_ON": "2027-01-01", "PLAN_SEAT_LIMIT": "400", "PLAN_TEACHER_LIMIT": "15",
		"PLAN_MEDIA_BYTES": "5368709120", "PLAN_IMPORT_BYTES": "2147483648",
	})
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Plan
	if p.Name != "Centre plan" || !p.RenewsOn.Equal(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)) ||
		p.SeatLimit != 400 || p.TeacherLimit != 15 || p.MediaBytes != 5<<30 || p.ImportBytes != 2<<30 {
		t.Fatalf("loaded: %+v", p)
	}
}

func TestALimitMayStandWithoutAName(t *testing.T) {
	cfg, err := loadWith(t, map[string]string{"PLAN_SEAT_LIMIT": "400"})
	if err != nil || cfg.Plan.Name != "" || cfg.Plan.SeatLimit != 400 {
		t.Fatalf("plan %+v, error %v", cfg.Plan, err)
	}
}

func TestABadPlanValueIsRefusedByName(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"PLAN_NAME": strings.Repeat("a", 81)}, "PLAN_NAME"},
		{map[string]string{"PLAN_NAME": "Centre", "PLAN_RENEWS_ON": "1 Jan 2027"}, "PLAN_RENEWS_ON"},
		{map[string]string{"PLAN_RENEWS_ON": "2027-01-01"}, "PLAN_RENEWS_ON needs PLAN_NAME"},
		{map[string]string{"PLAN_SEAT_LIMIT": "many"}, "PLAN_SEAT_LIMIT"},
		{map[string]string{"PLAN_SEAT_LIMIT": "0"}, "PLAN_SEAT_LIMIT"},
		{map[string]string{"PLAN_TEACHER_LIMIT": "-3"}, "PLAN_TEACHER_LIMIT"},
		{map[string]string{"PLAN_MEDIA_BYTES": "5 GB"}, "PLAN_MEDIA_BYTES"},
		{map[string]string{"PLAN_MEDIA_BYTES": "0"}, "PLAN_MEDIA_BYTES"},
		{map[string]string{"PLAN_IMPORT_BYTES": "99999999999999999999"}, "PLAN_IMPORT_BYTES"},
	}
	for _, c := range cases {
		if _, err := loadWith(t, c.env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: error %v, want one naming %q", c.env, err, c.want)
		}
	}
}
