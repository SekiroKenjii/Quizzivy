package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Plan is the plan the Admin console's Plan & usage page shows: a name, a renewal date and the limits it states. Every field is optional and a zero limit is no limit. Nothing here is enforced (DG-55): the console shows the figures beside them and refuses nothing.
type Plan struct {
	Name         string
	RenewsOn     time.Time
	SeatLimit    int
	TeacherLimit int
	MediaBytes   int64
	ImportBytes  int64
}

const (
	maxPlanNameRunes = 80
	maxPlanSeats     = 1_000_000
	maxPlanBytes     = int64(1) << 50
	planDateLayout   = "2006-01-02"
)

func loadPlan(cfg *Config) error {
	p := &cfg.Plan
	p.Name = strings.TrimSpace(os.Getenv("PLAN_NAME"))
	if utf8.RuneCountInString(p.Name) > maxPlanNameRunes {
		return fmt.Errorf("PLAN_NAME must be at most %d characters", maxPlanNameRunes)
	}
	if raw := strings.TrimSpace(os.Getenv("PLAN_RENEWS_ON")); raw != "" {
		renews, err := time.Parse(planDateLayout, raw)
		if err != nil {
			return fmt.Errorf("PLAN_RENEWS_ON must be a date such as 2027-01-01, got %q", raw)
		}
		if p.Name == "" {
			return fmt.Errorf("PLAN_RENEWS_ON needs PLAN_NAME: a renewal date belongs to a named plan")
		}
		p.RenewsOn = renews
	}
	var err error
	if p.SeatLimit, err = planLimit("PLAN_SEAT_LIMIT", maxPlanSeats); err != nil {
		return err
	}
	if p.TeacherLimit, err = planLimit("PLAN_TEACHER_LIMIT", maxPlanSeats); err != nil {
		return err
	}
	if p.MediaBytes, err = planBytes("PLAN_MEDIA_BYTES"); err != nil {
		return err
	}
	p.ImportBytes, err = planBytes("PLAN_IMPORT_BYTES")
	return err
}

func planLimit(name string, ceiling int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", name, raw)
	}
	if n < 1 || n > ceiling {
		return 0, fmt.Errorf("%s must be between 1 and %d, or unset for no limit", name, ceiling)
	}
	return n, nil
}

func planBytes(name string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number of bytes, got %q", name, raw)
	}
	if n < 1 || n > maxPlanBytes {
		return 0, fmt.Errorf("%s must be between 1 and %d bytes, or unset for no limit", name, maxPlanBytes)
	}
	return n, nil
}
