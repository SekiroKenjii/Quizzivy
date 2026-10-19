package wiring_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"quizzivy/internal/core/wiring"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

type branding struct {
	answer organizationmodel.Branding
	err    error
}

func (b branding) Handle(context.Context, organizationquery.Branding) (organizationmodel.Branding, error) {
	return b.answer, b.err
}

func headerName(t *testing.T, b branding) (string, error) {
	t.Helper()
	cfg := config.Config{Email: config.Email{SinkDir: t.TempDir()}}
	mail, err := wiring.BuildMail(cfg, db.NewContext(nil), b, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("BuildMail: %v", err)
	}
	return mail.Dispatcher.Org(context.Background())
}

func TestEmailHeaderNamesTheCentre(t *testing.T) {
	cases := []struct {
		name string
		in   organizationmodel.Branding
		want string
	}{
		{"the short name first", organizationmodel.Branding{Name: "Trung tâm Anh ngữ Ánh Dương", ShortName: "Ánh Dương"}, "Ánh Dương"},
		{"the name without a short one", organizationmodel.Branding{Name: "Trung tâm Anh ngữ Ánh Dương"}, "Trung tâm Anh ngữ Ánh Dương"},
		{"Quizzivy without either", organizationmodel.Branding{}, "Quizzivy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := headerName(t, branding{answer: c.in})
			if err != nil {
				t.Fatalf("Org: %v", err)
			}
			if got != c.want {
				t.Errorf("Org = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEmailHeaderPassesOnAFailedSettingsRead(t *testing.T) {
	failed := errors.New("settings unavailable")
	if _, err := headerName(t, branding{err: failed}); !errors.Is(err, failed) {
		t.Fatalf("Org error = %v, want %v", err, failed)
	}
}
