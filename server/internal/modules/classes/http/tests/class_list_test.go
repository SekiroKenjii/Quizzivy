package http_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/stats"
)

type listed struct {
	asked []query.List
	items []domain.ListedClass
}

func (l *listed) app() classeshttp.Classes {
	return classeshttp.NewClasses(&application.Application{Queries: application.Queries{
		List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
			l.asked = append(l.asked, q)
			return query.ListResult{Items: l.items}, nil
		}),
		Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(context.Context, query.Facets) (domain.Facets, error) {
			return domain.Facets{}, nil
		}),
	}})
}

func withCodes(v bool) *bool { return &v }

func TestTheListAsksForCodesOnlyWhenToldToAndOnlyForAHolderOfClassesWrite(t *testing.T) {
	writer := access.NewSet(access.TeachingClassesWrite)
	reader := access.NewSet(access.PeopleStudentsRead)
	for name, c := range map[string]struct {
		grants access.Set
		ask    *bool
		want   bool
	}{
		"a writer who asks":         {writer, withCodes(true), true},
		"a writer who does not say": {writer, nil, false},
		"a writer who declines":     {writer, withCodes(false), false},
		"a reader who asks":         {reader, withCodes(true), false},
		"a reader who does not say": {reader, nil, false},
	} {
		l := &listed{}
		ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: c.grants})
		response, err := l.app().ListClasses(ctx, openapi.ListClassesRequestObject{Params: openapi.ListClassesParams{WithCodes: c.ask}})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ok, isOK := response.(openapi.ListClasses200JSONResponse)
		if !isOK {
			t.Fatalf("%s answered %T, not an error: a reader who asks for codes simply gets none", name, response)
		}
		if len(l.asked) != 1 || l.asked[0].OpenCodes != c.want {
			t.Errorf("%s: codes opened %v, want %v", name, l.asked, c.want)
		}
		if ok.Headers.CacheControl == nil || *ok.Headers.CacheControl != "no-store" {
			t.Errorf("%s: Cache-Control %v, want no-store whether or not codes were asked for", name, ok.Headers.CacheControl)
		}
	}
}

func TestAListedClassShowsItsCodeGroupedAndItsScheduleAndAverage(t *testing.T) {
	expires := time.Date(2026, 11, 9, 16, 59, 59, 0, time.UTC)
	uses := 40
	room, label := "A1.02", "Thứ 3, 5 · 18:00"
	l := &listed{items: []domain.ListedClass{
		{Class: domain.Class{
			ID: uuid.NewString(), Name: "Có mã", ScheduleLabel: &label, Room: &room,
			JoinCode:     &domain.JoinCodeInfo{Hint: "P9QR", ExpiresAt: expires, MaxUses: &uses, UsesCount: 2},
			AverageScore: &stats.ClassScore{Earned: 19, Total: 30, PendingManual: 1},
		}, Code: "K7M3P9QR"},
		{Class: domain.Class{ID: uuid.NewString(), Name: "Mã cũ", JoinCode: &domain.JoinCodeInfo{Hint: "OLD1", ExpiresAt: expires, Legacy: true}}},
		{Class: domain.Class{ID: uuid.NewString(), Name: "Chưa có mã"}},
	}}
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	response, err := l.app().ListClasses(ctx, openapi.ListClassesRequestObject{Params: openapi.ListClassesParams{WithCodes: withCodes(true)}})
	if err != nil {
		t.Fatal(err)
	}
	items := response.(openapi.ListClasses200JSONResponse).Body.Items
	first, second, third := items[0], items[1], items[2]
	if first.JoinCode == nil || first.JoinCode.Code == nil || *first.JoinCode.Code != "K7M3-P9QR" || first.JoinCode.Legacy || first.JoinCode.UsesCount != 2 || *first.JoinCode.MaxUses != 40 {
		t.Errorf("the sealed code is shown as %+v", first.JoinCode)
	}
	if first.ScheduleLabel == nil || *first.ScheduleLabel != label || first.Room == nil || *first.Room != room {
		t.Errorf("schedule %v room %v", first.ScheduleLabel, first.Room)
	}
	if first.AverageScore == nil || first.AverageScore.Earned != 19 || first.AverageScore.Total != 30 || first.AverageScore.PendingManual != 1 {
		t.Errorf("average %+v", first.AverageScore)
	}
	if second.JoinCode == nil || second.JoinCode.Code != nil || !second.JoinCode.Legacy || second.AverageScore != nil {
		t.Errorf("the legacy code is shown as %+v with average %v", second.JoinCode, second.AverageScore)
	}
	if third.JoinCode != nil {
		t.Errorf("a class with no code shows %+v", third.JoinCode)
	}
}

func TestUpdatingAClassTellsAbsentFromNullFromBlankFromText(t *testing.T) {
	var seen domain.UpdateInput
	app := classeshttp.NewClasses(&application.Application{Commands: application.Commands{
		Update: cqrs.HandlerFunc[command.Update, domain.Class](func(_ context.Context, c command.Update) (domain.Class, error) {
			seen = c.Input
			return domain.Class{}, nil
		}),
	}})
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	for name, c := range map[string]struct {
		label, room json.RawMessage
		wantLabel   domain.TextPatch
		wantRoom    domain.TextPatch
	}{
		"both omitted":    {nil, nil, domain.TextPatch{}, domain.TextPatch{}},
		"null and text":   {json.RawMessage(`null`), json.RawMessage(`"B2"`), domain.TextPatch{Set: true}, domain.TextPatch{Set: true, Value: new("B2")}},
		"blank and empty": {json.RawMessage(`"   "`), json.RawMessage(`""`), domain.TextPatch{Set: true, Value: new("   ")}, domain.TextPatch{Set: true, Value: new("")}},
	} {
		seen = domain.UpdateInput{}
		if _, err := app.UpdateClass(ctx, openapi.UpdateClassRequestObject{Id: uuid.New(), Body: &openapi.UpdateClassJSONRequestBody{ScheduleLabel: c.label, Room: c.room}}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !samePatch(seen.ScheduleLabel, c.wantLabel) || !samePatch(seen.Room, c.wantRoom) {
			t.Errorf("%s: patches %+v and %+v, want %+v and %+v", name, seen.ScheduleLabel, seen.Room, c.wantLabel, c.wantRoom)
		}
	}
}

func samePatch(a, b domain.TextPatch) bool {
	return a.Set == b.Set && ((a.Value == nil) == (b.Value == nil)) && (a.Value == nil || *a.Value == *b.Value)
}

func keysOf(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestAStudentsClassCarriesExactlyTheseFieldsAndNeverACountMemberOrAverage(t *testing.T) {
	label, room, teacher, photo := "Thứ 2 · 18:00", "A1", "Cô Thương", "https://photos.example.test/t.png?X-Amz-Signature=x"
	app := classeshttp.NewClasses(&application.Application{Queries: application.Queries{
		ListMine: cqrs.HandlerFunc[query.ListMine, []domain.MyClass](func(context.Context, query.ListMine) ([]domain.MyClass, error) {
			return []domain.MyClass{
				{ID: uuid.NewString(), Name: "Lớp đầy đủ", TeacherName: &teacher, TeacherAvatarURL: &photo, ScheduleLabel: &label, Room: &room, JoinedAt: time.Now()},
				{ID: uuid.NewString(), Name: "Lớp trống", JoinedAt: time.Now()},
			}, nil
		}),
	}})
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	response, err := app.ListMyClasses(ctx, openapi.ListMyClassesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"description", "id", "joinedAt", "name", "room", "scheduleLabel", "teacherAvatarUrl", "teacherName"}
	for i, item := range response.(openapi.ListMyClasses200JSONResponse).Items {
		if got := keysOf(t, item); !slices.Equal(got, want) {
			t.Errorf("item %d carries %v, want exactly %v: a student's class holds no count, roster, code or average, and a missing value is null, not absent", i, got, want)
		}
	}
	for _, banned := range []string{"studentCount", "openAssignmentCount", "members", "joinCode", "averageScore", "selfJoinEnabled", "archivedAt", "createdAt"} {
		if slices.Contains(want, banned) {
			t.Errorf("%s is a field a student's class must not have", banned)
		}
	}
}

func TestTheJoinResponseDoesNotCarryAnAverageOrACode(t *testing.T) {
	room := "A1"
	got := keysOf(t, classeshttp.ToAPIClass(domain.EnrolledClass{ID: uuid.NewString(), Name: "Lớp", Room: &room, StudentCount: 3, CreatedAt: time.Now()}))
	for _, banned := range []string{"averageScore", "joinCode"} {
		if slices.Contains(got, banned) {
			t.Errorf("the class a student joins carries %s: %v", banned, got)
		}
	}
	if !slices.Contains(got, "room") {
		t.Errorf("the joined class lost its room: %v", got)
	}
}

func TestTheListRowRepeatsEveryPropertyOfAClass(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	names := func(schema string) []string {
		t.Helper()
		out := []string{}
		for name := range spec.Components.Schemas[schema].Value.Properties {
			out = append(out, name)
		}
		slices.Sort(out)
		return out
	}
	if class, row := names("Class"), names("ClassListItem"); !slices.Equal(class, row) {
		t.Errorf("Class has %v and ClassListItem %v: the row repeats the class so that no other response carries a readable code, and the two lists must stay the same", class, row)
	}
	if my := names("MyClass"); !slices.Equal(my, []string{"description", "id", "joinedAt", "name", "room", "scheduleLabel", "teacherAvatarUrl", "teacherName"}) {
		t.Errorf("MyClass declares %v", my)
	}
	class, row := spec.Components.Schemas["Class"].Value, spec.Components.Schemas["ClassListItem"].Value
	extra := []string{}
	for _, name := range row.Required {
		if !slices.Contains(class.Required, name) {
			extra = append(extra, name)
		}
	}
	if !slices.Equal(extra, []string{"averageScore"}) {
		t.Errorf("the row requires %v beyond the class, want only averageScore", extra)
	}
	if info := spec.Components.Schemas["JoinCodeInfo"].Value.Properties; info["code"] != nil {
		t.Error("JoinCodeInfo carries a code: every operation that returns a Class would reveal it")
	}
}
