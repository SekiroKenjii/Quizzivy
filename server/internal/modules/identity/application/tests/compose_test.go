package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

type composingShelf struct {
	guardedShelf
	created domain.NewStudent
	patched domain.StudentPatch
	stored  map[string]string
}

func (c *composingShelf) Get(ctx context.Context, scope access.Scope, id string) (domain.Student, error) {
	student, err := c.guardedShelf.Get(ctx, scope, id)
	student.Email = c.stored[id]
	return student, err
}

func (c *composingShelf) Create(_ context.Context, _ domain.WriteRequest, in domain.NewStudent) (domain.Student, error) {
	c.writes = append(c.writes, "create")
	c.created = in
	return domain.Student{}, nil
}

func (c *composingShelf) Update(_ context.Context, _ domain.WriteRequest, in domain.StudentPatch) (domain.Student, error) {
	c.writes = append(c.writes, "update")
	c.patched = in
	return domain.Student{ID: in.ID}, nil
}

func composingStudents() (*application.Application, *composingShelf) {
	shelf := &composingShelf{guardedShelf: guardedShelf{visible: map[string]bool{"pupil": true}}}
	held := &heldPermissions{sets: map[string]access.Set{"pupil": access.NewSet(access.LearningTakeTests)}}
	app := application.New(nil, nil, 0, shelf, noFigures{})
	app.SetPrincipals(held)
	return app, shelf
}

func writer() domain.WriteRequest {
	return domain.WriteRequest{ActorID: "teacher", All: true, Grants: access.NewSet(access.All()...)}
}

func TestACreatedStudentIsStoredWithTheNameComposedAndTheEmailAsGiven(t *testing.T) {
	app, shelf := composingStudents()
	_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{
		Email: norm.NFD.String("Hoc.Sinh.é") + "@Example.com", FullName: norm.NFD.String("Nguyễn Văn Á"), ClassIDs: []string{"class-id"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := shelf.created
	if got.FullName != "Nguyễn Văn Á" || got.Email != norm.NFD.String("Hoc.Sinh.é")+"@Example.com" || got.ClassIDs[0] != "class-id" || got.Hash == "" {
		t.Fatalf("created=%+v", got)
	}
}

func TestAnUpdatedStudentIsStoredWithTheNameComposedAndTheEmailAsGiven(t *testing.T) {
	app, shelf := composingStudents()
	name, email := norm.NFD.String("Trần Thị B"), norm.NFD.String("Mới")+"@Example.com"
	if _, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: domain.StudentPatch{ID: "pupil", FullName: &name, Email: &email}}); err != nil {
		t.Fatal(err)
	}
	got := shelf.patched
	if *got.FullName != "Trần Thị B" || *got.Email != norm.NFD.String("Mới")+"@Example.com" || got.ID != "pupil" {
		t.Fatalf("patched=%+v", got)
	}
}

func TestAStudentNameComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	name := strings.Repeat("क़", domain.MaxFullNameLength)
	app, shelf := composingStudents()
	for label, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{Email: "a@example.com", FullName: name}})
			return err
		},
		"update": func() error {
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: domain.StudentPatch{ID: "pupil", FullName: &name}})
			return err
		},
	} {
		t.Run(label, func(t *testing.T) {
			var invalid *validation.Error
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "fullName" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if len(shelf.writes) != 0 {
		t.Fatalf("a refused name reached the repository: %v", shelf.writes)
	}
}

func TestAStudentWhoseNameOrAddressCannotBeReadIsRefusedOnItsFieldBeforeAnyWrite(t *testing.T) {
	hidden := "a" + string(rune(0x200b)) + "@example.com"
	for label, c := range map[string]struct{ name, email, field string }{
		"five spaces":          {"     ", "a@example.com", "fullName"},
		"a zero-width space":   {"An" + string(rune(0x200b)) + "Bình", "a@example.com", "fullName"},
		"a right-to-left mark": {"An" + string(rune(0x202e)) + "Bình", "a@example.com", "fullName"},
		"a control character":  {"An" + string(rune(0x07)) + "Bình", "a@example.com", "fullName"},
		"a hidden address":     {"An", hidden, "email"},
		"a braille blank":      {"An", "a" + string(rune(0x2800)) + "@example.com", "email"},
		"a no-break space":     {"An", "a" + string(rune(0xa0)) + "b@example.com", "email"},
	} {
		t.Run(label, func(t *testing.T) {
			app, shelf := composingStudents()
			_, createErr := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{Email: c.email, FullName: c.name}})
			patch := domain.StudentPatch{ID: "pupil", FullName: &c.name, Email: &c.email}
			_, updateErr := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: patch})
			for op, err := range map[string]error{"create": createErr, "update": updateErr} {
				var invalid *validation.Error
				if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != c.field {
					t.Errorf("%s: err=%v, want a refusal on %s", op, err, c.field)
				}
			}
			if len(shelf.writes) != 0 {
				t.Fatalf("a refused student reached the repository: %v", shelf.writes)
			}
		})
	}
}

func TestAStudentNameIsStoredTrimmedWithItsSpaceFolded(t *testing.T) {
	app, shelf := composingStudents()
	if _, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{Email: "a@example.com", FullName: "  Nguyễn \t Văn   An "}}); err != nil {
		t.Fatal(err)
	}
	if shelf.created.FullName != "Nguyễn Văn An" {
		t.Fatalf("name = %q", shelf.created.FullName)
	}
}

func TestAProfileStoresItsNamesComposed(t *testing.T) {
	repo := &profileUsers{user: domain.User{ID: "actor"}}
	app := application.New(repo, nil, time.Hour, nil, nil)
	patch := domain.ProfilePatch{FullName: profileName(norm.NFD.String("Nguyễn An")), DisplayNameSet: true, DisplayName: profileName(norm.NFD.String("Cô An")), PhoneSet: true, Phone: profileName("+84 123456")}
	if _, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: patch}); err != nil {
		t.Fatal(err)
	}
	got := repo.got.Patch
	if *got.FullName != "Nguyễn An" || *got.DisplayName != "Cô An" || *got.Phone != "+84 123456" {
		t.Fatalf("patch=%+v", got)
	}
}

func TestAProfileNameComposingLeavesOverItsLimitIsRefusedByTheNamedError(t *testing.T) {
	for label, patch := range map[string]domain.ProfilePatch{
		"full name":    {FullName: profileName(strings.Repeat("क़", 101))},
		"display name": {DisplayNameSet: true, DisplayName: profileName(strings.Repeat("क़", 41))},
	} {
		t.Run(label, func(t *testing.T) {
			repo := &profileUsers{user: domain.User{ID: "actor"}}
			_, err := application.New(repo, nil, time.Hour, nil, nil).Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: patch})
			if !errors.Is(err, domain.ErrNameTooLong) && !errors.Is(err, domain.ErrDisplayNameInvalid) {
				t.Fatalf("err=%v", err)
			}
			if repo.calls != 0 {
				t.Fatal("a refused profile reached the repository")
			}
		})
	}
}

func TestAStudentWhoseStoredAddressHoldsAnInvisibleCharacterCanBeSavedWithItUnchangedButNotWithAnother(t *testing.T) {
	stored := "a" + string(rune(0x2800)) + "@example.com"
	name := "An"
	for label, c := range map[string]struct {
		sent    string
		refused bool
	}{
		"the stored address as it is":       {sent: stored},
		"another address with the same gap": {sent: "b" + string(rune(0x2800)) + "@example.com", refused: true},
		"a clean address":                   {sent: "a@example.com"},
		"a variation selector added":        {sent: "a@example.com" + string(rune(0xfe0f)), refused: true},
		"only the case changed":             {sent: "A" + string(rune(0x2800)) + "@example.com", refused: true},
	} {
		t.Run(label, func(t *testing.T) {
			app, shelf := composingStudents()
			shelf.stored = map[string]string{"pupil": stored}
			patch := domain.StudentPatch{ID: "pupil", FullName: &name, Email: &c.sent}
			_, err := app.Commands.UpdateStudent.Handle(context.Background(), command.UpdateStudent{Request: writer(), Input: patch})
			var invalid *validation.Error
			if c.refused {
				if !errors.As(err, &invalid) || invalid.Fields[0].Field != "email" || len(shelf.writes) != 0 {
					t.Fatalf("err=%v writes=%v, want a refusal on email before any write", err, shelf.writes)
				}
				return
			}
			if err != nil || len(shelf.writes) != 1 {
				t.Fatalf("err=%v writes=%v, want the update made", err, shelf.writes)
			}
		})
	}
}

func TestACreatedStudentWithARefusedNameIsRefusedInBothLanguages(t *testing.T) {
	for label, name := range map[string]string{
		"empty":    "   ",
		"too long": strings.Repeat("a", domain.MaxFullNameLength+1),
		"hidden":   "An" + string(rune(0x200b)),
		"no glyph": "́",
	} {
		t.Run(label, func(t *testing.T) {
			app, _ := composingStudents()
			_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{
				Email: "a@example.com", FullName: name,
			}})
			var invalid *validation.Error
			if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
				t.Fatalf("err=%v", err)
			}
			field := invalid.Fields[0]
			if field.Field != "fullName" || field.Message == "" || field.English == "" || field.English == field.Message {
				t.Fatalf("field=%+v, want a Vietnamese and an English sentence", field)
			}
		})
	}
}

func TestACreatedStudentWithAnInvisibleAddressIsRefusedInTheCallersLanguage(t *testing.T) {
	app, _ := composingStudents()
	_, err := app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{Request: writer(), Input: domain.NewStudent{
		Email: "a" + string(rune(0x200b)) + "@example.com", FullName: "An",
	}})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("err=%v", err)
	}
	field := invalid.Fields[0]
	if !strings.Contains(field.Message, "U+200B") || !strings.Contains(field.English, "U+200B") || field.English == field.Message {
		t.Fatalf("field=%+v, want a Vietnamese and an English sentence naming U+200B", field)
	}
}
