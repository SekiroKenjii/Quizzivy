package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

type versionRow struct {
	ID          string
	Version     int
	PublishedAt time.Time
}

// DiffPapers reads the two papers of a comparison in one REPEATABLE READ, READ
// ONLY transaction, so both come from one snapshot, and writes nothing and
// locks no row: it never touches the draft's group locks and runs no FOR
// clause. A test outside the request's scope, and a version that does not
// exist, are domain.ErrNotFound; a test with no version, when the request asks
// for the latest, is domain.ErrNotPublished; and a draft whose groups are
// refused is domain.ErrDraftUnreadable, as a version whose stored group fails
// its validation is domain.ErrVersionUnreadable, each wrapping the cause.
//
// It cannot run on a Postgres built over a transaction: Begin there is a
// savepoint, and SET TRANSACTION is refused once a statement has run.
func (s *Postgres) DiffPapers(ctx context.Context, req domain.DiffRequest) (domain.DiffPapers, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.DiffPapers{}, fmt.Errorf("diff: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY`); err != nil {
		return domain.DiffPapers{}, fmt.Errorf("diff: set transaction: %w", err)
	}
	versions, err := readDiffVersions(ctx, tx, req)
	if err != nil {
		return domain.DiffPapers{}, err
	}
	from, to, err := chooseSides(versions, req)
	if err != nil {
		return domain.DiffPapers{}, err
	}
	papers := domain.DiffPapers{}
	if from != nil {
		paper, err := s.readDiffPaper(ctx, tx, req.TestID, *from)
		if err != nil {
			return domain.DiffPapers{}, err
		}
		papers.From = &paper
	}
	if papers.To, err = s.readDiffPaper(ctx, tx, req.TestID, *to); err != nil {
		return domain.DiffPapers{}, err
	}
	return papers, nil
}

func readDiffVersions(ctx context.Context, tx pgx.Tx, req domain.DiffRequest) ([]versionRow, error) {
	var visible string
	err := tx.QueryRow(ctx, `SELECT id::text FROM app.tests WHERE id = $1 AND deleted_at IS NULL AND `+scopedTest,
		req.TestID, req.Scope.All, opt.String(req.Scope.UserID)).Scan(&visible)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("diff: read test: %w", err)
	}
	versions, err := db.QueryMany(ctx, tx, `SELECT id::text, version, published_at FROM app.test_versions
		WHERE test_id = $1 ORDER BY version`, []any{req.TestID},
		func(rows pgx.Rows) (versionRow, error) {
			var row versionRow
			err := rows.Scan(&row.ID, &row.Version, &row.PublishedAt)
			return row, err
		})
	if err != nil {
		return nil, fmt.Errorf("diff: read versions: %w", err)
	}
	return versions, nil
}

type diffSide struct {
	draft   bool
	version versionRow
}

func chooseSides(versions []versionRow, req domain.DiffRequest) (*diffSide, *diffSide, error) {
	base, err := baseVersion(versions, req.Version)
	if err != nil {
		return nil, nil, err
	}
	switch req.Against.Kind {
	case domain.AgainstDraft:
		return &diffSide{version: base}, &diffSide{draft: true}, nil
	case domain.AgainstPrevious:
		from, to := previousSides(versions, base)
		return from, to, nil
	case domain.AgainstVersion:
		return numberedSides(versions, base, req.Against.Version)
	}
	return nil, nil, domain.ErrBadAgainst
}

func baseVersion(versions []versionRow, number int) (versionRow, error) {
	switch {
	case len(versions) == 0 && number == 0:
		return versionRow{}, domain.ErrNotPublished
	case len(versions) == 0:
		return versionRow{}, domain.ErrNotFound
	case number == 0:
		return versions[len(versions)-1], nil
	}
	if row, found := findVersion(versions, number); found {
		return row, nil
	}
	return versionRow{}, domain.ErrNotFound
}

func findVersion(versions []versionRow, number int) (versionRow, bool) {
	index := slices.IndexFunc(versions, func(v versionRow) bool { return v.Version == number })
	if index < 0 {
		return versionRow{}, false
	}
	return versions[index], true
}

func previousSides(versions []versionRow, base versionRow) (*diffSide, *diffSide) {
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].Version < base.Version {
			return &diffSide{version: versions[i]}, &diffSide{version: base}
		}
	}
	return nil, &diffSide{version: base}
}

func numberedSides(versions []versionRow, base versionRow, number int) (*diffSide, *diffSide, error) {
	other, found := findVersion(versions, number)
	switch {
	case !found:
		return nil, nil, domain.ErrNotFound
	case other.Version == base.Version:
		return nil, nil, domain.ErrSameVersion
	case other.Version < base.Version:
		return &diffSide{version: other}, &diffSide{version: base}, nil
	}
	return &diffSide{version: base}, &diffSide{version: other}, nil
}

func (s *Postgres) readDiffPaper(ctx context.Context, tx pgx.Tx, testID string, side diffSide) (domain.DiffPaper, error) {
	if side.draft {
		content, err := s.loadDiffDraft(ctx, tx, testID)
		return domain.DiffPaper{Side: domain.DiffSide{Draft: true}, Content: content}, err
	}
	content, err := s.loadVersion(ctx, tx, side.version.ID)
	if err != nil {
		return domain.DiffPaper{}, err
	}
	content.TestID = testID
	return domain.DiffPaper{
		Side:    domain.DiffSide{Version: side.version.Version, PublishedAt: side.version.PublishedAt},
		Content: content,
	}, nil
}

func (s *Postgres) loadDiffDraft(ctx context.Context, tx pgx.Tx, testID string) (domain.DraftContent, error) {
	content, err := s.loadDraftSnapshot(ctx, tx, testID)
	var group *domain.GroupError
	switch {
	case err == nil:
		return content, nil
	case errors.Is(err, domain.ErrNoContent):
		return domain.DraftContent{TestID: testID}, nil
	case errors.As(err, &group), errors.Is(err, domain.ErrUnknownQuestion), errors.Is(err, errGroupOtherSection):
		return domain.DraftContent{}, fmt.Errorf("%w: %w", domain.ErrDraftUnreadable, err)
	}
	return domain.DraftContent{}, fmt.Errorf("diff: read draft: %w", err)
}

func (s *Postgres) loadVersion(ctx context.Context, tx pgx.Tx, versionID string) (domain.DraftContent, error) {
	sections, order, err := loadVersionSections(ctx, tx, versionID)
	if err != nil {
		return domain.DraftContent{}, err
	}
	questions, err := loadVersionQuestions(ctx, tx, versionID)
	if err != nil {
		return domain.DraftContent{}, err
	}
	units, err := loadVersionUnits(ctx, tx, versionID)
	if err != nil {
		return domain.DraftContent{}, err
	}
	var content domain.DraftContent
	for _, id := range order {
		section := sections[id]
		section.Questions = questions[id]
		section.Units = units[id]
		for _, unit := range section.Units {
			if unit.GroupID == "" {
				continue
			}
			group, err := readFrozenGroup(ctx, tx, unit.GroupID, versionUnlocked)
			if err != nil {
				return domain.DraftContent{}, versionGroupError(err)
			}
			section.Groups = append(section.Groups, group)
		}
		content.Sections = append(content.Sections, section)
	}
	return content, nil
}

func versionGroupError(err error) error {
	var invalid *domain.GroupError
	if errors.As(err, &invalid) || errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("%w: %w", domain.ErrVersionUnreadable, err)
	}
	return fmt.Errorf("diff: read group of version: %w", err)
}

func loadVersionSections(ctx context.Context, tx pgx.Tx, versionID string) (map[string]domain.DraftSection, []string, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, ordinal, title, instructions
		FROM app.test_version_sections WHERE test_version_id = $1 ORDER BY ordinal`, versionID)
	if err != nil {
		return nil, nil, fmt.Errorf("diff: read version sections: %w", err)
	}
	defer rows.Close()
	sections := map[string]domain.DraftSection{}
	var order []string
	for rows.Next() {
		var section domain.DraftSection
		if err := rows.Scan(&section.ID, &section.Ordinal, &section.Title, &section.Instructions); err != nil {
			return nil, nil, fmt.Errorf("diff: scan version section: %w", err)
		}
		sections[section.ID] = section
		order = append(order, section.ID)
	}
	return sections, order, rows.Err()
}

func loadVersionQuestions(ctx context.Context, tx pgx.Tx, versionID string) (map[string][]domain.DraftQuestion, error) {
	rows, err := tx.Query(ctx, `SELECT q.test_version_section_id::text, q.ordinal, q.id::text, coalesce(q.source_question_id::text, ''),
		       q.type::text, q.prompt, q.media_asset_id::text, q.media_asset_kind::text, q.media_alt,
		       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after,
		       q.transcript, q.points::text, q.explanation, q.sample_answer, q.prompt_content, q.explanation_content
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		 WHERE s.test_version_id = $1
		 ORDER BY s.ordinal, q.ordinal`, versionID)
	if err != nil {
		return nil, fmt.Errorf("diff: read version questions: %w", err)
	}
	defer rows.Close()
	bySection := map[string][]domain.DraftQuestion{}
	for rows.Next() {
		var sectionID string
		var q domain.DraftQuestion
		if err := rows.Scan(&sectionID, &q.Ordinal, &q.FrozenID, &q.SourceID, &q.Type, &q.Prompt,
			&q.MediaAssetID, &q.MediaAssetKind, &q.MediaAlt, &q.MaxPlays, &q.AllowSeek, &q.ShowTranscript,
			&q.Transcript, &q.Points, &q.Explanation, &q.SampleAnswer, &q.PromptContent, &q.ExplanationContent); err != nil {
			return nil, fmt.Errorf("diff: scan version question: %w", err)
		}
		bySection[sectionID] = append(bySection[sectionID], q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("diff: read version questions: %w", err)
	}
	if len(bySection) == 0 {
		return bySection, nil
	}
	options, err := loadVersionOptions(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	blanks, err := loadVersionBlanks(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	for _, list := range bySection {
		for i := range list {
			list[i].Options = options[list[i].FrozenID]
			list[i].Blanks = blanks[list[i].FrozenID]
		}
	}
	return bySection, nil
}

func loadVersionOptions(ctx context.Context, tx pgx.Tx, versionID string) (map[string][]domain.DraftOption, error) {
	options, err := db.GroupBy(ctx, tx, `SELECT o.test_version_question_id::text, o.ordinal, o.text, o.is_correct, o.content
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_options o ON o.test_version_question_id = q.id
		 WHERE s.test_version_id = $1
		 ORDER BY o.test_version_question_id, o.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.DraftOption, error) {
			var questionID string
			var option domain.DraftOption
			err := rows.Scan(&questionID, &option.Ordinal, &option.Text, &option.IsCorrect, &option.Content)
			return questionID, option, err
		})
	if err != nil {
		return nil, fmt.Errorf("diff: read version options: %w", err)
	}
	return options, nil
}

func loadVersionBlanks(ctx context.Context, tx pgx.Tx, versionID string) (map[string][]domain.DraftBlank, error) {
	blanks, err := db.GroupBy(ctx, tx, `SELECT b.test_version_question_id::text, b.ordinal, b.gap_id, b.case_sensitive,
		       coalesce(array_agg(a.answer ORDER BY a.answer) FILTER (WHERE a.answer IS NOT NULL), '{}')
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_blanks b ON b.test_version_question_id = q.id
		  LEFT JOIN app.test_version_blank_answers a ON a.test_version_blank_id = b.id
		 WHERE s.test_version_id = $1
		 GROUP BY b.test_version_question_id, b.id, b.ordinal, b.gap_id, b.case_sensitive
		 ORDER BY b.test_version_question_id, b.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.DraftBlank, error) {
			var questionID string
			var blank domain.DraftBlank
			err := rows.Scan(&questionID, &blank.Ordinal, &blank.GapID, &blank.CaseSensitive, &blank.AcceptedAnswers)
			return questionID, blank, err
		})
	if err != nil {
		return nil, fmt.Errorf("diff: read version blanks: %w", err)
	}
	return blanks, nil
}

func loadVersionUnits(ctx context.Context, tx pgx.Tx, versionID string) (map[string][]domain.DraftUnit, error) {
	units, err := db.GroupBy(ctx, tx, `SELECT s.id::text, coalesce(u.question_id::text, ''), coalesce(u.group_id::text, '')
		  FROM app.test_version_sections s
		  JOIN app.test_version_units u ON u.test_version_section_id = s.id
		 WHERE s.test_version_id = $1
		 ORDER BY s.ordinal, u.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.DraftUnit, error) {
			var sectionID string
			var unit domain.DraftUnit
			err := rows.Scan(&sectionID, &unit.QuestionID, &unit.GroupID)
			return sectionID, unit, err
		})
	if err != nil {
		return nil, fmt.Errorf("diff: read version units: %w", err)
	}
	return units, nil
}
