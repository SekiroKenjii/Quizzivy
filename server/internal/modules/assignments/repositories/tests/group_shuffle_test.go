//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	attemptrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"testing"

	"github.com/jackc/pgx/v5"
)

func frozenAssignmentGroup(t *testing.T, tx pgx.Tx, versionID string, fixed bool) (string, []string) {
	t.Helper()
	ctx := context.Background()
	var section, group string
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_sections(test_version_id,ordinal,title) VALUES($1,0,'Part') RETURNING id::text`, versionID).Scan(&section); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_groups(test_version_section_id,title) VALUES($1,'Shared context') RETURNING id::text`, section).Scan(&group); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := range 3 {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_questions(test_version_section_id,ordinal,type,prompt,points)
            VALUES($1,$2,'single_choice','Choose',1) RETURNING id::text`, section, i).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_options(test_version_question_id,ordinal,text,is_correct)
            VALUES($1,0,'First',true),($1,1,'Second',false)`, id); err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			policy := "shuffle"
			if i == 1 && fixed {
				policy = "fixed"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_group_members(group_id,test_version_section_id,question_id,ordinal,option_order)
                VALUES($1,$2,$3,$4,$5)`, group, section, id, i-1, policy); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_units(test_version_section_id,ordinal,question_id,group_id)
        VALUES($1,0,$2,NULL),($1,1,NULL,$3)`, section, ids[0], group); err != nil {
		t.Fatal(err)
	}

	return group, ids
}

func TestAssignmentRejectsFixedGroupOptionShuffleWithoutPartialWrites(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	group, ids := frozenAssignmentGroup(t, tx, w.versionID, true)
	attempts := attemptrepo.NewPostgres(db.NewContext(tx))
	loaded, err := attempts.Questions(ctx, w.versionID)
	if err != nil || len(loaded) != 3 {
		t.Fatalf("paper load: %+v, %v", loaded, err)
	}
	if loaded[0].GroupID != "" || loaded[0].FixedOptionOrder || loaded[1].GroupID != group || loaded[1].GroupOrdinal != 0 || !loaded[1].FixedOptionOrder || loaded[2].GroupOrdinal != 1 || loaded[2].FixedOptionOrder || loaded[2].ID != ids[2] {
		t.Fatal("frozen membership/order policy lost in learner reader")
	}
	repo := repositories.NewPostgres(db.NewContext(tx))
	in := legalInput(w)
	in.ShuffleO = true
	for _, draft := range []bool{false, true} {
		in.Draft = draft
		_, err := repo.Create(ctx, request(w), in)
		assertFixedShuffleError(t, err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.assignments WHERE test_id=$1`, w.testID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid assignment was written: %d, %v", count, err)
	}
	in.ShuffleO = false
	in.Draft = false
	created, err := repo.Create(ctx, request(w), in)
	if err != nil {
		t.Fatal(err)
	}
	req := request(w)
	req.ID = created.ID
	in.ShuffleO = true
	_, err = repo.Update(ctx, req, in)
	assertFixedShuffleError(t, err)
	var shuffled bool
	if err := tx.QueryRow(ctx, `SELECT shuffle_options FROM app.assignments WHERE id=$1`, created.ID).Scan(&shuffled); err != nil || shuffled {
		t.Fatalf("invalid update persisted: %v, %v", shuffled, err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1 AND action LIKE 'assignment.%'`, w.admin).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed validation wrote an audit: %d, %v", count, err)
	}
	var nextVersion string
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_versions(test_id,version,total_points,published_by)
        VALUES($1,2,3,$2) RETURNING id::text`, w.testID, w.admin).Scan(&nextVersion); err != nil {
		t.Fatal(err)
	}
	frozenAssignmentGroup(t, tx, nextVersion, false)
	in.TestVersionID = nextVersion
	if _, err := repo.Create(ctx, request(w), in); err != nil {
		t.Fatalf("fixed member in another version blocked a valid assignment: %v", err)
	}

}

func assertFixedShuffleError(t *testing.T, err error) {
	t.Helper()
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "shuffleOptions" {
		t.Fatalf("wanted actionable shuffleOptions validation: %v", err)
	}
}
