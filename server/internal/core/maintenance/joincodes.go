package maintenance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
)

// RekeyReport is what rekey-join-codes found or did: the sealed codes by key
// id and the legacy codes when it finished, how many it moved from the
// previous key to the current one, and how many are still under the previous
// key. A dry run moves nothing, so Pending is what -apply would move.
type RekeyReport struct {
	CurrentKeyID  int16           `json:"currentKeyId"`
	PreviousKeyID int16           `json:"previousKeyId"`
	SealedByKey   map[int16]int64 `json:"sealedByKey"`
	Legacy        int64           `json:"legacy"`
	Moved         int64           `json:"moved"`
	Pending       int64           `json:"pending"`
	Applied       bool            `json:"applied"`
}

// RekeyJoinCodes re-seals and re-hashes under the current key every sealed
// join code, revoked or not, that the previous key sealed. It works in
// batches of batch rows, one transaction each, and writes only code_hash,
// code_ciphertext and key_id. Legacy codes hold no ciphertext and cannot be
// re-keyed; R4 rotates them. keys must hold a previous key. A row that does
// not open under the previous key stops the command, naming the row and
// never the code.
func RekeyJoinCodes(ctx context.Context, conn db.Conn, keys classesdomain.JoinCodeKeys, apply bool, batch int) (RekeyReport, error) {
	previous, ok := keys.PreviousID()
	if !ok {
		return RekeyReport{}, errors.New("rekey-join-codes needs JOIN_CODE_KEY_PREVIOUS: it moves codes from the previous key to JOIN_CODE_KEY")
	}
	if batch < 1 || batch > 10000 {
		return RekeyReport{}, errors.New("batch must be between 1 and 10000")
	}
	report := RekeyReport{CurrentKeyID: keys.CurrentID(), PreviousKeyID: previous, Applied: apply}
	for apply {
		moved, err := rekeyBatch(ctx, conn, keys, previous, batch)
		report.Moved += moved
		if err != nil {
			return report, err
		}
		if moved < int64(batch) {
			break
		}
	}
	if err := countSealed(ctx, conn, &report); err != nil {
		return report, err
	}
	report.Pending = report.SealedByKey[previous]
	return report, nil
}

type sealedRow struct {
	id, classID string
	ciphertext  []byte
}

func rekeyBatch(ctx context.Context, conn db.Conn, keys classesdomain.JoinCodeKeys, previous int16, batch int) (int64, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id::text, class_id::text, code_ciphertext
		  FROM app.class_join_codes
		 WHERE lookup_scheme = 2 AND key_id = $1
		 ORDER BY id
		 LIMIT $2
		   FOR UPDATE`, previous, batch)
	if err != nil {
		return 0, fmt.Errorf("select join codes to re-key: %w", err)
	}
	sealed, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (sealedRow, error) {
		var s sealedRow
		return s, r.Scan(&s.id, &s.classID, &s.ciphertext)
	})
	if err != nil {
		return 0, fmt.Errorf("read join codes to re-key: %w", err)
	}
	for _, s := range sealed {
		code, err := keys.Open(s.classID, s.id, previous, s.ciphertext)
		if err != nil {
			return 0, fmt.Errorf("join code %s does not open under the previous key; revoke it and run again", s.id)
		}
		resealed, err := keys.Seal(s.classID, s.id, code)
		if err != nil {
			return 0, fmt.Errorf("re-seal join code %s: %w", s.id, err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.class_join_codes
			   SET code_hash = $2, code_ciphertext = $3, key_id = $4
			 WHERE id = $1 AND key_id = $5`,
			s.id, keys.Hash(code), resealed, keys.CurrentID(), previous); err != nil {
			return 0, fmt.Errorf("re-key join code %s: %w", s.id, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit re-keyed join codes: %w", err)
	}
	return int64(len(sealed)), nil
}

func countSealed(ctx context.Context, conn db.Querier, report *RekeyReport) error {
	rows, err := conn.Query(ctx, `
		SELECT lookup_scheme, key_id, count(*)
		  FROM app.class_join_codes
		 GROUP BY lookup_scheme, key_id`)
	if err != nil {
		return fmt.Errorf("count join codes: %w", err)
	}
	defer rows.Close()
	report.SealedByKey = map[int16]int64{}
	for rows.Next() {
		var scheme int16
		var keyID *int16
		var n int64
		if err := rows.Scan(&scheme, &keyID, &n); err != nil {
			return fmt.Errorf("count join codes: %w", err)
		}
		if scheme == int16(classesdomain.LookupLegacy) || keyID == nil {
			report.Legacy += n
			continue
		}
		report.SealedByKey[*keyID] += n
	}
	return rows.Err()
}
