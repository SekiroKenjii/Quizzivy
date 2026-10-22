//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func proseDoc(text string, media ...string) json.RawMessage {
	blocks := []string{fmt.Sprintf(`{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}`, text)}
	return json.RawMessage(`{"format":"semantic_v1","blocks":[` + strings.Join(append(blocks, media...), ",") + `]}`)
}

func proseImage(asset, alt string) string {
	return fmt.Sprintf(`{"type":"image","assetId":%q,"alt":%q}`, asset, alt)
}

// referenceDigest is every reference row a set of questions holds, by the
// question's prompt, so copies with new ids compare equal.
func referenceDigest(t *testing.T, tx pgx.Tx, table, owner, where string, arg string) string {
	t.Helper()
	var digest string
	err := tx.QueryRow(context.Background(), `SELECT coalesce(string_agg(q.prompt || '|' || r.field || '|' || r.media_asset_kind::text || '|' || r.media_asset_id::text, ','
		ORDER BY q.prompt || '|' || r.field || '|' || r.media_asset_kind::text || '|' || r.media_asset_id::text), '')
		FROM app.`+table+` r JOIN `+where+` ON q.id = r.`+owner, arg).Scan(&digest)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func frozenReferences(t *testing.T, tx pgx.Tx, versionID string) string {
	return referenceDigest(t, tx, "test_version_question_media_references", "test_version_question_id",
		`(SELECT q.* FROM app.test_version_questions q JOIN app.test_version_sections s ON s.id = q.test_version_section_id WHERE s.test_version_id = $1) q`, versionID)
}

func draftReferences(t *testing.T, tx pgx.Tx, testID string) string {
	return referenceDigest(t, tx, "question_media_references", "question_id",
		`(SELECT q.* FROM app.questions q WHERE q.id IN (
			SELECT sq.question_id FROM app.test_section_questions sq JOIN app.test_sections s ON s.id = sq.test_section_id WHERE s.test_id = $1
			UNION SELECT u.question_id FROM app.test_section_units u JOIN app.test_sections s ON s.id = u.test_section_id WHERE s.test_id = $1 AND u.question_id IS NOT NULL
			UNION SELECT m.id FROM app.questions m JOIN app.question_groups g ON g.id = m.context_group_id JOIN app.test_sections s ON s.id = g.owner_section_id WHERE s.test_id = $1)) q`, testID)
}

// TestPublishFreezesProseMediaAndEveryCopyCarriesIt is T-R6.5b's freeze: a
// version freezes the references beside the content, locking each asset as it
// locks question media, and editing the bank afterwards leaves them; "Edit
// from this version" and test duplication copy them back into drafts.
func TestPublishFreezesProseMediaAndEveryCopyCarriesIt(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	scope := access.Scope{UserID: author}
	testID, section, updated := snapshotDraft(t, tx, author)
	chart, figure, voice := storedGroupAsset(t, tx, author, "image"), storedGroupAsset(t, tx, author, "image"), storedGroupAsset(t, tx, author, "audio")

	var standalone string
	if err := tx.QueryRow(ctx, `SELECT question_id::text FROM app.test_section_questions WHERE test_section_id = $1`, section).Scan(&standalone); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.questions SET prompt = 'Standalone' || chr(10) || chr(10) || 'A chart', prompt_content = $2 WHERE id = $1`,
		standalone, proseDoc("Standalone", proseImage(chart, "A chart"))); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind) VALUES ($1, 'prompt', $2, 'image')`, standalone, chart); err != nil {
		t.Fatal(err)
	}

	bundle := storedGroupFixture(t, "")
	why := "Vì"
	member := &bundle.Questions[0].Input
	member.Prompt, member.PromptContent = "Chọn từ", proseDoc("Chọn từ", proseImage(figure, ""))
	member.Explanation, member.ExplanationContent = &why, proseDoc(why, fmt.Sprintf(`{"type":"audio","assetId":%q}`, voice))
	for i := range bundle.Questions {
		bundle.Questions[i].Input.Tags = []string{}
	}
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: scope, Grants: bothKeys}); err != nil {
		t.Fatalf("create the group: %v", err)
	}
	draft := draftReferences(t, tx, testID)
	for _, want := range []string{"Standalone\n\nA chart|prompt|image|" + chart, "Chọn từ|prompt|image|" + figure, "Chọn từ|explanation|audio|" + voice} {
		if !strings.Contains(draft, want) {
			t.Fatalf("the draft holds %q, which lacks %q", draft, want)
		}
	}

	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	frozen := frozenReferences(t, tx, published.ID)
	if frozen != draft {
		t.Fatalf("the version froze %q, want the draft's %q", frozen, draft)
	}

	copied, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: author, Now: time.Now(), Scope: scope})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if got := draftReferences(t, tx, copied.ID); got != draft {
		t.Fatalf("the duplicate holds %q, want %q", got, draft)
	}

	if _, err := tx.Exec(ctx, `UPDATE app.questions SET prompt = 'Standalone', prompt_content = $2 WHERE id = $1`, standalone, proseDoc("Standalone")); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.question_media_references WHERE question_id = $1`, standalone); err != nil {
		t.Fatal(err)
	}
	if got := frozenReferences(t, tx, published.ID); got != frozen {
		t.Fatalf("editing the bank changed the version's references to %q", got)
	}

	req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author, Scope: scope}, Version: published.Version}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&req.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDraftFromVersion(ctx, req, time.Now()); err != nil {
		t.Fatalf("edit from this version: %v", err)
	}
	if got := draftReferences(t, tx, testID); got != frozen {
		t.Fatalf("the draft from the version holds %q, want %q", got, frozen)
	}
	republished, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatalf("publish the restored draft: %v", err)
	}
	if got := frozenReferences(t, tx, republished.ID); got != frozen {
		t.Fatalf("the restored draft froze %q, want %q", got, frozen)
	}
}

// TestPublishNamesTheQuestionWhoseProseShowsAGoneAsset is the publish check
// for prose media: an asset deleted since the question was written refuses
// the version on that question, as a gone attachment does.
func TestPublishNamesTheQuestionWhoseProseShowsAGoneAsset(t *testing.T) {
	ctx := context.Background()
	tx, author, _ := groupTransaction(t)
	scope := access.Scope{UserID: author}
	testID, section, _ := snapshotDraft(t, tx, author)
	chart := storedGroupAsset(t, tx, author, "image")
	var standalone string
	if err := tx.QueryRow(ctx, `UPDATE app.questions SET prompt = 'Standalone' || chr(10) || chr(10) || 'A chart', prompt_content = $2
		WHERE id = (SELECT question_id FROM app.test_section_questions WHERE test_section_id = $1) RETURNING id::text`,
		section, proseDoc("Standalone", proseImage(chart, "A chart"))).Scan(&standalone); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, chart); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	_, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	var invalid *domain.PublishValidationError
	if !errors.As(err, &invalid) || len(invalid.Violations) != 1 || invalid.Violations[0].QuestionID != standalone || invalid.Violations[0].Rule != domain.QuestionValid {
		t.Fatalf("publishing a prompt whose image was deleted answered %v", err)
	}
}
