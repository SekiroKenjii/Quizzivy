package questions

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the slice of pgx both a pool and a transaction satisfy, so the
// check below can run inside the caller's transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// draftOutlineTable is the table the real implementation will query. Named once
// so the guard test and this comment cannot drift apart.
const draftOutlineTable = "test_section_questions"

// CountDraftReferences reports how many draft test outlines use this question.
// A non-zero count blocks deletion with a 409 (§15).
//
// Returns 0 unconditionally: `test_section_questions` does not exist until
// migration 00015 (T-2.7), so no reference can exist yet. When it does, this
// becomes
//
//	SELECT count(*) FROM app.test_section_questions WHERE question_id = $1
//
// This is the same shape as media.CountReferences and carries the same risk: a
// stub that outlives its blocker silently turns a 409 into a successful delete
// that breaks a teacher's draft. TestDraftReferenceCheckIsWiredOnceTestTables-
// Exist fails the moment the table appears, so it cannot stay quiet.
//
// The direction is chosen so being wrong is safe in the way that matters here.
// Returning 0 permits a delete that should have been refused -- but the delete
// is SOFT, so the row survives and a wrongly-deleted question is recoverable.
// Returning a non-zero count instead would block every delete in the bank.
func CountDraftReferences(ctx context.Context, q Querier, questionID string) (int, error) {
	_, _, _ = ctx, q, questionID
	return 0, nil
}
