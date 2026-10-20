package command

import (
	"context"
	"encoding/json"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/validation"
)

// UpdateRole changes a role's name, description, icon or colour; a nil field is kept. The role must pass the subset rule, else domain.ErrTargetRoleNotAllowed, and Expected must be its revision, else domain.ErrStaleWrite. A built-in role may be renamed. A name another role has, in any letter case, is domain.ErrNameTaken. A change that alters nothing writes nothing; any other moves the revision and writes one role.updated audit row naming what changed, and the command forgets every cached principal on this machine.
type UpdateRole struct {
	By          domain.Caller
	ID          string
	Expected    int64
	Name        *string
	Description *string
	Icon        *string
	Color       *string
}

// UpdateRoleHandler runs UpdateRole.
type UpdateRoleHandler struct {
	Store domain.RoleStore
	*support.Service
}

// Handle changes the role and returns it as the caller sees it.
func (h UpdateRoleHandler) Handle(ctx context.Context, cmd UpdateRole) (model.Role, error) {
	cmd.Name, cmd.Description = trimmed(cmd.Name), trimmed(cmd.Description)
	if err := cmd.validate(); err != nil {
		return model.Role{}, err
	}
	changed := false
	err := h.Store.InTx(ctx, func(ctx context.Context, tx domain.RoleTx) (err error) {
		changed, err = h.apply(ctx, tx, cmd)
		return err
	})
	if err != nil {
		return model.Role{}, err
	}
	if changed {
		h.ForgetAll()
	}
	return one(ctx, h.Service, cmd.By, cmd.ID)
}

func (cmd UpdateRole) validate() error {
	var refused validation.Error
	if cmd.Name == nil && cmd.Description == nil && cmd.Icon == nil && cmd.Color == nil {
		refused.Add("name", "change at least one field")
	}
	if cmd.Name != nil {
		checkName(&refused, *cmd.Name)
	}
	if cmd.Description != nil {
		checkDescription(&refused, *cmd.Description)
	}
	if err := refused.OrNil(); err != nil {
		return err
	}
	return nil
}

func (h UpdateRoleHandler) apply(ctx context.Context, tx domain.RoleTx, cmd UpdateRole) (bool, error) {
	locked, err := tx.Lock(ctx, []string{cmd.ID})
	if err != nil {
		return false, err
	}
	if len(locked) == 0 {
		return false, domain.ErrRoleNotFound
	}
	role := locked[0]
	if err := h.Guard().CheckEdit(cmd.By, role, cmd.Expected); err != nil {
		return false, err
	}
	edit, diff := cmd.against(role)
	if len(diff) == 0 {
		return false, nil
	}
	if err := tx.Update(ctx, edit); err != nil {
		return false, err
	}
	raw, err := json.Marshal(map[string]any{"changed": diff})
	if err != nil {
		return false, err
	}
	return true, tx.Audit(ctx, domain.AuditEntry{By: cmd.By, Action: domain.AuditRoleUpdated, RoleID: role.ID, Diff: raw, At: h.Now()})
}

func (cmd UpdateRole) against(role domain.RoleRow) (domain.RoleEdit, map[string]map[string]string) {
	edit := domain.RoleEdit{ID: role.ID}
	diff := map[string]map[string]string{}
	for _, f := range []struct {
		field   string
		current string
		next    *string
		set     **string
	}{
		{"name", role.Name, cmd.Name, &edit.Name},
		{"description", role.Description, cmd.Description, &edit.Description},
		{"icon", role.Icon, cmd.Icon, &edit.Icon},
		{"color", role.Color, cmd.Color, &edit.Color},
	} {
		if f.next != nil && *f.next != f.current {
			*f.set = f.next
			diff[f.field] = map[string]string{"from": f.current, "to": *f.next}
		}
	}
	return edit, diff
}

func trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	return new(validation.Tidy(*s))
}
