//go:build integration

package repositories_test

import (
	"context"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
	"testing"
)

func TestIntroUsesQuestionAndSharedAudioOnTheAssignedVersion(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	group, questions := frozenAssignmentGroup(t, tx, w.versionID, false)
	repo := repositories.NewPostgres(db.NewContext(tx))
	a := createFor(t, repo, w, legalInput(w))
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	asset := func() string {
		t.Helper()
		var id string
		must(tx.QueryRow(ctx, `INSERT INTO app.media_assets
		 (kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by)
		 VALUES ('audio',$1,'audio/mpeg',100,1000,'synthetic.mp3',sha256('audio'::bytea),$2) RETURNING id::text`, nonce(t), w.admin).Scan(&id))
		return id
	}
	firstAsset := asset()
	_, err = tx.Exec(ctx, `INSERT INTO app.test_version_group_recordings(group_id,media_asset_id,max_plays,show_transcript_after_submit)
	 VALUES($1,$2,NULL,true)`, group, firstAsset)
	must(err)
	intro, err := repo.StudentDetail(ctx, a.ID, w.student)
	must(err)
	if !intro.HasAudio || !intro.HasSharedAudio || !intro.ShowsTranscript || intro.AudioMaxPlays != nil {
		t.Fatalf("unlimited shared recording missing: %+v", intro)
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.test_version_group_recordings(group_id,media_asset_id,max_plays)
	 VALUES($1,$2,2)`, group, asset())
	must(err)
	_, err = tx.Exec(ctx, `UPDATE app.test_version_questions SET media_asset_id=$2,media_asset_kind='audio',
	 audio_max_plays=3,audio_allow_seek=false,audio_show_transcript_after=false WHERE id=$1`, questions[0], firstAsset)
	must(err)
	var newer string
	must(tx.QueryRow(ctx, `INSERT INTO app.test_versions(test_id,version,total_points,published_by)
	 VALUES($1,2,3,$2) RETURNING id::text`, w.testID, w.admin).Scan(&newer))
	newGroup, _ := frozenAssignmentGroup(t, tx, newer, false)
	_, err = tx.Exec(ctx, `INSERT INTO app.test_version_group_recordings(group_id,media_asset_id,max_plays)
	 VALUES($1,$2,1)`, newGroup, firstAsset)
	must(err)
	_, err = tx.Exec(ctx, `UPDATE app.tests SET current_version=2 WHERE id=$1`, w.testID)
	must(err)
	intro, err = repo.StudentDetail(ctx, a.ID, w.student)
	must(err)
	if !intro.HasAudio || !intro.HasSharedAudio || !intro.ShowsTranscript || intro.AudioMaxPlays == nil || *intro.AudioMaxPlays != 2 {
		t.Fatalf("intro used question-only/default-version cap: %+v", intro)
	}
}
