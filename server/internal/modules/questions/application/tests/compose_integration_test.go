//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/content"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"
)

func composeAuthor(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	email := "compose-" + hex.EncodeToString(nonce) + "@example.com"
	t.Cleanup(func() {
		ctx := context.Background()
		for _, statement := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id IN (SELECT id FROM app.users WHERE email = $1)`,
			`DELETE FROM app.questions WHERE created_by IN (SELECT id FROM app.users WHERE email = $1)`,
			`DELETE FROM app.users WHERE email = $1`,
		} {
			if _, err := pool.Exec(ctx, statement, email); err != nil {
				t.Errorf("cleanup %q: %v", statement, err)
			}
		}
	})
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id) VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`,
		email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func decomposedProse(t *testing.T, text string) (json.RawMessage, string) {
	t.Helper()
	raw := fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]}`, norm.NFD.String(text))
	document, err := content.ParseQuestion([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(raw), document.PlainText()
}

func TestAnOldDecomposedRowAcceptsTheComposedTextOfAClientThatSendsNoDocument(t *testing.T) {
	pool := newPool(t)
	author := composeAuthor(t, pool)
	ctx := context.Background()
	repo := repositories.NewPostgres(db.NewContext(pool))

	promptRaw, promptText := decomposedProse(t, "Đọc kỹ đoạn văn")
	explanationRaw, explanationText := decomposedProse(t, "Vì sao như vậy")
	stored, err := repo.Create(ctx, domain.WriteInput{ActorID: author, Now: time.Now(), Input: domain.Input{
		Type: domain.ShortAnswer, Points: "1", Tags: []string{},
		Prompt: promptText, PromptContent: promptRaw, Explanation: &explanationText, ExplanationContent: explanationRaw,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if stored.Prompt == norm.NFC.String(stored.Prompt) {
		t.Fatal("the fixture row was stored composed")
	}

	svc := newService(t, pool)
	composedPrompt, composedExplanation := norm.NFC.String(promptText), norm.NFC.String(explanationText)
	updated, err := svc.Commands.Update.Handle(ctx, command.Update{Request: domain.WriteRequest{ID: stored.ID, ActorID: author, Input: domain.Input{
		Type: domain.ShortAnswer, Points: "2", Tags: []string{}, Prompt: composedPrompt, Explanation: &composedExplanation,
	}}})
	if err != nil {
		t.Fatalf("a legacy client sending the composed text was refused: %v", err)
	}
	if updated.Prompt != composedPrompt || *updated.Explanation != composedExplanation {
		t.Fatalf("stored text is not the composed text: %q %q", updated.Prompt, *updated.Explanation)
	}
	for field, raw := range map[string]json.RawMessage{"promptContent": updated.PromptContent, "explanationContent": updated.ExplanationContent} {
		if len(raw) == 0 {
			t.Fatalf("%s was dropped", field)
		}
		if string(raw) != norm.NFC.String(string(raw)) {
			t.Fatalf("%s is still decomposed: %s", field, raw)
		}
	}
	document, err := content.ParseQuestion(updated.PromptContent)
	if err != nil || document.PlainText() != updated.Prompt {
		t.Fatalf("the stored pair stopped matching: %v", err)
	}
}

func TestAnOldDecomposedRowStillRefusesDifferentTextFromAClientThatSendsNoDocument(t *testing.T) {
	pool := newPool(t)
	author := composeAuthor(t, pool)
	ctx := context.Background()
	repo := repositories.NewPostgres(db.NewContext(pool))
	promptRaw, promptText := decomposedProse(t, "Đọc kỹ đoạn văn")
	stored, err := repo.Create(ctx, domain.WriteInput{ActorID: author, Now: time.Now(), Input: domain.Input{
		Type: domain.ShortAnswer, Points: "1", Tags: []string{}, Prompt: promptText, PromptContent: promptRaw,
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = newService(t, pool).Commands.Update.Handle(ctx, command.Update{Request: domain.WriteRequest{ID: stored.ID, ActorID: author, Input: domain.Input{
		Type: domain.ShortAnswer, Points: "1", Tags: []string{}, Prompt: "Một đề khác",
	}}})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "promptContent" {
		t.Fatalf("err=%v", err)
	}
	again, err := repo.Get(ctx, everyone, stored.ID)
	if err != nil || again.Prompt != promptText {
		t.Fatalf("a refused write changed the row: %v", err)
	}
}
