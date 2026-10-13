package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
)

func appendLedger(ctx context.Context, tx pgx.Tx, s domain.Settlement, feature domain.Feature, charged int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.ai_usage_ledger
		       (user_id, month, feature, tier, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, credits, request_id)
		VALUES ($1::uuid, $2::date, $3, $4, $5, $6, $7, $8, $9, $10)`,
		s.Reservation.UserID, s.Reservation.Month, string(feature), string(s.Tier),
		s.Tokens.Input, s.Tokens.Output, s.Tokens.CacheRead, s.Tokens.CacheWrite, charged, opt.String(s.RequestID)); err != nil {
		return fmt.Errorf("ai: append to the ledger: %w", err)
	}
	return nil
}

func categorySums(ctx context.Context, q db.Querier, month time.Time, userID string) ([]domain.CategorySum, error) {
	byFeature, err := db.QueryMany(ctx, q, `
		SELECT feature, sum(credits)::bigint FROM app.ai_usage_ledger
		 WHERE user_id = $1::uuid AND month = $2::date GROUP BY feature`,
		[]any{userID, month}, func(rows pgx.Rows) (featureSum, error) {
			var f featureSum
			err := rows.Scan(&f.feature, &f.credits)
			return f, err
		})
	if err != nil {
		return nil, fmt.Errorf("ai: sum the ledger: %w", err)
	}
	totals := map[domain.Category]int64{}
	for _, f := range byFeature {
		totals[domain.Feature(f.feature).Category()] += f.credits
	}
	out := make([]domain.CategorySum, 0, len(domain.Categories()))
	for _, c := range domain.Categories() {
		out = append(out, domain.CategorySum{Category: c, Credits: totals[c]})
	}
	return out, nil
}

type featureSum struct {
	feature string
	credits int64
}
