package command

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"
)

// CreateRole adds a custom role with the default icon and colour. CopyFrom, when set, is the role whose visible permissions the new role starts with; it must pass the subset rule and not be the Assistant, else domain.ErrTargetRoleNotAllowed or domain.ErrNotAssignable, and domain.ErrRoleNotFound when it does not exist. A name another role has, in any letter case, is domain.ErrNameTaken. It writes one role.created audit row.
type CreateRole struct {
	By          domain.Caller
	Name        string
	Description string
	CopyFrom    string
}

// CreateRoleHandler runs CreateRole.
type CreateRoleHandler struct {
	Store domain.RoleStore
	*support.Service
}

// Handle creates the role and returns it as the caller sees it.
func (h CreateRoleHandler) Handle(ctx context.Context, cmd CreateRole) (model.Role, error) {
	cmd.Name, cmd.Description = tidy(cmd.Name), tidy(cmd.Description)
	var refused validation.Error
	checkName(&refused, cmd.Name)
	checkDescription(&refused, cmd.Description)
	if err := refused.OrNil(); err != nil {
		return model.Role{}, err
	}
	var id string
	err := h.Store.InTx(ctx, func(ctx context.Context, tx domain.RoleTx) (err error) {
		id, err = h.create(ctx, tx, cmd)
		return err
	})
	if err != nil {
		return model.Role{}, err
	}
	return one(ctx, h.Service, cmd.By, id)
}

func (h CreateRoleHandler) create(ctx context.Context, tx domain.RoleTx, cmd CreateRole) (string, error) {
	var keys []access.Key
	if cmd.CopyFrom != "" {
		source, err := tx.LockShared(ctx, cmd.CopyFrom)
		if err != nil {
			return "", err
		}
		if err := h.Guard().CheckSource(cmd.By, source); err != nil {
			return "", err
		}
		keys = h.Guard().Copy(source)
	}
	id, err := tx.Insert(ctx, domain.NewRole{By: cmd.By, Name: cmd.Name, Description: cmd.Description, CopiedFrom: cmd.CopyFrom})
	if err != nil {
		return "", err
	}
	if len(keys) > 0 {
		if err := tx.Grant(ctx, id, keys); err != nil {
			return "", err
		}
	}
	diff, err := json.Marshal(map[string]any{"name": cmd.Name, "copiedFrom": nilIfEmpty(cmd.CopyFrom), "permissions": keysOf(keys)})
	if err != nil {
		return "", err
	}
	return id, tx.Audit(ctx, domain.AuditEntry{By: cmd.By, Action: domain.AuditRoleCreated, RoleID: id, Diff: diff, At: h.Now()})
}

func tidy(s string) string {
	return strings.TrimSpace(norm.NFC.String(s))
}

func hidden(r rune) bool {
	return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
}

func checkName(refused *validation.Error, name string) {
	if n := utf8.RuneCountInString(name); n < 1 || n > domain.MaxNameLength {
		refused.Add("name", "must be between 1 and 60 characters")
	} else if strings.ContainsFunc(name, hidden) {
		refused.Add("name", "must not contain line breaks, control or invisible characters")
	}
}

func checkDescription(refused *validation.Error, description string) {
	if utf8.RuneCountInString(description) > domain.MaxDescriptionLength {
		refused.Add("description", "must be at most 300 characters")
	}
}
