//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/core/adapters"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"sort"
	"testing"
	"time"
)

func metadataBank(t *testing.T) (context.Context, *pgxpool.Pool, pgx.Tx, string, string) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var version int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&version); err != nil || version < 180000 {
		t.Fatalf("PG18 required: %d %v", version, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{uuid.NewString(), uuid.NewString()}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("metadata rollback: %v", err)
			return
		}
		var remaining int
		err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM app.users WHERE id=ANY($1::uuid[]))+(SELECT count(*) FROM app.questions WHERE created_by=ANY($1::uuid[]))+(SELECT count(*) FROM app.audit_log WHERE actor_user_id=ANY($1::uuid[]))+(SELECT count(*) FROM app.question_groups WHERE created_by=ANY($1::uuid[]))+(SELECT count(*) FROM app.media_assets WHERE uploaded_by=ANY($1::uuid[]))`, ids).Scan(&remaining)
		if err != nil || remaining != 0 {
			t.Errorf("metadata exact-owned absence: count=%d err=%v", remaining, err)
		}
		t.Logf("metadata rollback checked owners=%v", ids)
	})
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Metadata teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, id, id+"@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool, tx, ids[0], ids[1]
}

func TestMetadataFacetsSelfExcludeAndTagMatchPreserveScope(t *testing.T) {
	ctx, _, tx, owner, foreign := metadataBank(t)
	repo := repositories.NewPostgres(db.NewContext(tx))
	marker := uuid.NewString()
	add := func(by string, typ domain.Type, level *domain.Level, skill *domain.Skill, tags []string) string {
		t.Helper()
		in := domain.Input{Type: typ, Prompt: marker + " phát âm", Points: "1", Tags: tags, Level: level, Skill: skill}
		if typ.IsChoice() {
			in.Options = []domain.OptionInput{{Text: "yes", IsCorrect: true}, {Text: "no"}}
		}
		q, err := repo.Create(ctx, domain.WriteInput{Input: in, ActorID: by, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		return q.ID
	}
	a1, a2, c2 := domain.Level("a1"), domain.Level("a2"), domain.Level("c2")
	grammar, reading, speaking := domain.Skill("grammar"), domain.Skill("reading"), domain.Skill("speaking")
	first := add(owner, domain.SingleChoice, &a1, &grammar, []string{"one", "two"})
	second := add(owner, domain.MultipleChoice, &a2, &reading, []string{"one"})
	third := add(owner, domain.ShortAnswer, nil, &reading, []string{"two"})
	fourth := add(owner, domain.ShortAnswer, &c2, &speaking, []string{"one", "two"})
	other := add(foreign, domain.SingleChoice, &a1, &grammar, []string{"one", "two"})
	in := domain.ListInput{Scope: access.Scope{UserID: owner}, Query: marker, Tags: []string{"one", "two"}}
	assertIDs := func(want ...string) {
		t.Helper()
		rows, page, err := repo.List(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, q := range rows {
			got[q.ID] = true
		}
		expected := map[string]bool{}
		for _, id := range want {
			expected[id] = true
		}
		if !reflect.DeepEqual(got, expected) || page.Total != len(want) {
			t.Fatalf("exact metadata population got=%v total=%d want=%v", got, page.Total, expected)
		}
	}
	assertIDs(first, second, third, fourth)
	in.TagMatch = "all"
	assertIDs(first, fourth)
	in.Levels = []domain.Level{a1, a2}
	in.Skills = []domain.Skill{grammar, reading}
	assertIDs(first)
	in.Scope = access.Scope{All: true}
	assertIDs(first, other)
	in.Scope = access.Scope{}
	assertIDs()
	in.Scope = access.Scope{UserID: owner}
	in.Tags = nil
	in.TagMatch = "all"
	assertIDs(first, second)
	in.Types = []domain.Type{domain.SingleChoice}
	in.Levels = []domain.Level{a2}
	in.Skills = []domain.Skill{grammar}
	facets, err := repo.Facets(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if facets.All != 0 || facets.ByLevel[a1] != 1 || len(facets.ByLevel) != 1 || len(facets.BySkill) != 0 {
		t.Fatalf("level self-exclusion: %+v", facets)
	}
	in.Levels = []domain.Level{a1}
	in.Skills = []domain.Skill{reading}
	facets, err = repo.Facets(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if facets.BySkill[grammar] != 1 || len(facets.BySkill) != 1 || len(facets.ByLevel) != 0 {
		t.Fatalf("skill self-exclusion: %+v", facets)
	}
	in.Skills = []domain.Skill{grammar}
	in.Types = []domain.Type{domain.ShortAnswer}
	facets, err = repo.Facets(ctx, in)
	if err != nil || facets.All != 1 || facets.ByType[domain.SingleChoice] != 1 {
		t.Fatalf("type self-exclusion %+v %v", facets, err)
	}
	in.Types = nil
	in.Tags = []string{"missing"}
	tags, err := repo.Tags(ctx, in)
	if err != nil || !reflect.DeepEqual(tags, []string{"one", "two"}) {
		t.Fatalf("tag self-exclusion %v %v", tags, err)
	}
	in.Tags = nil
	in.Levels = nil
	in.Skills = nil
	in.Query = marker + " phat am"
	assertIDs(first, second, third, fourth)
	q, err := repo.Get(ctx, access.Scope{UserID: owner}, first)
	if err != nil || q.Level == nil || *q.Level != a1 || q.Skill == nil || *q.Skill != grammar {
		t.Fatalf("metadata read %+v %v", q, err)
	}
	if _, err := repo.Get(ctx, access.Scope{UserID: owner}, other); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign read %v", err)
	}
	q, err = repo.Update(ctx, domain.WriteInput{ID: first, Input: domain.Input{Type: domain.ShortAnswer, Prompt: marker, Points: "1", Tags: []string{}}, ActorID: owner, Now: time.Now()})
	if err != nil || q.Level != nil || q.Skill != nil {
		t.Fatalf("replacement unset %+v %v", q, err)
	}
}

func TestMetadataCountsAndPaginationCombineAudioAndExactTags(t *testing.T) {
	ctx, _, tx, owner, foreign := metadataBank(t)
	dbx := db.NewContext(tx)
	repo := repositories.NewPostgres(dbx)
	marker := uuid.NewString()
	a1, a2 := domain.Level("a1"), domain.Level("a2")
	grammar, reading := domain.Skill("grammar"), domain.Skill("reading")
	now := time.Now()
	duration := 1000
	asset, err := mediarepo.Insert(ctx, tx, mediadomain.InsertInput{ID: uuid.NewString(), Kind: mediadomain.KindAudio, StorageKey: marker, MimeType: "audio/mpeg", Bytes: 1, DurationMs: &duration, OriginalFilename: "fixture.mp3", ChecksumSHA256: make([]byte, 32), UploaderID: owner, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	add := func(by string, audio bool, level domain.Level, skill domain.Skill, tags []string) string {
		t.Helper()
		input := domain.Input{Type: domain.ShortAnswer, Prompt: marker, Points: "1", Tags: tags, Level: &level, Skill: &skill}
		var kind *string
		if audio {
			value := "audio"
			kind = &value
			input.MediaAssetID = &asset.ID
			input.Audio = &domain.AudioPolicy{}
		}
		q, err := repo.Create(ctx, domain.WriteInput{Input: input, MediaAssetKind: kind, ActorID: by, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		return q.ID
	}
	first := add(owner, true, a1, grammar, []string{"nghé", "shared"})
	second := add(owner, true, a1, grammar, []string{"nghé"})
	plain := add(owner, false, a1, grammar, []string{"nghé"})
	unaccented := add(owner, true, a1, grammar, []string{"nghe"})
	decomposed := add(owner, true, a1, grammar, []string{"nghe\u0301"})
	add(owner, true, a2, reading, []string{"nghé"})
	deleted := add(owner, true, a1, grammar, []string{"nghé"})
	if err := repo.SoftDelete(ctx, domain.WriteInput{ID: deleted, ActorID: owner, Now: now}); err != nil {
		t.Fatal(err)
	}
	add(foreign, false, a1, grammar, []string{"nghé"})
	member := uuid.NewString()
	groups := testsrepo.NewGroupsPostgres(dbx, adapters.GroupQuestions{}, mediarepo.NewPostgres(dbx))
	_, err = groups.Create(ctx, testsdomain.CreateGroupInput{Bundle: testsdomain.GroupBundle{Group: testsdomain.QuestionGroup{ID: uuid.NewString(), Title: marker, Members: []testsdomain.GroupMember{{QuestionID: member, OptionOrder: "fixed"}}}, Questions: []testsdomain.GroupQuestion{{ID: member, Input: domain.Input{Type: domain.ShortAnswer, Prompt: marker, Points: "1", Tags: []string{"nghé"}, Level: &a1, Skill: &grammar}}}}, ActorID: owner, Scope: access.Scope{UserID: owner}, Grants: access.NewSet(access.ContentQuestionsWrite), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	audio := true
	in := domain.ListInput{Scope: access.Scope{UserID: owner}, Query: marker, Types: []domain.Type{domain.ShortAnswer}, Levels: []domain.Level{a1, a1}, Skills: []domain.Skill{grammar, grammar}, Tags: []string{"nghé", "nghé"}, HasAudio: &audio, Limit: 1}
	assertPopulation := func(want ...string) {
		t.Helper()
		total, filtered, err := repo.Counts(ctx, in)
		if err != nil || total != 6 || filtered != len(want) {
			t.Fatalf("unfiltered bank/filtered count=%d/%d want=6/%d err=%v", total, filtered, len(want), err)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(want)))
		for page := 1; page <= len(want)+1; page++ {
			in.Page = page
			rows, paging, err := repo.List(ctx, in)
			if err != nil || paging.Total != len(want) || paging.Number != page || paging.Size != 1 {
				t.Fatalf("page=%d total=%d size=%d err=%v", paging.Number, paging.Total, paging.Size, err)
			}
			if page <= len(want) {
				if len(rows) != 1 || rows[0].ID != want[page-1] {
					t.Fatalf("page %d exact ID want=%s rows=%+v", page, want[page-1], rows)
				}
			} else if len(rows) != 0 {
				t.Fatalf("past-end page %d contains %+v", page, rows)
			}
		}
	}
	assertPopulation(first, second)
	facets, err := repo.Facets(ctx, in)
	if err != nil || facets.All != 2 || !reflect.DeepEqual(facets.ByType, map[domain.Type]int{domain.ShortAnswer: 2}) || !reflect.DeepEqual(facets.ByLevel, map[domain.Level]int{a1: 2}) || !reflect.DeepEqual(facets.BySkill, map[domain.Skill]int{grammar: 2}) {
		t.Fatalf("combined audio metadata facets=%+v err=%v", facets, err)
	}
	in.Tags = []string{"nghé", "shared"}
	in.TagMatch = "all"
	assertPopulation(first)
	in.TagMatch = "any"
	in.Tags = []string{"nghe"}
	assertPopulation(unaccented)
	in.Tags = []string{"nghe\u0301"}
	assertPopulation(decomposed)
	in.Tags = []string{"nghé"}
	in.HasAudio = nil
	assertPopulation(first, second, plain)
	in.Scope = access.Scope{}
	total, filtered, err := repo.Counts(ctx, in)
	if err != nil || total != 0 || filtered != 0 {
		t.Fatalf("zero scope counts=%d/%d err=%v", total, filtered, err)
	}
}
