package repositories

import (
	"context"
	"errors"
	"fmt"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// snapshot freezes the draft into the version tables.
func snapshot(ctx context.Context, tx pgx.Tx, versionID string, d domain.DraftContent, media MediaLocks) error {
	missing, err := lockMediaAssets(ctx, tx, d, media)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		if refusal := domain.Publishing.MissingMedia(d, missing); refusal != nil {
			return refusal
		}
		return goneAsset(missing[0])
	}

	for _, section := range d.Sections {
		sectionID, err := freezeSection(ctx, tx, versionID, section)
		if err != nil {
			return err
		}
		if err := freezeSectionGraph(ctx, tx, sectionID, section); err != nil {
			return err
		}
	}
	return nil
}

func freezeSection(ctx context.Context, tx pgx.Tx, versionID string, section domain.DraftSection) (string, error) {
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title, instructions)
		 VALUES ($1, $2, $3, $4) RETURNING id::text`,
		versionID, section.Ordinal, section.Title, section.Instructions).Scan(&id); err != nil {
		return "", fmt.Errorf("publish: freeze section: %w", err)
	}
	return id, nil
}

// lockMediaAssets takes every asset lock the snapshot needs, up front and in
// sorted order.
func lockMediaAssets(ctx context.Context, tx pgx.Tx, d domain.DraftContent, media MediaLocks) (missing []string, err error) {
	ids, err := snapshotMediaIDs(d)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 && media == nil {
		return nil, fmt.Errorf("publish: media reference locks unavailable")
	}
	for _, id := range ids {
		err := media.LockForVersionUse(ctx, tx, id)
		if errors.Is(err, mediadomain.ErrNotFound) {
			missing = append(missing, id)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("publish: media asset %s: %w", id, err)
		}
	}
	return missing, nil
}

func goneAsset(id string) error {
	return fmt.Errorf("publish: media asset %s: %w", id, mediadomain.ErrNotFound)
}

func freezeQuestion(ctx context.Context, tx pgx.Tx, sectionID string, q domain.DraftQuestion) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.test_version_questions
		       (test_version_section_id, ordinal, source_question_id, type, prompt,
		        media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
		        audio_show_transcript_after, transcript, points, explanation, sample_answer, prompt_content, explanation_content, level, skill, media_alt,
		        audio_allow_pause, select_count, max_words, match_allow_reuse)
		VALUES ($1, $2, $3, $4::app.question_type, $5, $6, $7::app.media_kind, $8, $9, $10,
		        $11, $12::numeric, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
		RETURNING id::text`,
		sectionID, q.Ordinal, q.SourceID, q.Type, q.Prompt,
		q.MediaAssetID, q.MediaAssetKind, q.MaxPlays, q.AllowSeek, q.ShowTranscript,
		q.Transcript, q.Points, q.Explanation, q.SampleAnswer, q.PromptContent, q.ExplanationContent, q.Level, q.Skill, q.MediaAlt,
		q.AllowPause, q.SelectCount, q.MaxWords, q.MatchAllowReuse).Scan(&id); err != nil {
		return "", fmt.Errorf("publish: freeze question: %w", err)
	}

	optionIDs, err := freezeOptions(ctx, tx, id, q.Options)
	if err != nil {
		return "", err
	}
	if err := freezeBlanks(ctx, tx, id, q.Blanks); err != nil {
		return "", err
	}
	if err := freezeMatchItems(ctx, tx, id, q.MatchItems, optionIDs); err != nil {
		return "", err
	}
	if err := freezeRubric(ctx, tx, id, q.Rubric); err != nil {
		return "", err
	}
	return id, nil
}

func freezeOptions(ctx context.Context, tx pgx.Tx, questionID string, options []domain.DraftOption) ([]string, error) {
	if len(options) == 0 {
		return nil, nil
	}
	ordered := slices.Clone(options)
	slices.SortStableFunc(ordered, func(a, b domain.DraftOption) int { return a.Ordinal - b.Ordinal })
	ids := make([]string, len(ordered))
	rows := make([][]any, len(ordered))
	for i, o := range ordered {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("publish: mint option id: %w", err)
		}
		ids[i] = id.String()
		rows[i] = []any{ids[i], questionID, o.Ordinal, o.Text, o.IsCorrect, o.Content}
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"app", "test_version_options"},
		[]string{"id", frozenOwner, "ordinal", "text", "is_correct", "content"},
		pgx.CopyFromRows(rows)); err != nil {
		return nil, fmt.Errorf("publish: freeze options: %w", err)
	}
	return ids, nil
}

func freezeMatchItems(ctx context.Context, tx pgx.Tx, questionID string, items []domain.DraftMatchItem, optionIDs []string) error {
	if len(items) == 0 {
		return nil
	}
	ordinals, texts, keys := make([]int, len(items)), make([]string, len(items)), make([]string, len(items))
	for i, item := range items {
		if item.CorrectOption < 0 || item.CorrectOption >= len(optionIDs) {
			return fmt.Errorf("publish: match item %d names option %d of %d", i, item.CorrectOption, len(optionIDs))
		}
		ordinals[i], texts[i], keys[i] = item.Ordinal, item.Text, optionIDs[item.CorrectOption]
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_match_items (test_version_question_id, ordinal, text, correct_option_id)
		SELECT $1, i.ordinal, i.text, i.option FROM unnest($2::int[], $3::text[], $4::uuid[]) AS i(ordinal, text, option)`,
		questionID, ordinals, texts, keys); err != nil {
		return fmt.Errorf("publish: freeze match items: %w", err)
	}
	return nil
}

func freezeRubric(ctx context.Context, tx pgx.Tx, questionID string, rubric []domain.DraftCriterion) error {
	if len(rubric) == 0 {
		return nil
	}
	ordinals, labels := make([]int, len(rubric)), make([]string, len(rubric))
	for i, criterion := range rubric {
		ordinals[i], labels[i] = criterion.Ordinal, criterion.Label
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label)
		SELECT $1, c.ordinal, c.label FROM unnest($2::int[], $3::text[]) AS c(ordinal, label)`,
		questionID, ordinals, labels); err != nil {
		return fmt.Errorf("publish: freeze rubric: %w", err)
	}
	return nil
}

func freezeBlanks(ctx context.Context, tx pgx.Tx, questionID string, blanks []domain.DraftBlank) error {
	for _, b := range blanks {
		var blankID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.test_version_blanks
			        (test_version_question_id, ordinal, case_sensitive, gap_id)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			questionID, b.Ordinal, b.CaseSensitive, b.GapID).Scan(&blankID); err != nil {
			return fmt.Errorf("publish: freeze blank: %w", err)
		}
		for _, answer := range b.AcceptedAnswers {
			if _, err := tx.Exec(ctx,
				`INSERT INTO app.test_version_blank_answers (test_version_blank_id, answer)
				 VALUES ($1, $2)`, blankID, answer); err != nil {
				return fmt.Errorf("publish: freeze blank answer: %w", err)
			}
		}
	}
	return nil
}

func snapshotMediaIDs(d domain.DraftContent) ([]string, error) {
	seen := make(map[string]bool)
	for _, section := range d.Sections {
		for _, question := range section.Questions {
			if question.MediaAssetID != nil {
				seen[strings.ToLower(*question.MediaAssetID)] = true
			}
		}
		if err := collectGroupAssets(section.Groups, seen); err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func collectGroupAssets(groups []domain.GroupBundle, seen map[string]bool) error {
	for _, group := range groups {
		for _, material := range group.Group.Stimuli {
			document, err := content.Parse(material.Content)
			if err != nil {
				return err
			}
			for _, asset := range document.Assets() {
				seen[strings.ToLower(asset.ID)] = true
			}
		}
	}
	return nil
}
