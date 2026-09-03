package students_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/students"
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

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func makeStudent(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,$2,'student')
		 RETURNING id::text`, "st-"+nonce(t)+"@example.com", name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, id)
	})
	return id
}

func ids(found []students.Student) map[string]bool {
	out := map[string]bool{}
	for _, s := range found {
		out[s.ID] = true
	}
	return out
}

// The whole reason search folds accents: a teacher typing without a Vietnamese
// keyboard must still find Hân.
func TestSearchIgnoresAccentsAndCase(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	han := makeStudent(t, pool, "Phạm Gia Hân")
	makeStudent(t, pool, "Trần Bảo Long")

	for _, query := range []string{"hân", "han", "HAN", "Gia Hân"} {
		t.Run(query, func(t *testing.T) {
			found, _, err := store.List(context.Background(), students.ListInput{Query: query})
			if err != nil {
				t.Fatal(err)
			}
			if !ids(found)[han] {
				t.Errorf("%q did not find Phạm Gia Hân", query)
			}
		})
	}
}

func TestSearchDoesNotMatchEverybody(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	makeStudent(t, pool, "Phạm Gia Hân")
	long := makeStudent(t, pool, "Trần Bảo Long")

	found, _, err := store.List(context.Background(), students.ListInput{Query: "Hân"})
	if err != nil {
		t.Fatal(err)
	}
	if ids(found)[long] {
		t.Error("searching for Hân returned Long")
	}
}

// A '%' typed into the box is a character, not a wildcard.
func TestWildcardsInTheQueryAreLiteral(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	makeStudent(t, pool, "Phạm Gia Hân")

	found, _, err := store.List(context.Background(), students.ListInput{Query: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("a bare %% matched %d students", len(found))
	}
}

func TestAnAdminIsNeverAStudent(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	ctx := context.Background()

	var admin string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên Kiểm','admin')
		 RETURNING id::text`, "adm-"+nonce(t)+"@example.com").Scan(&admin); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, admin)
	})

	found, _, err := store.List(ctx, students.ListInput{Query: "Kiểm"})
	if err != nil {
		t.Fatal(err)
	}
	if ids(found)[admin] {
		t.Error("the students listing returned an admin")
	}
}

func TestTheClassFilterNarrowsToThatRoster(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	ctx := context.Background()

	inside := makeStudent(t, pool, "Trong Lớp")
	outside := makeStudent(t, pool, "Ngoài Lớp")

	var classID, teacher string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'GV','admin') RETURNING id::text`,
		"cls-"+nonce(t)+"@example.com").Scan(&teacher); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`,
		"Lớp "+nonce(t)).Scan(&classID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid,$2::uuid,'admin',$3::uuid)`, classID, inside, teacher); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1::uuid`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1::uuid`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1::uuid`, teacher)
	})

	found, _, err := store.List(ctx, students.ListInput{ClassID: classID})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(found)
	if !got[inside] {
		t.Error("the class filter dropped a member")
	}
	if got[outside] {
		t.Error("the class filter returned a non-member")
	}
}

func TestThePageStopsAtTheLimitAndTheNextPageFollowsWithoutOverlap(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	ctx := context.Background()

	tag := nonce(t)
	for range 3 {
		makeStudent(t, pool, "Phân Trang "+tag)
	}

	first, page, err := store.List(ctx, students.ListInput{Query: tag, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || page.Total != 3 || page.Number != 1 || page.Size != 2 {
		t.Fatalf("first page = %d rows, %+v; want 2 of 3", len(first), page)
	}

	second, page, err := store.List(ctx, students.ListInput{Query: tag, Limit: 2, Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || page.Total != 3 {
		t.Fatalf("second page = %d rows, %+v; want the third", len(second), page)
	}
	// OFFSET over a stable order (id DESC): the pages must not overlap.
	for _, s := range second {
		if ids(first)[s.ID] {
			t.Errorf("%s appeared on both pages", s.FullName)
		}
	}
}
