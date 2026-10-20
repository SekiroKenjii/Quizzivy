package command

import (
	"context"
	"encoding/json"
	"slices"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/validation"
)

// MaxGrantChanges is the most cells one matrix save may change.
const MaxGrantChanges = 200

// RoleRevision is the revision of a role as the caller last read it.
type RoleRevision struct {
	RoleID   string
	Revision int64
}

// SetRolePermissions turns cells of the matrix on and off in one transaction, and refuses the whole save when any change breaks a rule: domain.ErrRoleNotFound, domain.ErrKeyNotGrantable, domain.ErrRoleLocked, domain.ErrStudentNeedsTakeTests, domain.ErrCannotActOnSelf, domain.ErrTargetRoleNotAllowed or, for a role past its expected revision or with none named, domain.ErrStaleWrite. A change that leaves a cell as it was is accepted and writes nothing. It writes one role.permissions_changed audit row per role that changed, and forgets every cached principal on this machine.
type SetRolePermissions struct {
	By       domain.Caller
	Changes  []domain.GrantChange
	Expected []RoleRevision
}

// SetRolePermissionsHandler runs SetRolePermissions.
type SetRolePermissionsHandler struct {
	Store domain.RoleStore
	*support.Service
}

type save struct {
	By       domain.Caller
	Changes  []domain.GrantChange
	IDs      []string
	Expected map[string]int64
}

// Handle saves the changes and returns the roles they touched, as the caller sees them.
func (h SetRolePermissionsHandler) Handle(ctx context.Context, cmd SetRolePermissions) ([]model.Role, error) {
	s, err := normalise(cmd)
	if err != nil {
		return nil, err
	}
	changed := false
	err = h.Store.InTx(ctx, func(ctx context.Context, tx domain.RoleTx) (err error) {
		changed, err = h.apply(ctx, tx, s)
		return err
	})
	if err != nil {
		return nil, err
	}
	if changed {
		h.ForgetAll()
	}
	return h.Views(ctx, cmd.By, s.IDs)
}

func (h SetRolePermissionsHandler) apply(ctx context.Context, tx domain.RoleTx, s save) (bool, error) {
	locked, err := tx.Lock(ctx, s.IDs)
	if err != nil {
		return false, err
	}
	roles := make(map[string]domain.RoleRow, len(locked))
	for _, r := range locked {
		roles[r.ID] = r
	}
	delta, err := h.Guard().CheckGrants(s.By, roles, s.Changes, s.Expected)
	if err != nil {
		return false, err
	}
	changed := false
	for _, id := range s.IDs {
		if delta[id].Empty() {
			continue
		}
		if err := h.write(ctx, tx, s.By, id, delta[id]); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func (h SetRolePermissionsHandler) write(ctx context.Context, tx domain.RoleTx, by domain.Caller, id string, d domain.Delta) error {
	if err := tx.Grant(ctx, id, d.Granted); err != nil {
		return err
	}
	if err := tx.Revoke(ctx, id, d.Revoked); err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"granted": keysOf(d.Granted), "revoked": keysOf(d.Revoked)})
	if err != nil {
		return err
	}
	return tx.Audit(ctx, domain.AuditEntry{By: by, Action: domain.AuditRolePermissionsChanged, RoleID: id, Diff: raw, At: h.Now()})
}

func normalise(cmd SetRolePermissions) (save, error) {
	var refused validation.Error
	if n := len(cmd.Changes); n < 1 || n > MaxGrantChanges {
		refused.Add("changes", "must hold between 1 and 200 changes")
	}
	changes, ids := collapse(&refused, cmd.Changes)
	expected := make(map[string]int64, len(cmd.Expected))
	for _, e := range cmd.Expected {
		if known, again := expected[e.RoleID]; again && known != e.Revision {
			refused.Add("expectedRevisions", "a role has two revisions")
		}
		expected[e.RoleID] = e.Revision
	}
	if err := refused.OrNil(); err != nil {
		return save{}, err
	}
	slices.Sort(ids)
	return save{By: cmd.By, Changes: changes, IDs: ids, Expected: expected}, nil
}

func collapse(refused *validation.Error, in []domain.GrantChange) ([]domain.GrantChange, []string) {
	type cell struct{ role, key string }
	seen := map[cell]bool{}
	var changes []domain.GrantChange
	var ids []string
	for _, c := range in {
		at := cell{c.RoleID, string(c.Key)}
		if granted, again := seen[at]; again {
			if granted != c.Granted {
				refused.Add("changes", "a cell is both granted and revoked")
			}
			continue
		}
		seen[at] = c.Granted
		changes = append(changes, c)
		if !slices.Contains(ids, c.RoleID) {
			ids = append(ids, c.RoleID)
		}
	}
	return changes, ids
}
