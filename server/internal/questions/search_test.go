package questions_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// makeAuthor registers cleanup BEFORE inserting anything, so a failure midway
// still removes what it created.
func makeAuthor(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	email := "author-" + hex.EncodeToString(nonce) + "@example.com"

	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.questions WHERE created_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, id)
	})
	return id
}

func newService(t *testing.T, pool *pgxpool.Pool) *questions.Service {
	t.Helper()
	return questions.NewService(questions.NewStore(pool))
}

// write creates a short_answer question with the given prompt and tags.
func write(t *testing.T, svc *questions.Service, author, prompt string, tags ...string) questions.Question {
	t.Helper()
	if tags == nil {
		tags = []string{}
	}
	q, err := svc.Create(context.Background(), questions.WriteRequest{
		Input: questions.Input{
			Type: questions.ShortAnswer, Prompt: prompt, Points: "1.00", Tags: tags,
		},
		ActorID: author,
	})
	if err != nil {
		t.Fatalf("create %q: %v", prompt, err)
	}
	return q
}

// found reports whether a search returned the given question id.
func found(t *testing.T, svc *questions.Service, query, id string) bool {
	t.Helper()
	results, _, err := svc.List(context.Background(),
		questions.ListInput{Query: query, Limit: questions.MaxLimit})
	if err != nil {
		t.Fatalf("search %q: %v", query, err)
	}
	for _, q := range results {
		if q.ID == id {
			return true
		}
	}
	return false
}

// TestSearchIsAccentInsensitive is D-11's reason for existing.
//
// Without the unaccent fold every one of these fails: `to_tsvector('simple')`
// does no diacritic folding, and pg_trgm is case-insensitive but NOT
// accent-insensitive. In a Vietnamese-first product that means a teacher who
// types without diacritics -- which is most typing -- finds nothing.
func TestSearchIsAccentInsensitive(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	nghe := write(t, svc, author, "Con nghé đang gặm cỏ ngoài đồng")
	phatAm := write(t, svc, author, "Luyện phát âm phụ âm cuối")
	duong := write(t, svc, author, "Đường tới trường dài bao nhiêu")

	cases := []struct {
		query string
		want  string
		id    string
	}{
		{"nghe", "nghé", nghe.ID},
		{"phat am", "phát âm", phatAm.ID},
		{"duong", "Đường", duong.ID},
		// The other direction: typing the diacritics must still work.
		{"nghé", "nghé", nghe.ID},
		{"phát âm", "phát âm", phatAm.ID},
		{"Đường", "Đường", duong.ID},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			if !found(t, svc, tc.query, tc.id) {
				t.Errorf("searching %q did not find the prompt containing %q", tc.query, tc.want)
			}
		})
	}
}

func TestSearchStillDiscriminates(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	nghe := write(t, svc, author, "Con nghé đang gặm cỏ")
	if found(t, svc, "hoàn toàn không liên quan", nghe.ID) {
		t.Error("search matched a prompt with nothing in common")
	}
}

// TestSearchTreatsWildcardsAsText: `%` and `_` are LIKE wildcards, and the
// search box is not a pattern language. Unescaped, a search for `%` returns the
// entire bank.
func TestSearchTreatsWildcardsAsText(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	plain := write(t, svc, author, "Một câu hỏi bình thường")
	literal := write(t, svc, author, "Giảm giá 50% cho học sinh")

	if found(t, svc, "%", plain.ID) {
		t.Error("searching for %% matched every question; the wildcard is not escaped")
	}
	if !found(t, svc, "50%", literal.ID) {
		t.Error("searching for a literal 50%% did not find it")
	}
	if found(t, svc, "_", plain.ID) {
		t.Error("searching for _ matched everything; the single-character wildcard is not escaped")
	}
}

// TestTrigramIndexIsActuallyUsed is the guard T-2.6 asks for: a later refactor
// of the query expression must not silently fall back to a sequential scan.
//
// Postgres matches an expression index only when the query spells the
// expression IDENTICALLY, and a mismatch is invisible -- the rows come back
// correct, just slowly.
//
// HOW THIS AVOIDS BEING A FLAKY TEST, which took some finding. Asserting "the
// planner chose the trigram index" against realistic data is a coin flip: at
// 3000 rows it picked the index in 2 of 5 runs, because ANALYZE's sampling moves
// the cost estimate back and forth across the crossover. Disabling seqscan alone
// does not help either -- app.questions carries four other partial indexes on
// `deleted_at IS NULL`, so the planner just uses one of those and applies the
// trigram condition as a Filter, and which one it picks also wobbles.
//
// So the alternatives are removed instead of out-competed. Inside a transaction
// that is always rolled back, the competing indexes are dropped and seq scans
// disabled, leaving the trigram index as the only path. The planner then uses it
// if and only if the expression matches -- and falls back to the disabled seq
// scan if it does not, which is the failure being detected. 10 consecutive runs
// agreed, on an empty table and a populated one.
func TestTrigramIndexIsActuallyUsed(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DROP INDEX app.questions_tags_idx, app.questions_type_id_idx,
	                                      app.questions_prompt_fts_idx, app.questions_media_idx`); err != nil {
		if strings.Contains(err.Error(), "lock timeout") || strings.Contains(err.Error(), "canceling statement") {
			t.Skipf("could not take the table lock within 5s, so the plan could not be isolated: %v", err)
		}
		t.Fatalf("dropping the competing indexes: %v", err)
	}
	sql := `EXPLAIN SELECT q.id FROM app.questions q
	         WHERE q.deleted_at IS NULL
	           AND ` + questions.TrigramExpression +
		` LIKE '%' || app.immutable_unaccent(lower($1)) || '%' ESCAPE '\'`

	rows, err := tx.Query(ctx, sql, "nghe")
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	text := plan.String()
	indexed := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Index Cond:") && strings.Contains(line, "~~") {
			indexed = true
		}
	}
	if !indexed || !strings.Contains(text, "questions_prompt_trgm_idx") {
		t.Errorf("the search condition is not being answered by the trigram index -- it appears "+
			"as a Filter rather than an Index Cond, so every row is rechecked.\n"+
			"The index is on app.immutable_unaccent(lower(prompt)) and the query must spell it "+
			"IDENTICALLY; a mismatch still returns correct rows, just slowly.\nplan:\n%s", text)
	}
}

// TestTheSearchQueryUsesTheSharedExpression closes the gap the EXPLAIN test
// cannot: that the STORE uses the constant the EXPLAIN test verifies. Without
// this, someone could rewrite search.go's condition and leave TrigramExpression
// behind as an unused constant that both other tests keep happily checking.
func TestTheSearchQueryUsesTheSharedExpression(t *testing.T) {
	source, err := os.ReadFile("search.go")
	if err != nil {
		t.Fatalf("reading search.go: %v", err)
	}
	if !strings.Contains(string(source), "` + TrigramExpression + `") {
		t.Error("the search condition no longer interpolates TrigramExpression, so the " +
			"EXPLAIN test is verifying an expression the store does not use")
	}
}

// TestSearchQueryUsesTheIndexedExpressionVerbatim keeps the constant honest:
// TrigramExpression is what both the store and the EXPLAIN test use, so it has
// to be the text the migration indexed, with the opclass that serves LIKE.
func TestSearchQueryUsesTheIndexedExpressionVerbatim(t *testing.T) {
	pool := newPool(t)
	var indexdef string
	err := pool.QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes
		  WHERE schemaname = 'app' AND indexname = 'questions_prompt_trgm_idx'`).Scan(&indexdef)
	if err != nil {
		t.Fatalf("reading the index definition: %v", err)
	}

	// The index is declared on `prompt`; the query aliases the table as `q`.
	want := strings.ReplaceAll(questions.TrigramExpression, "q.prompt", "prompt")
	normalised := strings.ReplaceAll(indexdef, " ", "")
	if !strings.Contains(normalised, strings.ReplaceAll(want, " ", "")) {
		t.Errorf("TrigramExpression is %q, which does not appear in the index definition:\n%s",
			questions.TrigramExpression, indexdef)
	}
	if !strings.Contains(indexdef, "gin_trgm_ops") {
		t.Errorf("questions_prompt_trgm_idx is not a gin_trgm_ops index, so it cannot serve "+
			"a LIKE:\n%s", indexdef)
	}
}

// A-06's rail is checkboxes and chips, so the two groups behave differently
// from each other: within a group the values widen, across groups they narrow.
func TestFiltersWidenWithinAGroupAndNarrowAcross(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	store := questions.NewStore(pool)
	ctx := context.Background()
	tag := "grp-" + strings.ReplaceAll(author, "-", "")[:12]

	essay := write(t, svc, author, "Câu tự luận", tag, tag+"-b")

	// A second type, so the type group has something to widen into.
	single, err := svc.Create(ctx, questions.WriteRequest{
		Input: questions.Input{
			Type: questions.SingleChoice, Prompt: "Câu một đáp án", Points: "1.00",
			Tags: []string{tag},
			Options: []questions.OptionInput{
				{Text: "A", IsCorrect: true}, {Text: "B"},
			},
		},
		ActorID: author,
	})
	if err != nil {
		t.Fatalf("create single_choice: %v", err)
	}

	ids := func(in questions.ListInput) map[string]bool {
		found, _, err := store.List(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, q := range found {
			out[q.ID] = true
		}
		return out
	}

	one := ids(questions.ListInput{
		Tags:  []string{tag},
		Types: []questions.Type{questions.SingleChoice},
	})
	if !one[single.ID] || one[essay.ID] {
		t.Errorf("one ticked type should match only that type: %v", one)
	}

	// Ticking the second type must ADD rows, never remove any.
	both := ids(questions.ListInput{
		Tags:  []string{tag},
		Types: []questions.Type{questions.SingleChoice, questions.ShortAnswer},
	})
	if !both[single.ID] || !both[essay.ID] {
		t.Errorf("two ticked types should match both: %v", both)
	}

	// A second chip widens too -- overlap, not containment. Under `@>` the
	// single-choice question, which lacks the second tag, would drop out.
	widened := ids(questions.ListInput{Tags: []string{tag, tag + "-b"}})
	if !widened[single.ID] || !widened[essay.ID] {
		t.Errorf("a second tag chip removed rows instead of adding them: %v", widened)
	}
}

// A-06's bulk "Gắn thẻ".
func TestAddingTagsInBulkIsAdditiveAndIdempotent(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	store := questions.NewStore(pool)
	ctx := context.Background()
	tag := "bulk-" + strings.ReplaceAll(author, "-", "")[:10]

	keeps := write(t, svc, author, "Giữ thẻ cũ", tag+"-old")
	bare := write(t, svc, author, "Chưa có thẻ")

	updated, err := store.AddTags(ctx, []string{keeps.ID, bare.ID}, []string{tag + "-new"})
	if err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if updated != 2 {
		t.Errorf("updated = %d, want 2", updated)
	}

	got, err := svc.Get(ctx, keeps.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Union, not replacement: a concurrent tagging of an overlapping selection
	// would otherwise have the later write erase the earlier one's tags.
	if !slices.Contains(got.Tags, tag+"-old") {
		t.Errorf("the existing tag was dropped: %v", got.Tags)
	}
	if !slices.Contains(got.Tags, tag+"-new") {
		t.Errorf("the new tag was not added: %v", got.Tags)
	}

	// A retry after a dropped connection must not look like it did work.
	again, err := store.AddTags(ctx, []string{keeps.ID, bare.ID}, []string{tag + "-new"})
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Errorf("re-applying the same tags reported %d updated, want 0", again)
	}

	after, err := svc.Get(ctx, keeps.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Tags) != len(got.Tags) {
		t.Errorf("a repeat run duplicated tags: %v", after.Tags)
	}
}

// A soft-deleted question is not in the bank, so a selection that happens to
// name one must not resurrect it into somebody's tag set.
func TestBulkTaggingSkipsDeletedQuestions(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	store := questions.NewStore(pool)
	ctx := context.Background()

	q := write(t, svc, author, "Sắp bị xoá")
	if _, err := pool.Exec(ctx,
		`UPDATE app.questions SET deleted_at = now() WHERE id = $1::uuid`, q.ID); err != nil {
		t.Fatal(err)
	}

	updated, err := store.AddTags(ctx, []string{q.ID}, []string{"khong-nen-co"})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 0 {
		t.Errorf("updated = %d, want 0 — a deleted question was tagged", updated)
	}
}
