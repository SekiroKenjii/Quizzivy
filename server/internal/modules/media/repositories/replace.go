package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/limits"
	"quizzivy/internal/shared/opt"
	"strings"
	"time"
)

// FindReplacementTarget reads one scoped live library asset and its owner without locking.
func (s *Postgres) FindReplacementTarget(ctx context.Context, scope access.Scope, id string) (domain.ReplacementTarget, error) {
	return replacementTarget(ctx, s.Conn(), scope, id)
}
func replacementTarget(ctx context.Context, q db.Querier, scope access.Scope, id string) (domain.ReplacementTarget, error) {
	var t domain.ReplacementTarget
	var kind string
	a := &t.Asset
	err := q.QueryRow(ctx, `SELECT `+assetColumns+`,a.owner_id::text FROM app.media_assets a WHERE `+libraryAsset, id, scope.All, opt.String(scope.UserID)).Scan(&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes, &a.DurationMs, &a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt, &a.DisplayName, &a.DefaultMaxPlays, &a.Width, &a.Height, &t.OwnerID)
	a.Kind = domain.Kind(kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReplacementTarget{}, domain.ErrNotFound
	}
	return t, err
}

// Replace requires a concrete pool and commits one immutable replacement with its editable references.
func (s *Postgres) Replace(ctx context.Context, in domain.ReplaceInput) (domain.ReplaceResult, error) {
	pool, ok := s.Conn().(*pgxpool.Pool)
	if !ok || pool == nil {
		return domain.ReplaceResult{}, &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: errors.New("media: replacement requires a concrete pool")}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return domain.ReplaceResult{}, &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: err}
	}
	result, err := replaceBody(ctx, tx, in)
	if err != nil {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		rollbackErr := tx.Rollback(rollbackCtx)
		cancel()
		return domain.ReplaceResult{}, &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: err, RollbackError: rollbackErr}
	}
	if err := tx.Commit(ctx); err != nil {
		outcome := domain.ReplacementUnknown
		if errors.Is(err, pgx.ErrTxCommitRollback) || pgconn.SafeToRetry(err) {
			outcome = domain.ReplacementNotCommitted
		}
		return domain.ReplaceResult{}, &domain.ReplacementError{Outcome: outcome, Cause: err}
	}
	return result, nil
}

const replacementGroupJoins = ` FROM app.question_groups g LEFT JOIN app.test_sections s ON s.id=g.owner_section_id LEFT JOIN app.tests t ON t.id=s.test_id AND t.deleted_at IS NULL `
const replacementGroupOwner = `CASE WHEN g.owner_section_id IS NULL THEN g.owner_id ELSE t.owner_id END`
const replacementGroupsUsing = `g.id IN (SELECT group_id FROM app.group_recordings WHERE media_asset_id=$1 UNION SELECT group_id FROM app.group_stimulus_assets WHERE media_asset_id=$1 UNION SELECT context_group_id FROM app.questions WHERE media_asset_id=$1 AND context_group_id IS NOT NULL AND deleted_at IS NULL UNION SELECT q.context_group_id FROM app.question_media_references r JOIN app.questions q ON q.id=r.question_id WHERE r.media_asset_id=$1 AND q.context_group_id IS NOT NULL AND q.deleted_at IS NULL)`

// replacementQuestionUses is a question that attaches the asset or shows it in its prompt or explanation (DG-109).
const replacementQuestionUses = `(q.media_asset_id=$1 OR EXISTS (SELECT 1 FROM app.question_media_references r WHERE r.question_id=q.id AND r.media_asset_id=$1))`

func replacementGroups(ctx context.Context, tx pgx.Tx, id, owner string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT g.id::text`+replacementGroupJoins+`WHERE `+replacementGroupOwner+`=$2::uuid AND `+replacementGroupsUsing+` ORDER BY g.id FOR UPDATE OF g`, id, owner)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
func replacementQuestions(ctx context.Context, tx pgx.Tx, id, owner string, groups []string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT q.id::text FROM app.questions q WHERE `+replacementQuestionUses+` AND q.deleted_at IS NULL AND ((q.context_group_id IS NULL AND q.owner_id=$2::uuid) OR q.context_group_id=ANY($3::uuid[])) ORDER BY q.id FOR UPDATE OF q`, id, owner, groups)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
func replaceBody(ctx context.Context, tx pgx.Tx, in domain.ReplaceInput) (domain.ReplaceResult, error) {
	selected, err := replacementTarget(ctx, tx, in.Scope, in.ID)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	groups, err := replacementGroups(ctx, tx, in.ID, selected.OwnerID)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	questions, err := replacementQuestions(ctx, tx, in.ID, selected.OwnerID, groups)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	if err := LockForVersionUse(ctx, tx, in.ID); err != nil {
		return domain.ReplaceResult{}, err
	}
	locked, err := replacementTarget(ctx, tx, in.Scope, in.ID)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	if locked.OwnerID != selected.OwnerID {
		return domain.ReplaceResult{}, domain.ErrNotFound
	}
	if locked.Asset.Kind != in.Asset.Kind {
		return domain.ReplaceResult{}, domain.ErrKindMismatch
	}
	in.Asset.OwnerID = locked.OwnerID
	in.Asset.DisplayName = &locked.Asset.DisplayName
	in.Asset.DefaultMaxPlays = locked.Asset.DefaultMaxPlays
	if err := LockQuota(ctx, tx, locked.OwnerID); err != nil {
		return domain.ReplaceResult{}, err
	}
	asset, err := Insert(ctx, tx, in.Asset)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	result, err := replacementGraph(ctx, tx, in, asset, questions, groups)
	if err != nil {
		return result, err
	}
	if result.Previous, err = retireReplaced(ctx, tx, in, locked.Asset); err != nil {
		return result, err
	}
	if err := CheckQuota(ctx, tx, QuotaCheck{OwnerID: locked.OwnerID, Quota: in.Asset.QuotaBytes}); err != nil {
		return result, err
	}
	if in.Asset.StorageLimit != nil {
		if err := limits.RequireStorage(ctx, tx, *in.Asset.StorageLimit, 0); err != nil {
			return result, err
		}
	}
	return result, nil
}
func retireReplaced(ctx context.Context, tx pgx.Tx, in domain.ReplaceInput, old domain.Asset) (domain.Retired, error) {
	usedIn, err := References(ctx, tx, in.ID)
	if err != nil {
		return domain.Retired{}, err
	}
	return describeRetired(ctx, tx, old, usedIn, in.Asset.Now)
}
func replacementGraph(ctx context.Context, tx pgx.Tx, in domain.ReplaceInput, asset domain.Asset, questions, groups []string) (domain.ReplaceResult, error) {
	result := domain.ReplaceResult{Asset: asset}
	if _, err := tx.Exec(ctx, `UPDATE app.media_assets SET replaced_by=$2 WHERE id=$1`, in.ID, asset.ID); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `UPDATE app.questions SET media_asset_id=$2 WHERE id=ANY($3::uuid[]) AND media_asset_id=$1 RETURNING id::text, context_group_id::text`, in.ID, asset.ID, questions)
	if err != nil {
		return result, err
	}
	changed, repointed := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var question string
		var group *string
		if err := rows.Scan(&question, &group); err != nil {
			rows.Close()
			return result, err
		}
		repointed[question] = true
		if group != nil {
			changed[*group] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if err := repointQuestionProse(ctx, tx, in.ID, asset.ID, questions, repointed, changed); err != nil {
		return result, err
	}
	result.Repointed.Questions = len(repointed)
	if err := repointGroupRows(ctx, tx, in.ID, asset.ID, groups, changed); err != nil {
		return result, err
	}
	if err := repointGroupContent(ctx, tx, in.ID, asset.ID, groups, changed); err != nil {
		return result, err
	}
	changedIDs := make([]string, 0, len(changed))
	for id := range changed {
		changedIDs = append(changedIDs, id)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.question_groups SET revision=revision+1 WHERE id=ANY($1::uuid[])`, changedIDs); err != nil {
		return result, err
	}
	result.Repointed.Groups = len(changedIDs)
	result.Left, err = replacementLeft(ctx, tx, in.ID, in.Asset.OwnerID)
	if err != nil {
		return result, err
	}
	diff := map[string]any{"replacedBy": asset.ID, "repointed": map[string]int{"questions": result.Repointed.Questions, "groups": result.Repointed.Groups}, "left": map[string]int{"questions": result.Left.Questions, "groups": result.Left.Groups}}
	encoded, err := json.Marshal(diff)
	if err != nil {
		return result, err
	}
	err = audit.Write(ctx, tx, audit.Entry{ActorUserID: &in.Asset.UploaderID, Action: "media.replaced", Entity: auditedEntity, EntityID: &in.ID, OccurredAt: in.Asset.Now, IP: in.Asset.IP, UserAgent: in.Asset.UserAgent, Diff: encoded})
	return result, err
}
func repointGroupRows(ctx context.Context, tx pgx.Tx, old, newID string, groups []string, changed map[string]bool) error {
	for _, table := range []string{"group_recordings", "group_stimulus_assets"} {
		rows, err := tx.Query(ctx, `UPDATE app.`+table+` SET media_asset_id=$2 WHERE group_id=ANY($3::uuid[]) AND media_asset_id=$1 RETURNING group_id::text`, old, newID, groups)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			changed[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type replacementProse struct {
	id                  string
	group               *string
	prompt, explanation []byte
}

// repointQuestionProse rewrites the live questions' prompts and explanations
// that show the old asset to show the new one, and moves their reference rows
// with them, as repointGroupContent does for a group's materials (DG-109). A
// question counts once however many of its fields changed; frozen rows stay
// as published.
func repointQuestionProse(ctx context.Context, tx pgx.Tx, old, newID string, questions []string, repointed, changed map[string]bool) error {
	rows, err := tx.Query(ctx, `SELECT q.id::text, q.context_group_id::text, q.prompt_content, q.explanation_content FROM app.questions q
		WHERE q.id=ANY($2::uuid[]) AND EXISTS (SELECT 1 FROM app.question_media_references r WHERE r.question_id=q.id AND r.media_asset_id=$1) ORDER BY q.id`, old, questions)
	if err != nil {
		return err
	}
	shown, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (replacementProse, error) {
		var p replacementProse
		err := row.Scan(&p.id, &p.group, &p.prompt, &p.explanation)
		return p, err
	})
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(shown))
	for _, p := range shown {
		prompt, err := rebindProse(content.ParsePromptWithMedia, p.prompt, old, newID)
		if err != nil {
			return err
		}
		explanation, err := rebindProse(content.ParseExplanationWithMedia, p.explanation, old, newID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.questions SET prompt_content=$2, explanation_content=$3 WHERE id=$1`, p.id, prompt, explanation); err != nil {
			return err
		}
		ids = append(ids, p.id)
		repointed[p.id] = true
		if p.group != nil {
			changed[*p.group] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind)
		SELECT question_id, field, $2, media_asset_kind FROM app.question_media_references WHERE question_id=ANY($3::uuid[]) AND media_asset_id=$1
		ON CONFLICT DO NOTHING`, old, newID, ids); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM app.question_media_references WHERE question_id=ANY($2::uuid[]) AND media_asset_id=$1`, old, ids)
	return err
}

// rebindProse is raw with the old asset named as the new one, under the
// field's media rules; a document that does not name it is returned as stored.
func rebindProse(parse func([]byte) (content.Document, error), raw []byte, old, newID string) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	document, err := parse(raw)
	if err != nil {
		return nil, err
	}
	if !documentNames(document, old) {
		return raw, nil
	}
	rewritten, err := document.WithAssetIDs(map[string]string{old: newID})
	if err != nil {
		return nil, err
	}
	return rewritten.MarshalJSON()
}

type replacementStimulus struct {
	id, group string
	body      []byte
}

func repointGroupContent(ctx context.Context, tx pgx.Tx, old, newID string, groups []string, changed map[string]bool) error {
	rows, err := tx.Query(ctx, `SELECT id::text,group_id::text,content FROM app.group_stimuli WHERE group_id=ANY($1::uuid[]) ORDER BY id`, groups)
	if err != nil {
		return err
	}
	stimuli, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (replacementStimulus, error) {
		var s replacementStimulus
		err := row.Scan(&s.id, &s.group, &s.body)
		return s, err
	})
	if err != nil {
		return err
	}
	for _, s := range stimuli {
		document, err := content.Parse(s.body)
		if err != nil {
			return err
		}
		if !documentNames(document, old) {
			continue
		}
		rewritten, err := document.WithAssetIDs(map[string]string{old: newID})
		if err != nil {
			return err
		}
		raw, err := rewritten.MarshalJSON()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.group_stimuli SET content=$2 WHERE id=$1 AND group_id=ANY($3::uuid[])`, s.id, raw, groups); err != nil {
			return err
		}
		changed[s.group] = true
	}
	return nil
}
func documentNames(d content.Document, id string) bool {
	for _, a := range d.Assets() {
		if strings.EqualFold(a.ID, id) {
			return true
		}
	}
	return false
}
func replacementLeft(ctx context.Context, tx pgx.Tx, id, owner string) (domain.ReplacementCounts, error) {
	var counts domain.ReplacementCounts
	err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions q LEFT JOIN app.question_groups g ON g.id=q.context_group_id LEFT JOIN app.test_sections s ON s.id=g.owner_section_id LEFT JOIN app.tests t ON t.id=s.test_id AND t.deleted_at IS NULL WHERE `+replacementQuestionUses+` AND q.deleted_at IS NULL AND CASE WHEN q.context_group_id IS NULL THEN q.owner_id ELSE `+replacementGroupOwner+` END <> $2::uuid`, id, owner).Scan(&counts.Questions)
	if err != nil {
		return counts, err
	}
	err = tx.QueryRow(ctx, `SELECT count(*)`+replacementGroupJoins+` WHERE `+replacementGroupOwner+`<>$2::uuid AND `+replacementGroupsUsing, id, owner).Scan(&counts.Groups)
	return counts, err
}
