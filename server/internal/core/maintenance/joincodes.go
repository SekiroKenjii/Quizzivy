package maintenance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
)

// RekeyReport is what rekey-join-codes found or did: the sealed codes by key
// id and the legacy codes when it finished; how many it moved from the
// previous key to the current one; how many under the previous key it would
// still move; the active codes sealed under a key id that is neither key;
// and the codes under the previous key whose ciphertext does not open.
type RekeyReport struct {
	CurrentKeyID  int16           `json:"currentKeyId"`
	PreviousKeyID int16           `json:"previousKeyId"`
	SealedByKey   map[int16]int64 `json:"sealedByKey"`
	Legacy        int64           `json:"legacy"`
	Moved         int64           `json:"moved"`
	Pending       int64           `json:"pending"`
	Unknown       int64           `json:"unknown"`
	Unopened      []UnopenedCode  `json:"unopened"`
	Applied       bool            `json:"applied"`
}

// UnopenedCode is a sealed code under the previous key that does not open:
// never its code, only where it is. An active one stops redeeming once the
// previous key is unset, so its class's code must be rotated first.
type UnopenedCode struct {
	ID      string `json:"id"`
	ClassID string `json:"classId"`
	Revoked bool   `json:"revoked"`
}

// RekeyJoinCodes re-seals and re-hashes under the current key every sealed
// join code, revoked or not, that the previous key sealed. It walks the codes
// in id order in batches of batch rows, one transaction each, and writes only
// code_hash, code_ciphertext and key_id. A code that does not open is listed
// in Unopened and left where it is. Legacy codes hold no ciphertext and
// cannot be re-keyed; R4 rotates them. keys must hold a previous key, and
// -apply refuses while an active code is sealed under a key id that is
// neither key, because one of the two keys given is then not the pair the
// API runs with. A dry run opens every code and writes nothing.
func RekeyJoinCodes(ctx context.Context, conn db.Conn, keys classesdomain.JoinCodeKeys, apply bool, batch int) (RekeyReport, error) {
	previous, ok := keys.PreviousID()
	if !ok {
		return RekeyReport{}, errors.New("rekey-join-codes needs JOIN_CODE_KEY_PREVIOUS: it moves codes from the previous key to JOIN_CODE_KEY")
	}
	if batch < 1 || batch > 10000 {
		return RekeyReport{}, errors.New("batch must be between 1 and 10000")
	}
	report := RekeyReport{CurrentKeyID: keys.CurrentID(), PreviousKeyID: previous, Applied: apply, Unopened: []UnopenedCode{}}
	if err := countSealed(ctx, conn, &report); err != nil {
		return report, err
	}
	if apply && report.Unknown > 0 {
		return report, fmt.Errorf("%d active join code(s) are sealed under a key id that is neither JOIN_CODE_KEY (%d) nor JOIN_CODE_KEY_PREVIOUS (%d); check both keys against the ids the API logs at startup", report.Unknown, report.CurrentKeyID, report.PreviousKeyID)
	}
	after := ""
	for {
		page, err := rekeyBatch(ctx, conn, keys, previous, apply, batch, after)
		report.Moved += page.moved
		report.Unopened = append(report.Unopened, page.unopened...)
		if err != nil {
			return report, err
		}
		if page.seen < batch {
			break
		}
		after = page.last
	}
	if err := countSealed(ctx, conn, &report); err != nil {
		return report, err
	}
	report.Pending = report.SealedByKey[previous] - int64(len(report.Unopened))
	return report, nil
}

type sealedRow struct {
	id, classID string
	revoked     bool
	ciphertext  []byte
}

type rekeyPage struct {
	seen     int
	moved    int64
	last     string
	unopened []UnopenedCode
}

func rekeyBatch(ctx context.Context, conn db.Conn, keys classesdomain.JoinCodeKeys, previous int16, apply bool, batch int, after string) (rekeyPage, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return rekeyPage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lock := ``
	if apply {
		lock = ` FOR UPDATE`
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, class_id::text, revoked_at IS NOT NULL, code_ciphertext
		  FROM app.class_join_codes
		 WHERE lookup_scheme = 2 AND key_id = $1 AND ($3::uuid IS NULL OR id > $3::uuid)
		 ORDER BY id
		 LIMIT $2`+lock, previous, batch, opt.String(after))
	if err != nil {
		return rekeyPage{}, fmt.Errorf("select join codes to re-key: %w", err)
	}
	sealed, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (sealedRow, error) {
		var s sealedRow
		return s, r.Scan(&s.id, &s.classID, &s.revoked, &s.ciphertext)
	})
	if err != nil {
		return rekeyPage{}, fmt.Errorf("read join codes to re-key: %w", err)
	}
	page := rekeyPage{seen: len(sealed)}
	updates := &pgx.Batch{}
	for _, s := range sealed {
		page.last = s.id
		code, err := keys.Open(s.classID, s.id, previous, s.ciphertext)
		if err != nil {
			page.unopened = append(page.unopened, UnopenedCode{ID: s.id, ClassID: s.classID, Revoked: s.revoked})
			continue
		}
		resealed, err := keys.Seal(s.classID, s.id, code)
		if err != nil {
			return rekeyPage{}, fmt.Errorf("re-seal join code %s: %w", s.id, err)
		}
		updates.Queue(`
			UPDATE app.class_join_codes
			   SET code_hash = $2, code_ciphertext = $3, key_id = $4
			 WHERE id = $1 AND key_id = $5`,
			s.id, keys.Hash(code), resealed, keys.CurrentID(), previous)
	}
	if !apply || updates.Len() == 0 {
		return page, nil
	}
	if err := tx.SendBatch(ctx, updates).Close(); err != nil {
		return rekeyPage{}, fmt.Errorf("re-key join codes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return rekeyPage{}, fmt.Errorf("commit re-keyed join codes: %w", err)
	}
	page.moved = int64(updates.Len())
	return page, nil
}

func countSealed(ctx context.Context, conn db.Querier, report *RekeyReport) error {
	rows, err := conn.Query(ctx, `
		SELECT lookup_scheme, key_id, count(*), count(*) FILTER (WHERE revoked_at IS NULL)
		  FROM app.class_join_codes
		 GROUP BY lookup_scheme, key_id`)
	if err != nil {
		return fmt.Errorf("count join codes: %w", err)
	}
	defer rows.Close()
	report.SealedByKey = map[int16]int64{}
	report.Legacy, report.Unknown = 0, 0
	for rows.Next() {
		var scheme int16
		var keyID *int16
		var n, active int64
		if err := rows.Scan(&scheme, &keyID, &n, &active); err != nil {
			return fmt.Errorf("count join codes: %w", err)
		}
		if scheme == int16(classesdomain.LookupLegacy) || keyID == nil {
			report.Legacy += n
			continue
		}
		report.SealedByKey[*keyID] += n
		if *keyID != report.CurrentKeyID && *keyID != report.PreviousKeyID {
			report.Unknown += active
		}
	}
	return rows.Err()
}
