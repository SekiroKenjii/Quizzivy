package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
)

type myClasses struct {
	domain.Repository
	rows []domain.MyClass
}

func (m myClasses) ListMine(context.Context, string) ([]domain.MyClass, error) {
	return append([]domain.MyClass(nil), m.rows...), nil
}

type signer struct {
	signed []string
	empty  map[string]bool
	fail   map[string]bool
}

func (s *signer) AvatarURL(_ context.Context, key string) (string, error) {
	s.signed = append(s.signed, key)
	if s.fail[key] {
		return "", errors.New("the store is down")
	}
	if s.empty[key] {
		return "", nil
	}
	return "https://photos.example.test/" + key + "?X-Amz-Signature=sig", nil
}

func TestAStudentsClassesCarryTheirTeachersSignedPhotoWhereThereIsOne(t *testing.T) {
	key := func(s string) *string { return &s }
	rows := []domain.MyClass{
		{ID: "1", Name: "Một", TeacherAvatarKey: key("avatars/a.png"), JoinedAt: time.Now()},
		{ID: "2", Name: "Hai", TeacherAvatarKey: key("avatars/a.png"), JoinedAt: time.Now()},
		{ID: "3", Name: "Ba", JoinedAt: time.Now()},
		{ID: "4", Name: "Bốn", TeacherAvatarKey: key("avatars/b.png"), JoinedAt: time.Now()},
		{ID: "5", Name: "Năm", TeacherAvatarKey: key("avatars/c.png"), JoinedAt: time.Now()},
	}
	photos := &signer{fail: map[string]bool{"avatars/b.png": true}, empty: map[string]bool{"avatars/c.png": true}}
	svc := application.New(myClasses{rows: rows}, nil, domain.JoinCodeKeys{}).WithAvatars(photos)

	got, err := svc.Queries.ListMine.Handle(context.Background(), query.ListMine{UserID: "student"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("%d classes, want all 5: a photo that cannot be signed must not drop a class", len(got))
	}
	wantURL := "https://photos.example.test/avatars/a.png?X-Amz-Signature=sig"
	for i, c := range got {
		switch c.ID {
		case "1", "2":
			if c.TeacherAvatarURL == nil || *c.TeacherAvatarURL != wantURL {
				t.Errorf("class %s: photo %v, want %s", got[i].ID, c.TeacherAvatarURL, wantURL)
			}
		default:
			if c.TeacherAvatarURL != nil {
				t.Errorf("class %s: photo %q, want none", c.ID, *c.TeacherAvatarURL)
			}
		}
	}
	if len(photos.signed) != 3 {
		t.Errorf("signed %v, want one signature for each distinct photo", photos.signed)
	}
}

func TestWithoutASignerNoPhotoIsShown(t *testing.T) {
	key := "avatars/a.png"
	svc := application.New(myClasses{rows: []domain.MyClass{{ID: "1", Name: "Một", TeacherAvatarKey: &key}}}, nil, domain.JoinCodeKeys{})
	got, err := svc.Queries.ListMine.Handle(context.Background(), query.ListMine{UserID: "student"})
	if err != nil || len(got) != 1 || got[0].TeacherAvatarURL != nil {
		t.Errorf("without a signer: %+v (%v)", got, err)
	}
}
