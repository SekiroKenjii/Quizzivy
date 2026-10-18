package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"slices"

	"github.com/jackc/pgx/v5"
)

// TransferLedger implements domain.TransferLedger over a db.Context bound to
// the transfer's transaction.
type TransferLedger struct{ db.Repository }

// NewTransferLedger returns the ledger of a transfer that runs on dbx.
func NewTransferLedger(dbx db.Context) *TransferLedger {
	return &TransferLedger{Repository: db.NewRepository(dbx)}
}

type transferParty struct {
	ID       string
	RoleID   string
	Disabled bool
}

// LockTransfer implements domain.TransferLedger.
func (l *TransferLedger) LockTransfer(ctx context.Context, in domain.ContentTransfer) error {
	rows, err := l.Query(ctx, `
		SELECT u.id::text, u.role_id::text, u.disabled_at IS NOT NULL
		  FROM app.users u
		 WHERE u.id = ANY($1::uuid[])
		 ORDER BY u.id
		   FOR NO KEY UPDATE`, []string{in.FromID, in.ToID})
	if err != nil {
		return fmt.Errorf("users: lock transfer accounts: %w", err)
	}
	parties, err := pgx.CollectRows(rows, pgx.RowToStructByPos[transferParty])
	if err != nil {
		return fmt.Errorf("users: lock transfer accounts: %w", err)
	}
	from := slices.IndexFunc(parties, func(p transferParty) bool { return p.ID == in.FromID })
	if from < 0 {
		return domain.ErrUserNotFound
	}
	if parties[from].RoleID != in.FromRoleID {
		return domain.ErrRoleChanged
	}
	if !parties[from].Disabled {
		return domain.ErrUserNotDisabled
	}
	to := slices.IndexFunc(parties, func(p transferParty) bool { return p.ID == in.ToID })
	if in.ToID == in.FromID || to < 0 || parties[to].Disabled {
		return domain.ErrInvalidRecipient
	}
	return l.lockTeacherRole(ctx, parties[to].RoleID)
}

func (l *TransferLedger) lockTeacherRole(ctx context.Context, roleID string) error {
	var admin bool
	var keys []string
	err := l.QueryRow(ctx, `
		SELECT coalesce(r.builtin_key, '') = 'admin',
		       coalesce(array_agg(rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')
		  FROM (SELECT r.id, r.builtin_key FROM app.roles r WHERE r.id = $1::uuid FOR SHARE) r
		  LEFT JOIN app.role_permissions rp ON rp.role_id = r.id
		 GROUP BY r.builtin_key`, roleID).Scan(&admin, &keys)
	if err != nil {
		return fmt.Errorf("users: lock recipient role: %w", err)
	}
	granted := access.All()
	if !admin {
		granted = make([]access.Key, len(keys))
		for i, key := range keys {
			granted[i] = access.Key(key)
		}
	}
	if !slices.Contains(access.Workspaces(access.NewSet(granted...)), access.TeacherWorkspace) {
		return domain.ErrInvalidRecipient
	}
	return nil
}

type transferDiff struct {
	Owner          ownerDiff `json:"owner"`
	To             string    `json:"to"`
	Classes        int       `json:"classes"`
	Tests          int       `json:"tests"`
	Questions      int       `json:"questions"`
	QuestionGroups int       `json:"questionGroups"`
	Media          int       `json:"media"`
	Assignments    int       `json:"assignments"`
	ImportsLeft    int       `json:"importsLeft"`
}

type ownerDiff struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// RecordTransfer implements domain.TransferLedger.
func (l *TransferLedger) RecordTransfer(ctx context.Context, req domain.WriteRequest, in domain.ContentTransfer, moved domain.MovedContent) (domain.LeftBehind, error) {
	var left domain.LeftBehind
	if err := l.QueryRow(ctx, `
		SELECT count(*)::int FROM app.word_imports w
		 WHERE w.created_by = $1::uuid AND w.status NOT IN ('committed', 'cancelled')`, in.FromID).Scan(&left.Imports); err != nil {
		return domain.LeftBehind{}, fmt.Errorf("users: count imports left behind: %w", err)
	}
	var owner ownerDiff
	if err := l.QueryRow(ctx, `
		SELECT (SELECT u.full_name FROM app.users u WHERE u.id = $1::uuid), (SELECT u.full_name FROM app.users u WHERE u.id = $2::uuid)`,
		in.FromID, in.ToID).Scan(&owner.Old, &owner.New); err != nil {
		return domain.LeftBehind{}, fmt.Errorf("users: read transfer names: %w", err)
	}
	diff, err := json.Marshal(transferDiff{
		Owner: owner, To: in.ToID, Classes: moved.Classes, Tests: moved.Tests, Questions: moved.Questions,
		QuestionGroups: moved.QuestionGroups, Media: moved.Media, Assignments: moved.Assignments, ImportsLeft: left.Imports,
	})
	if err != nil {
		return domain.LeftBehind{}, fmt.Errorf("users: encode transfer audit: %w", err)
	}
	err = audit.Write(ctx, l, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "user.content_transferred",
		Entity:      entityUser,
		EntityID:    &in.FromID,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	})
	if err != nil {
		return domain.LeftBehind{}, err
	}
	return left, nil
}
