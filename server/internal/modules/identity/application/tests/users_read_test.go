package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
)

type usersDirectory struct {
	domain.AdminUsers
	items   []domain.AdminUser
	detail  domain.AdminUserDetail
	missing bool
	listed  []domain.AdminUserQuery
	faceted []domain.AdminUserQuery
	failure error
}

func (d *usersDirectory) List(_ context.Context, in domain.AdminUserQuery) ([]domain.AdminUser, paging.Page, error) {
	d.listed = append(d.listed, in)
	return d.items, paging.Page{Number: 1, Size: 20, Total: len(d.items)}, d.failure
}

func (d *usersDirectory) Facets(_ context.Context, in domain.AdminUserQuery) (domain.AdminUserFacets, error) {
	d.faceted = append(d.faceted, in)
	return domain.AdminUserFacets{All: 7, Teachers: 3, Students: 2, Admins: 1, Disabled: 1}, nil
}

func (d *usersDirectory) Detail(context.Context, string) (domain.AdminUserDetail, error) {
	if d.missing {
		return domain.AdminUserDetail{}, domain.ErrUserNotFound
	}
	return d.detail, nil
}

func directoryApp(d *usersDirectory, held map[string]access.Set, store *photoStore) *application.Application {
	app := application.New(nil, nil, time.Hour, nil, nil)
	app.SetPrincipals(&heldPermissions{sets: held})
	if store != nil {
		app.SetAvatars(store, &photoMaker{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	app.SetAdminUsers(d, nil)
	return app
}

func detailOf(id string, status domain.AccountStatus, lastAdmin bool) domain.AdminUserDetail {
	return domain.AdminUserDetail{User: domain.AdminUser{ID: id, Status: status}, LastActiveAdmin: lastAdmin}
}

func TestWhatTheCallerMayDoToAnAccountIsWhatTheGuardsAnswer(t *testing.T) {
	everything := access.NewSet(access.All()...)
	teaching := access.NewSet(access.TeachingGrading)
	manager := access.NewSet(access.PeopleUsersManage)
	gradingManager := access.NewSet(access.PeopleUsersManage, access.TeachingGrading)
	managerOfRoles := access.NewSet(access.PeopleUsersManage, access.PeopleRolesManage, access.TeachingGrading)
	all := func(by domain.WriteRequest) domain.WriteRequest { by.All = true; return by }

	for name, c := range map[string]struct {
		by     domain.WriteRequest
		target access.Set
		detail domain.AdminUserDetail
		want   domain.UserCan
	}{
		"an Admin on a teacher": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: teaching, detail: detailOf("other", domain.AccountActive, false),
			want: domain.UserCan{ChangeRole: true, ResetPassword: true, Disable: true, SignOut: true},
		},
		"an Admin on a disabled teacher": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: teaching, detail: detailOf("other", domain.AccountDisabled, false),
			want: domain.UserCan{ChangeRole: true, Enable: true, SignOut: true, Transfer: true},
		},
		"an Admin on an Admin who is not the last": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: everything, detail: detailOf("other", domain.AccountActive, false),
			want: domain.UserCan{ChangeRole: true, ResetPassword: true, Disable: true, SignOut: true},
		},
		"an Admin on the last active Admin, who is another": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: everything, detail: detailOf("other", domain.AccountActive, true),
			want: domain.UserCan{ResetPassword: true, SignOut: true},
		},
		"an Admin on themselves, the last active Admin": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: everything, detail: detailOf("admin", domain.AccountActive, true),
			want: domain.UserCan{SignOut: true},
		},
		"an Admin on themselves": {
			by: all(domain.WriteRequest{ActorID: "admin", Grants: everything}), target: everything, detail: detailOf("admin", domain.AccountActive, false),
			want: domain.UserCan{SignOut: true},
		},
		"a manager on a teacher with a key the manager lacks": {
			by: domain.WriteRequest{ActorID: "manager", Grants: manager}, target: teaching, detail: detailOf("other", domain.AccountActive, false),
			want: domain.UserCan{},
		},
		"a manager on a teacher whose keys are the manager's": {
			by: domain.WriteRequest{ActorID: "manager", Grants: gradingManager}, target: teaching, detail: detailOf("other", domain.AccountActive, false),
			want: domain.UserCan{ResetPassword: true, Disable: true, SignOut: true},
		},
		"a manager of roles on the same teacher": {
			by: domain.WriteRequest{ActorID: "manager", Grants: managerOfRoles}, target: teaching, detail: detailOf("other", domain.AccountActive, false),
			want: domain.UserCan{ChangeRole: true, ResetPassword: true, Disable: true, SignOut: true},
		},
		"a manager on a student who also holds Take tests": {
			by: domain.WriteRequest{ActorID: "manager", Grants: manager}, target: access.NewSet(access.LearningTakeTests), detail: detailOf("pupil", domain.AccountActive, false),
			want: domain.UserCan{ResetPassword: true, Disable: true, SignOut: true},
		},
		"a caller without scope.all on a disabled student": {
			by: domain.WriteRequest{ActorID: "manager", Grants: manager}, target: access.NewSet(access.LearningTakeTests), detail: detailOf("pupil", domain.AccountDisabled, false),
			want: domain.UserCan{Enable: true, SignOut: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := &usersDirectory{detail: c.detail}
			app := directoryApp(directory, map[string]access.Set{c.detail.User.ID: c.target}, nil)
			got, err := app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: c.by, ID: c.detail.User.ID})
			if err != nil {
				t.Fatal(err)
			}
			if got.Can != c.want {
				t.Errorf("can = %+v, want %+v", got.Can, c.want)
			}
		})
	}
}

func TestAnAccountThatIsGoneIsNotFoundWhetherTheDirectoryOrTheAccessCacheSaysSo(t *testing.T) {
	by := domain.WriteRequest{ActorID: "admin", All: true, Grants: access.NewSet(access.All()...)}
	for name, c := range map[string]struct {
		directory *usersDirectory
		held      map[string]access.Set
	}{
		"no row":                   {&usersDirectory{missing: true}, map[string]access.Set{"ghost": {}}},
		"a row the cache has lost": {&usersDirectory{detail: detailOf("ghost", domain.AccountActive, false)}, map[string]access.Set{}},
	} {
		t.Run(name, func(t *testing.T) {
			app := directoryApp(c.directory, c.held, nil)
			_, err := app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: by, ID: "ghost"})
			if !errors.Is(err, domain.ErrUserNotFound) {
				t.Errorf("err = %v, want ErrUserNotFound", err)
			}
		})
	}
}

func TestAPhotoIsASignedLinkInTheListAndTheSheetAndNothingWithoutAStore(t *testing.T) {
	key := "avatars/u1/photo.png"
	by := domain.WriteRequest{ActorID: "admin", All: true, Grants: access.NewSet(access.All()...)}
	row := domain.AdminUser{ID: "u1", AvatarKey: &key}
	bare := domain.AdminUser{ID: "u2"}

	store := &photoStore{}
	directory := &usersDirectory{items: []domain.AdminUser{row, bare}, detail: domain.AdminUserDetail{User: row}}
	app := directoryApp(directory, map[string]access.Set{"u1": {}}, store)
	listed, err := app.Queries.ListUsers.Handle(context.Background(), query.ListUsers{Query: domain.AdminUserQuery{Tab: domain.TabAll}})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Items[0].AvatarURL != "https://objects.example/"+key || listed.Items[1].AvatarURL != "" {
		t.Errorf("links = %q, %q", listed.Items[0].AvatarURL, listed.Items[1].AvatarURL)
	}
	got, err := app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: by, ID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Detail.User.AvatarURL != "https://objects.example/"+key {
		t.Errorf("link = %q", got.Detail.User.AvatarURL)
	}
	if store.signedTTL != 24*time.Hour || len(store.signed) != 2 {
		t.Errorf("signed %d link(s) for %v, want one per photo for 24h", len(store.signed), store.signedTTL)
	}

	directory = &usersDirectory{items: []domain.AdminUser{row}}
	without, err := directoryApp(directory, nil, nil).Queries.ListUsers.Handle(context.Background(), query.ListUsers{})
	if err != nil {
		t.Fatal(err)
	}
	if without.Items[0].AvatarURL != "" {
		t.Errorf("link = %q without an object store, want none", without.Items[0].AvatarURL)
	}
}

func TestTheListAnswersThePageTheFacetsAndTheSameQueryTwice(t *testing.T) {
	directory := &usersDirectory{items: []domain.AdminUser{{ID: "u1"}}}
	app := directoryApp(directory, nil, nil)
	in := domain.AdminUserQuery{Tab: domain.TabStudents, Status: domain.AccountNeverSignedIn, Query: "mai", Email: "a@b.test", Page: 2, Limit: 10}
	got, err := app.Queries.ListUsers.Handle(context.Background(), query.ListUsers{Query: in})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Page.Total != 1 || got.Facets.All != 7 || got.Facets.Disabled != 1 {
		t.Errorf("answer = %+v", got)
	}
	if len(directory.listed) != 1 || directory.listed[0] != in || len(directory.faceted) != 1 || directory.faceted[0] != in {
		t.Errorf("list saw %+v and facets saw %+v, want the one query both times", directory.listed, directory.faceted)
	}
}

func TestAFailedListIsNotAnEmptyOne(t *testing.T) {
	boom := errors.New("the database is gone")
	app := directoryApp(&usersDirectory{failure: boom}, nil, nil)
	if _, err := app.Queries.ListUsers.Handle(context.Background(), query.ListUsers{}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the directory's failure", err)
	}
}
