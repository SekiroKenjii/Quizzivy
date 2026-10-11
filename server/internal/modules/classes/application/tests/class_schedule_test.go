//go:build integration

package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type fixedZone struct {
	name string
	err  error
}

func (z fixedZone) ZoneOf(context.Context, string) (string, error) { return z.name, z.err }

func text(s string) *string { return &s }

func rotatedAt(t *testing.T, pool *pgxpool.Pool, zones ports.Zones, now time.Time, days int) (domain.Rotated, time.Time) {
	t.Helper()
	svc := newSvc(t, pool)
	if zones != nil {
		svc.WithZones(zones)
	}
	svc.SetClock(func() time.Time { return now })
	classID, teacherID, _ := makeClassRow(t, pool)
	rotated, err := svc.Commands.Rotate.Handle(context.Background(), command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID, ExpiresInDays: &days}})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	var stored time.Time
	if err := pool.QueryRow(context.Background(), `SELECT expires_at FROM app.class_join_codes WHERE class_id = $1 AND revoked_at IS NULL`, classID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	return rotated, stored
}

func TestARotatedCodeExpiresAtTheEndOfItsLastDayInTheActorsZone(t *testing.T) {
	pool := newPool(t)
	jst, _ := time.LoadLocation("Asia/Tokyo")
	berlin, _ := time.LoadLocation("Europe/Berlin")
	hcm, _ := time.LoadLocation(domain.DefaultZone)
	for name, c := range map[string]struct {
		zones ports.Zones
		now   time.Time
		days  int
		want  time.Time
	}{
		"UTC+9 at 23:30 local": {fixedZone{name: "Asia/Tokyo"}, time.Date(2026, 10, 10, 23, 30, 0, 0, jst), 7, time.Date(2026, 10, 17, 23, 59, 59, 0, jst)},
		"UTC+9 at 00:30 local": {fixedZone{name: "Asia/Tokyo"}, time.Date(2026, 10, 11, 0, 30, 0, 0, jst), 7, time.Date(2026, 10, 18, 23, 59, 59, 0, jst)},
		"Berlin across the March change": {
			fixedZone{name: "Europe/Berlin"}, time.Date(2027, 3, 20, 12, 0, 0, 0, berlin), 14, time.Date(2027, 4, 3, 23, 59, 59, 0, berlin),
		},
		"the default zone, 30 days": {fixedZone{name: domain.DefaultZone}, time.Date(2026, 10, 10, 9, 0, 0, 0, hcm), 30, time.Date(2026, 11, 9, 23, 59, 59, 0, hcm)},
		"no zone port":              {nil, time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC), 3, time.Date(2026, 10, 14, 23, 59, 59, 0, hcm)},
		"a zone port that fails":    {fixedZone{err: errors.New("the profile cannot be read")}, time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC), 3, time.Date(2026, 10, 14, 23, 59, 59, 0, hcm)},
		"a zone no one knows":       {fixedZone{name: "Mars/Olympus_Mons"}, time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC), 3, time.Date(2026, 10, 14, 23, 59, 59, 0, hcm)},
	} {
		t.Run(name, func(t *testing.T) {
			rotated, stored := rotatedAt(t, pool, c.zones, c.now, c.days)
			if !rotated.ExpiresAt.Equal(c.want) {
				t.Errorf("rotate answered %s, want %s", rotated.ExpiresAt, c.want)
			}
			if !stored.Equal(c.want) {
				t.Errorf("the stored expiry is %s, want %s", stored, c.want)
			}
		})
	}
}

func TestAClassKeepsItsScheduleAndRoomAsItsTeacherWordedThem(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()
	_, teacherID, _ := makeClassRow(t, pool)
	who := actor.Actor{ID: teacherID, Scope: access.Scope{UserID: teacherID}}

	created, err := svc.Commands.Create.Handle(ctx, command.Create{Name: "Lớp lịch " + nonce(t), ScheduleLabel: text("  Thứ 3, 5 · 18:00 "), Room: text("   "), SelfJoin: true, Actor: who})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, created.ID); err != nil {
			t.Errorf("cleanup audit: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.classes WHERE id = $1::uuid`, created.ID); err != nil {
			t.Errorf("cleanup class: %v", err)
		}
	})
	if created.ScheduleLabel == nil || *created.ScheduleLabel != "Thứ 3, 5 · 18:00" || created.Room != nil || created.AverageScore != nil {
		t.Fatalf("created with schedule %v and room %v, want the trimmed label and no room", created.ScheduleLabel, created.Room)
	}

	update := func(in domain.UpdateInput) domain.Class {
		t.Helper()
		got, err := svc.Commands.Update.Handle(ctx, command.Update{ClassID: created.ID, Input: in, Scope: who.Scope})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := update(domain.UpdateInput{Name: text("Đổi tên")}); got.ScheduleLabel == nil || *got.ScheduleLabel != "Thứ 3, 5 · 18:00" || got.Name != "Đổi tên" {
		t.Errorf("renaming changed the schedule: %v", got.ScheduleLabel)
	}
	if got := update(domain.UpdateInput{Room: domain.TextPatch{Set: true, Value: text(" A1.02 ")}}); got.Room == nil || *got.Room != "A1.02" || got.ScheduleLabel == nil {
		t.Errorf("setting the room gave room %v and schedule %v", got.Room, got.ScheduleLabel)
	}
	if got := update(domain.UpdateInput{ScheduleLabel: domain.TextPatch{Set: true}}); got.ScheduleLabel != nil || got.Room == nil {
		t.Errorf("sending null gave schedule %v and room %v, want the schedule cleared and the room kept", got.ScheduleLabel, got.Room)
	}
	if got := update(domain.UpdateInput{Room: domain.TextPatch{Set: true, Value: text(" \t ")}}); got.Room != nil {
		t.Errorf("a blank room left %v, want it cleared", got.Room)
	}
	read, err := svc.Queries.Get.Handle(ctx, query.Get{ClassID: created.ID, Scope: who.Scope})
	if err != nil || read.ScheduleLabel != nil || read.Room != nil {
		t.Errorf("read back %v and %v (%v), want neither", read.ScheduleLabel, read.Room, err)
	}
}

func TestTheDatabaseRefusesAnEmptyOrOverlongScheduleLabelAndRoom(t *testing.T) {
	pool := newPool(t)
	_, teacherID, _ := makeClassRow(t, pool)
	for name, c := range map[string]struct {
		column string
		value  string
		check  string
	}{
		"an empty label":   {"schedule_label", "", "classes_schedule_label_check"},
		"a 121-char label": {"schedule_label", strings.Repeat("x", 121), "classes_schedule_label_check"},
		"an empty room":    {"room", "", "classes_room_check"},
		"a 61-char room":   {"room", strings.Repeat("x", 61), "classes_room_check"},
	} {
		_, err := pool.Exec(context.Background(), `INSERT INTO app.classes (name, teacher_id, `+c.column+`) VALUES ($1, $2, $3)`, "Lớp kiểm tra "+nonce(t), teacherID, c.value)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23514" || pg.ConstraintName != c.check {
			t.Errorf("%s: %v, want a violation of %s", name, err, c.check)
		}
	}
	for name, c := range map[string]struct{ column, value string }{
		"a 120-char label": {"schedule_label", strings.Repeat("x", 120)},
		"a 60-char room":   {"room", strings.Repeat("x", 60)},
	} {
		var id string
		if err := pool.QueryRow(context.Background(), `INSERT INTO app.classes (name, teacher_id, `+c.column+`) VALUES ($1, $2, $3) RETURNING id::text`, "Lớp biên "+nonce(t), teacherID, c.value).Scan(&id); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.classes WHERE id = $1::uuid`, id); err != nil {
			t.Errorf("cleanup %s: %v", name, err)
		}
	}
}
