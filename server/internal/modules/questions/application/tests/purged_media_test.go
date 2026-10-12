//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

func TestAQuestionWriteThatRacesTheMediaSweepAnswersAsAMissingFile(t *testing.T) {
	w := newBankWorld(t)
	ctx := context.Background()
	asset := w.image(t, w.a)
	if _, err := w.tx.Exec(ctx, `UPDATE app.media_assets SET deleted_at=now(),unreferenced_at=now(),purged_at=now() WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	racing := application.New(repositories.NewPostgres(db.NewContext(w.tx)), fixedMediaKind("image"))
	existing := w.create(t, w.a, "Chưa có tệp", nil)

	_, err := racing.Commands.Create.Handle(ctx, command.Create{Request: request(w.a, false, "", w.input("Tạo với tệp đã xoá", &asset))})
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Errorf("creating a question that names a file the sweep removed: %v, want the missing-file refusal", err)
	}
	_, err = racing.Commands.Update.Handle(ctx, command.Update{Request: request(w.a, false, existing.ID, w.input("Sửa với tệp đã xoá", &asset))})
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Errorf("updating a question to a file the sweep removed: %v, want the missing-file refusal", err)
	}

	var named int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE media_asset_id=$1`, asset).Scan(&named); err != nil || named != 0 {
		t.Errorf("%d questions name the removed file (%v)", named, err)
	}
	if kept, err := w.get(access.Scope{UserID: w.admin, All: true}, existing.ID); err != nil || kept.Prompt != "Chưa có tệp" {
		t.Errorf("the refused update changed the question: %+v (%v)", kept, err)
	}
	w.create(t, w.a, "Vẫn ghi được sau đó", nil)
}
