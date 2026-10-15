package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/command"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

type memoryStore struct {
	t       *testing.T
	roles   map[string]domain.RoleRow
	opened  int
	granted map[string][]access.Key
	revoked map[string][]access.Key
	audits  []domain.AuditEntry
	edits   []domain.RoleEdit
	created []domain.NewRole
	failOn  error
}

func newMemoryStore(t *testing.T, rows ...domain.RoleRow) *memoryStore {
	s := &memoryStore{t: t, roles: map[string]domain.RoleRow{}, granted: map[string][]access.Key{}, revoked: map[string][]access.Key{}}
	for _, r := range rows {
		s.roles[r.ID] = r
	}
	return s
}

func (s *memoryStore) InTx(ctx context.Context, fn func(context.Context, domain.RoleTx) error) error {
	s.opened++
	return fn(ctx, s)
}

func (s *memoryStore) Lock(_ context.Context, ids []string) ([]domain.RoleRow, error) {
	var out []domain.RoleRow
	for _, id := range ids {
		if r, ok := s.roles[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *memoryStore) LockShared(_ context.Context, id string) (domain.RoleRow, error) {
	r, ok := s.roles[id]
	if !ok {
		return domain.RoleRow{}, domain.ErrRoleNotFound
	}
	return r, nil
}

func (s *memoryStore) Row(ctx context.Context, id string) (domain.RoleRow, error) {
	return s.LockShared(ctx, id)
}

func (s *memoryStore) Insert(_ context.Context, role domain.NewRole) (string, error) {
	if s.failOn != nil {
		return "", s.failOn
	}
	s.created = append(s.created, role)
	id := "role-new"
	s.roles[id] = domain.RoleRow{Role: domain.Role{ID: id, Revision: 1}, Name: role.Name}
	return id, nil
}

func (s *memoryStore) Update(_ context.Context, edit domain.RoleEdit) error {
	if s.failOn != nil {
		return s.failOn
	}
	s.edits = append(s.edits, edit)
	return nil
}

func (s *memoryStore) Grant(_ context.Context, id string, keys []access.Key) error {
	s.granted[id] = append(s.granted[id], keys...)
	return nil
}

func (s *memoryStore) Revoke(_ context.Context, id string, keys []access.Key) error {
	s.revoked[id] = append(s.revoked[id], keys...)
	return nil
}

func (s *memoryStore) Audit(_ context.Context, entry domain.AuditEntry) error {
	s.audits = append(s.audits, entry)
	return nil
}

var (
	rowAdmin     = domain.RoleRow{Role: domain.Role{ID: "role-admin", Builtin: access.BuiltinAdmin, Revision: 1}, Name: "Admin"}
	rowTeacher   = domain.RoleRow{Role: domain.Role{ID: "role-teacher", Builtin: access.BuiltinTeacher, Revision: 2}, Name: "Teacher", Grants: access.NewSet(access.TeachingGrading, access.ContentTestsWrite)}
	rowAssistant = domain.RoleRow{Role: domain.Role{ID: "role-assistant", Builtin: access.BuiltinAssistant, Revision: 1}, Name: "Assistant", Grants: access.NewSet(access.TeachingGrading)}
	rowCustom    = domain.RoleRow{Role: domain.Role{ID: "role-custom", Revision: 3}, Name: "Custom", Description: "d", Icon: "star", Color: "amber", Grants: access.NewSet(access.TeachingGrading)}
)

func adminCaller() domain.Caller {
	return domain.Caller{Actor: actor.Actor{ID: "owner", IP: "203.0.113.9"}, RoleID: "role-admin", Permissions: domain.NewGuard(access.All()).Effective(rowAdmin)}
}

func roleApp(t *testing.T, store *memoryStore) *application.Application {
	t.Helper()
	repo := newRepo()
	repo.roles = []domain.RoleRow{rowAdmin, rowTeacher, rowAssistant, rowCustom, {Role: domain.Role{ID: "role-new", Revision: 1}, Name: "Mới"}}
	return application.New(repo).WithRoleStore(store)
}

func refusedFields(t *testing.T, err error) []string {
	t.Helper()
	var invalid *validation.Error
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want a validation error", err)
	}
	var fields []string
	for _, f := range invalid.Fields {
		fields = append(fields, f.Field)
	}
	return fields
}

func TestACommandWithBadInputOpensNoTransaction(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowTeacher, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	ctx := context.Background()
	blank, long, text := "  ", strings.Repeat("a", 61), strings.Repeat("d", 301)
	invisible, broken := "Tổ\u200bchấm", "Tổ\nchấm"
	var changes []domain.GrantChange
	for range command.MaxGrantChanges + 1 {
		changes = append(changes, domain.GrantChange{RoleID: "role-custom", Key: access.TeachingGrading, Granted: true})
	}
	for _, c := range []struct {
		name string
		run  func() error
		want []string
	}{
		{"a blank name", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: blank})
			return err
		}, []string{"name"}},
		{"a name of 61 characters and a long description", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: long, Description: text})
			return err
		}, []string{"name", "description"}},
		{"a name with a zero-width space", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: invisible})
			return err
		}, []string{"name"}},
		{"a name that is only a zero-width space", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\u200b"})
			return err
		}, []string{"name"}},
		{"a name with a line break", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: broken})
			return err
		}, []string{"name"}},
		{"a name that is only the Hangul filler", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\u3164"})
			return err
		}, []string{"name"}},
		{"a name that is only the Braille blank", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\u2800"})
			return err
		}, []string{"name"}},
		{"a name that is only a Hangul choseong filler", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\u115f"})
			return err
		}, []string{"name"}},
		{"a name that is only a combining accent", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\u0301"})
			return err
		}, []string{"name"}},
		{"a name that is only a variation selector", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "\ufe0f"})
			return err
		}, []string{"name"}},
		{"a name with a Hangul filler beside letters", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Tổ\u3164chấm"})
			return err
		}, []string{"name"}},
		{"a name with a private-use character", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Tổ\ue000chấm"})
			return err
		}, []string{"name"}},
		{"a name with a non-character", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Tổ\ufffechấm"})
			return err
		}, []string{"name"}},
		{"an update to a name that is only a combining accent", func() error {
			name := "\u0301"
			_, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Name: &name})
			return err
		}, []string{"name"}},
		{"a name of 61 characters typed decomposed", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: strings.Repeat("e\u0323\u0302", 61)})
			return err
		}, []string{"name"}},
		{"a description with a zero-width space", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Mới", Description: "Chấm\u200bbài"})
			return err
		}, []string{"description"}},
		{"a description with a tab", func() error {
			_, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Mới", Description: "Chấm\tbài"})
			return err
		}, []string{"description"}},
		{"an update to a description with a line separator", func() error {
			text := "Chấm\u2028bài"
			_, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Description: &text})
			return err
		}, []string{"description"}},
		{"an update to a name with a line separator", func() error {
			name := "Tổ\u2028chấm"
			_, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Name: &name})
			return err
		}, []string{"name"}},
		{"an update that changes nothing named", func() error {
			_, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3})
			return err
		}, []string{"name"}},
		{"an update to a blank name", func() error {
			_, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Name: &blank})
			return err
		}, []string{"name"}},
		{"a save of no changes", func() error {
			_, err := app.Commands.SetRolePermissions.Handle(ctx, command.SetRolePermissions{By: by})
			return err
		}, []string{"changes"}},
		{"a save of 201 changes", func() error {
			_, err := app.Commands.SetRolePermissions.Handle(ctx, command.SetRolePermissions{By: by, Changes: changes})
			return err
		}, []string{"changes"}},
		{"a cell granted and revoked", func() error {
			_, err := app.Commands.SetRolePermissions.Handle(ctx, command.SetRolePermissions{By: by, Changes: []domain.GrantChange{
				{RoleID: "role-custom", Key: access.TeachingGrading, Granted: true}, {RoleID: "role-custom", Key: access.TeachingGrading, Granted: false},
			}})
			return err
		}, []string{"changes"}},
		{"two revisions for one role", func() error {
			_, err := app.Commands.SetRolePermissions.Handle(ctx, command.SetRolePermissions{
				By:       by,
				Changes:  []domain.GrantChange{{RoleID: "role-custom", Key: access.TeachingGrading, Granted: true}},
				Expected: []command.RoleRevision{{RoleID: "role-custom", Revision: 3}, {RoleID: "role-custom", Revision: 4}},
			})
			return err
		}, []string{"expectedRevisions"}},
	} {
		if got := refusedFields(t, c.run()); !slices.Equal(got, c.want) {
			t.Errorf("%s: refused %v, want %v", c.name, got, c.want)
		}
	}
	if store.opened != 0 {
		t.Errorf("%d transactions were opened for input the command refuses up front", store.opened)
	}
}

func TestACreatedRoleIsTrimmedCopiedAndAudited(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowTeacher, rowAssistant, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	if _, err := app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: by, Name: "  Tổ trưởng  ", Description: " Chấm bài ", CopyFrom: "role-teacher"}); err != nil {
		t.Fatal(err)
	}
	if len(store.created) != 1 || store.created[0].Name != "Tổ trưởng" || store.created[0].Description != "Chấm bài" || store.created[0].CopiedFrom != "role-teacher" {
		t.Fatalf("inserted %+v", store.created)
	}
	if got := store.granted["role-new"]; !slices.Equal(got, rowTeacher.Grants.Keys()) {
		t.Errorf("the new role started with %v, want the Teacher's %v", got, rowTeacher.Grants.Keys())
	}
	if len(store.audits) != 1 || store.audits[0].Action != domain.AuditRoleCreated || store.audits[0].RoleID != "role-new" || store.audits[0].By.ID != "owner" {
		t.Errorf("audits = %+v, want one role.created by the owner", store.audits)
	}
	if !strings.Contains(string(store.audits[0].Diff), `"copiedFrom":"role-teacher"`) {
		t.Errorf("the audit diff %s does not name the source", store.audits[0].Diff)
	}
}

func TestANameIsStoredComposedWhateverFormItArrivesIn(t *testing.T) {
	decomposed, composed := "Gia\u0301o vie\u0302n", "Giáo viên"
	if decomposed == composed {
		t.Fatal("the fixture is not decomposed")
	}
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	ctx := context.Background()
	if _, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: " " + decomposed + " ", Description: "Mô ta\u0309"}); err != nil {
		t.Fatal(err)
	}
	if len(store.created) != 1 || store.created[0].Name != composed || store.created[0].Description != "Mô tả" {
		t.Errorf("inserted %+v, want the name %q and the description composed", store.created, composed)
	}
	if _, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Name: &decomposed}); err != nil {
		t.Fatal(err)
	}
	if got := store.edits[len(store.edits)-1]; got.Name == nil || *got.Name != composed {
		t.Errorf("the edit wrote the name %v, want %q", got.Name, composed)
	}
}

func TestANameIsMeasuredAfterItIsComposedAndNeedsAGlyph(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	ctx := context.Background()
	names := []string{strings.Repeat("e\u0323\u0302", 60), "\u2764\ufe0f", "2026", "\u2014", "Tổ 1 & 2"}
	for _, name := range names {
		if _, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: name}); err != nil {
			t.Errorf("name %q: %v, want it accepted", name, err)
		}
	}
	if len(store.created) != len(names) {
		t.Fatalf("%d roles were inserted, want %d", len(store.created), len(names))
	}
	if want := strings.Repeat("ệ", 60); store.created[0].Name != want {
		t.Errorf("a 60-character name typed decomposed was stored as %q, want %q", store.created[0].Name, want)
	}
}

func TestADescriptionMayHoldLineBreaks(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	ctx := context.Background()
	text := "Dòng một\nDòng hai"
	if _, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Mới", Description: text}); err != nil {
		t.Fatal(err)
	}
	if len(store.created) != 1 || store.created[0].Description != text {
		t.Errorf("inserted %+v, want the description %q kept", store.created, text)
	}
	if _, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Description: &text}); err != nil {
		t.Errorf("an update to a description with a line break: %v", err)
	}
}

func TestADescriptionStoresEveryLineEndingAsLF(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	by := adminCaller()
	ctx := context.Background()
	for _, sent := range []string{"Dòng một\r\nDòng hai", "Dòng một\rDòng hai", "Dòng một\nDòng hai"} {
		if _, err := app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Mới", Description: sent}); err != nil {
			t.Errorf("description %q: %v, want it accepted", sent, err)
		}
		if _, err := app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: "role-custom", Expected: 3, Description: &sent}); err != nil {
			t.Errorf("an update to the description %q: %v, want it accepted", sent, err)
		}
	}
	for i, c := range store.created {
		if c.Description != "Dòng một\nDòng hai" {
			t.Errorf("role %d was stored with the description %q, want the line ending as LF", i, c.Description)
		}
	}
	for i, e := range store.edits {
		if e.Description == nil || *e.Description != "Dòng một\nDòng hai" {
			t.Errorf("edit %d wrote the description %v, want the line ending as LF", i, e.Description)
		}
	}
	if len(store.created) != 3 || len(store.edits) != 3 {
		t.Errorf("%d roles and %d edits were written, want 3 and 3", len(store.created), len(store.edits))
	}
}

func TestARefusedNameSaysWhichCharacterIsInvisible(t *testing.T) {
	app := roleApp(t, newMemoryStore(t, rowAdmin, rowCustom))
	_, err := app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: adminCaller(), Name: "Tổ\u200dchấm"})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 {
		t.Fatalf("error = %v, want a validation error on one field", err)
	}
	if got := invalid.Fields[0].Message; !strings.Contains(got, "U+200D") {
		t.Errorf("the message %q does not name U+200D", got)
	}
}

func TestACopyFromTheAssistantOrAMissingRoleWritesNothing(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowAssistant)
	app := roleApp(t, store)
	for source, want := range map[string]error{"role-assistant": domain.ErrNotAssignable, "role-missing": domain.ErrRoleNotFound} {
		_, err := app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: adminCaller(), Name: "Mới", CopyFrom: source})
		if !errors.Is(err, want) {
			t.Errorf("copy from %s: %v, want %v", source, err, want)
		}
	}
	if len(store.created) != 0 || len(store.audits) != 0 {
		t.Errorf("a refused create wrote %d roles and %d audit rows", len(store.created), len(store.audits))
	}
}

func TestAnEditThatChangesNothingWritesNothing(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	same := "Custom"
	if _, err := app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: adminCaller(), ID: "role-custom", Expected: 3, Name: &same}); err != nil {
		t.Fatal(err)
	}
	if len(store.edits) != 0 || len(store.audits) != 0 {
		t.Errorf("an unchanged edit wrote %d edits and %d audit rows", len(store.edits), len(store.audits))
	}
}

func TestAnEditWritesOnlyWhatChangedAndNamesItInTheAudit(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	renamed, same, rose := " Tổ chấm ", "d", "rose"
	if _, err := app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: adminCaller(), ID: "role-custom", Expected: 3, Name: &renamed, Description: &same, Color: &rose}); err != nil {
		t.Fatal(err)
	}
	if len(store.edits) != 1 {
		t.Fatalf("%d edits", len(store.edits))
	}
	edit := store.edits[0]
	if edit.Name == nil || *edit.Name != "Tổ chấm" || edit.Color == nil || *edit.Color != "rose" || edit.Description != nil || edit.Icon != nil {
		t.Errorf("edit = %+v, want the name and the colour only", edit)
	}
	diff := string(store.audits[0].Diff)
	if store.audits[0].Action != domain.AuditRoleUpdated || !strings.Contains(diff, `"color":{"from":"amber","to":"rose"}`) || strings.Contains(diff, "description") {
		t.Errorf("audit %s %s", store.audits[0].Action, diff)
	}
}

func TestAnEditIsRefusedForAStaleRevisionAMissingRoleAndATakenName(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	name := "Khác"
	for _, c := range []struct {
		id       string
		revision int64
		fail     error
		want     error
	}{
		{"role-custom", 2, nil, domain.ErrStaleWrite},
		{"role-missing", 1, nil, domain.ErrRoleNotFound},
		{"role-custom", 3, domain.ErrNameTaken, domain.ErrNameTaken},
	} {
		store.failOn = c.fail
		if _, err := app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: adminCaller(), ID: c.id, Expected: c.revision, Name: &name}); !errors.Is(err, c.want) {
			t.Errorf("%s at %d: %v, want %v", c.id, c.revision, err, c.want)
		}
	}
	if len(store.audits) != 0 {
		t.Errorf("a refused edit wrote %d audit rows", len(store.audits))
	}
}

func TestASaveWritesTheRealChangesAuditsEachRoleAndForgetsThePrincipals(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowTeacher, rowCustom)
	app := roleApp(t, store)
	resolve(t, app, "owner")
	if app.Cached() != 1 {
		t.Fatalf("%d principals cached before the save", app.Cached())
	}
	roles, err := app.Commands.SetRolePermissions.Handle(context.Background(), command.SetRolePermissions{
		By: adminCaller(),
		Changes: []domain.GrantChange{
			{RoleID: "role-custom", Key: access.ContentMediaWrite, Granted: true},
			{RoleID: "role-custom", Key: access.TeachingGrading, Granted: true},
			{RoleID: "role-custom", Key: access.ContentMediaWrite, Granted: true},
			{RoleID: "role-teacher", Key: access.ContentTestsWrite, Granted: false},
		},
		Expected: []command.RoleRevision{{RoleID: "role-custom", Revision: 3}, {RoleID: "role-teacher", Revision: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.granted["role-custom"]; !slices.Equal(got, []access.Key{access.ContentMediaWrite}) {
		t.Errorf("granted %v to the custom role, want only the media key", got)
	}
	if got := store.revoked["role-teacher"]; !slices.Equal(got, []access.Key{access.ContentTestsWrite}) {
		t.Errorf("revoked %v from the Teacher", got)
	}
	if len(store.audits) != 2 {
		t.Fatalf("%d audit rows, want one per changed role", len(store.audits))
	}
	for _, a := range store.audits {
		if a.Action != domain.AuditRolePermissionsChanged || a.By.ID != "owner" {
			t.Errorf("audit %+v", a)
		}
	}
	if app.Cached() != 0 {
		t.Errorf("%d principals still cached after the save", app.Cached())
	}
	if ids := []string{roles[0].ID, roles[1].ID}; len(roles) != 2 || !slices.Contains(ids, "role-custom") || !slices.Contains(ids, "role-teacher") {
		t.Errorf("answered %d roles, want the two touched", len(roles))
	}
}

func TestASaveThatChangesNothingWritesAndForgetsNothing(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	resolve(t, app, "owner")
	_, err := app.Commands.SetRolePermissions.Handle(context.Background(), command.SetRolePermissions{
		By:       adminCaller(),
		Changes:  []domain.GrantChange{{RoleID: "role-custom", Key: access.TeachingGrading, Granted: true}, {RoleID: "role-custom", Key: access.ContentShare, Granted: false}},
		Expected: []command.RoleRevision{{RoleID: "role-custom", Revision: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.granted)+len(store.revoked)+len(store.audits) != 0 {
		t.Errorf("a save that changes nothing wrote %v %v %v", store.granted, store.revoked, store.audits)
	}
	if app.Cached() != 1 {
		t.Errorf("%d principals cached, the save should not have forgotten them", app.Cached())
	}
}

func TestARefusedSaveWritesNothingForAnyRole(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowTeacher, rowCustom)
	app := roleApp(t, store)
	_, err := app.Commands.SetRolePermissions.Handle(context.Background(), command.SetRolePermissions{
		By: adminCaller(),
		Changes: []domain.GrantChange{
			{RoleID: "role-custom", Key: access.ContentMediaWrite, Granted: true},
			{RoleID: "role-admin", Key: access.ContentMediaWrite, Granted: false},
		},
		Expected: []command.RoleRevision{{RoleID: "role-custom", Revision: 3}, {RoleID: "role-admin", Revision: 1}},
	})
	if !errors.Is(err, domain.ErrRoleLocked) {
		t.Fatalf("error = %v, want ErrRoleLocked", err)
	}
	if len(store.granted)+len(store.revoked)+len(store.audits) != 0 {
		t.Errorf("a refused save wrote %v %v %v", store.granted, store.revoked, store.audits)
	}
}

func TestTheStoreKeepsTheClockOfTheService(t *testing.T) {
	store := newMemoryStore(t, rowAdmin, rowCustom)
	app := roleApp(t, store)
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	app.SetClock(func() time.Time { return at })
	name := "Mới"
	if _, err := app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: adminCaller(), ID: "role-custom", Expected: 3, Name: &name}); err != nil {
		t.Fatal(err)
	}
	if !store.audits[0].At.Equal(at) {
		t.Errorf("audit at %v, want %v", store.audits[0].At, at)
	}
}
