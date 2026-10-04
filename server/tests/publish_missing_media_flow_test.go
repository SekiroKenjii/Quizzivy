//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
)

func TestPublishingADraftWhoseQuestionNamesADeletedAssetAnswers409NamingTheQuestion(t *testing.T) {
	ctx := context.Background()
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{"type": "true_false", "prompt": "Nghe và chọn", "points": 1,
		"options": []any{map[string]any{"text": "Đúng", "isCorrect": true}, map[string]any{"text": "Sai", "isCorrect": false}}})
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề dùng tệp đã xoá " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Phần 1", "questionIds": []string{id(question)}}}})

	var owner string
	if err := w.pool.QueryRow(ctx, `SELECT id::text FROM app.users WHERE email = $1`, email).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	var asset string
	if err := w.pool.QueryRow(ctx,
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes, duration_ms,
		         original_filename, checksum_sha256, uploaded_by)
		 VALUES ('audio', $1, 'audio/mpeg', 1024, 10000, 'nghe.mp3',
		         repeat('c', 32)::bytea, $2)
		 RETURNING id::text`, "audio/e2e-"+nonce(t)+".mp3", owner).Scan(&asset); err != nil {
		t.Fatal(err)
	}
	attached, err := w.pool.Exec(ctx,
		`UPDATE app.questions
		    SET media_asset_id = $1, media_asset_kind = 'audio',
		        audio_allow_seek = false, audio_show_transcript_after = true
		  WHERE id = $2`, asset, id(question))
	if err != nil {
		t.Fatal(err)
	}
	if attached.RowsAffected() != 1 {
		t.Fatalf("attaching the asset touched %d rows, want 1", attached.RowsAffected())
	}
	deleted, err := w.pool.Exec(ctx, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, asset)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.RowsAffected() != 1 {
		t.Fatalf("soft-deleting the asset touched %d rows, want 1", deleted.RowsAffected())
	}

	blocked := teacher.must(http.StatusConflict, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)
	if code := blocked["error"].(map[string]any)["code"]; code != "PUBLISH_VALIDATION_FAILED" {
		t.Fatalf("the refusal carries code %v, want PUBLISH_VALIDATION_FAILED", code)
	}
	violations := blocked["violations"].([]any)
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1: %v", len(violations), violations)
	}
	violation := violations[0].(map[string]any)
	if violation["questionId"] != id(question) || violation["rule"] != "audio_question_has_asset" {
		t.Fatalf("the refusal did not name the question that uses the deleted file: %v", violation)
	}
	var count int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.test_versions WHERE test_id = $1`, id(test)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("the refused publication left a snapshot: %d, %v", count, err)
	}

	restored, err := w.pool.Exec(ctx, `UPDATE app.media_assets SET deleted_at = NULL WHERE id = $1`, asset)
	if err != nil {
		t.Fatal(err)
	}
	if restored.RowsAffected() != 1 {
		t.Fatalf("restoring the asset touched %d rows, want 1", restored.RowsAffected())
	}
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)
}
