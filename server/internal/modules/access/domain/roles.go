package domain

import (
	"context"
	"errors"
	"slices"
	"time"

	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

// The refusals of a role command. Each is final: nothing was written.
var (
	// ErrRoleNotFound means no role has the id.
	ErrRoleNotFound = errors.New("access: unknown role")
	// ErrNameTaken means another role has the name, in any letter case.
	ErrNameTaken = errors.New("access: role name taken")
	// ErrNotAssignable means the role is the Assistant, which no picker and no
	// copy offers.
	ErrNotAssignable = errors.New("access: role not assignable")
	// ErrRoleLocked means the cell is the Admin's, which holds every key.
	ErrRoleLocked = errors.New("access: the Admin role holds every permission")
	// ErrStudentNeedsTakeTests means the change would take learning.take_tests
	// from the built-in Student.
	ErrStudentNeedsTakeTests = errors.New("access: the Student role needs learning.take_tests")
	// ErrKeyNotGrantable means the key is hidden or outside the catalogue.
	ErrKeyNotGrantable = errors.New("access: key not grantable")
	// ErrCannotActOnSelf means the change is to the caller's own role.
	ErrCannotActOnSelf = errors.New("access: a role's holder cannot change its permissions")
	// ErrTargetRoleNotAllowed means the subset rule keeps the caller from the
	// role, or from the key being granted or revoked.
	ErrTargetRoleNotAllowed = errors.New("access: role or key outside the caller's own permissions")
	// ErrStaleWrite means a role moved on since the caller read it.
	ErrStaleWrite = errors.New("access: role changed since it was read")
)

// Limits on a role's text, matching app.roles' CHECKs.
const (
	MaxNameLength        = 60
	MaxDescriptionLength = 300
)

// A new custom role's look.
const (
	NewRoleIcon  = "key-round"
	NewRoleColor = "gray"
)

// AuditEntity is the entity name of every role audit row.
const AuditEntity = "role"

// The audit actions of the role commands.
const (
	AuditRoleCreated            = "role.created"
	AuditRoleUpdated            = "role.updated"
	AuditRolePermissionsChanged = "role.permissions_changed"
)

// Caller is who runs a role command: the actor, the role they hold and what
// that role lets them do.
type Caller struct {
	actor.Actor
	RoleID      string
	Permissions access.Set
}

// GrantChange turns one role's cell for one key on or off.
type GrantChange struct {
	RoleID  string
	Key     access.Key
	Granted bool
}

// NewRole is a role to insert. The id and the revision are the database's.
type NewRole struct {
	By          Caller
	Name        string
	Description string
	CopiedFrom  string
}

// RoleEdit is the change to a role's text and look. A nil field is kept.
type RoleEdit struct {
	ID          string
	Name        *string
	Description *string
	Icon        *string
	Color       *string
}

// AuditEntry is one row of app.audit_log a role command writes in its own
// transaction. Diff is JSON.
type AuditEntry struct {
	By     Caller
	Action string
	RoleID string
	Diff   []byte
	At     time.Time
}

// RoleStore runs the role commands' writes.
type RoleStore interface {
	// InTx runs fn in one transaction, committed when it returns nil.
	InTx(ctx context.Context, fn func(ctx context.Context, tx RoleTx) error) error
}

// RoleTx is the writes of a role command inside its transaction.
type RoleTx interface {
	// Lock reads the roles with ids and their grants and locks them against
	// any other writer, in id order so two saves cannot deadlock. An unknown id
	// is left out of the answer.
	Lock(ctx context.Context, ids []string) ([]RoleRow, error)
	// LockShared reads one role and its grants and holds its grants still
	// while the transaction lasts, or returns ErrRoleNotFound.
	LockShared(ctx context.Context, id string) (RoleRow, error)
	// Insert adds a custom role with the default look and returns its id, or
	// ErrNameTaken.
	Insert(ctx context.Context, role NewRole) (string, error)
	// Update changes a role's text and look and moves its revision, or returns
	// ErrNameTaken.
	Update(ctx context.Context, edit RoleEdit) error
	// Grant adds keys to a role's grants; a key it already holds is left.
	Grant(ctx context.Context, roleID string, keys []access.Key) error
	// Revoke removes keys from a role's grants.
	Revoke(ctx context.Context, roleID string, keys []access.Key) error
	// Row reads one role as it stands in this transaction, or returns
	// ErrRoleNotFound.
	Row(ctx context.Context, id string) (RoleRow, error)
	// Audit appends one audit row.
	Audit(ctx context.Context, entry AuditEntry) error
}

// Guard holds the role commands to the subset rule: a caller acts on a role
// only when its permissions, learning.take_tests apart, are the caller's own,
// and grants or revokes only keys the caller holds.
type Guard struct {
	manager   PermissionManager
	catalogue []access.Key
}

// NewGuard returns the Guard over catalogue, the keys the Admin's wildcard
// expands to.
func NewGuard(catalogue []access.Key) Guard { return Guard{catalogue: catalogue} }

// Effective returns what the role's users may do.
func (g Guard) Effective(role RoleRow) access.Set {
	return g.manager.Effective(role.Role, role.Grants, g.catalogue)
}

// Visible returns the role's effective permissions in catalogue order without
// the hidden keys: the matrix column.
func (g Guard) Visible(role RoleRow) []access.Key {
	keys := g.Effective(role).Keys()
	return slices.DeleteFunc(keys, access.Key.Hidden)
}

// CanActOn reports whether the subset rule lets the caller act on the role.
func (g Guard) CanActOn(by Caller, role RoleRow) bool {
	return access.CanActOn(by.Permissions, g.Effective(role))
}

// Assignable returns the roles the caller may give to a user, in the order
// given: those the subset rule allows, without the Assistant.
func (g Guard) Assignable(by Caller, roles []RoleRow) []RoleRow {
	var out []RoleRow
	for _, r := range roles {
		if r.Builtin != access.BuiltinAssistant && g.CanActOn(by, r) {
			out = append(out, r)
		}
	}
	return out
}

// CheckSource refuses a role to copy from: the Assistant, or one the subset
// rule keeps from the caller.
func (g Guard) CheckSource(by Caller, source RoleRow) error {
	if source.Builtin == access.BuiltinAssistant {
		return ErrNotAssignable
	}
	if !g.CanActOn(by, source) {
		return ErrTargetRoleNotAllowed
	}
	return nil
}

// Copy returns the keys a role copied from source starts with: its visible
// permissions.
func (g Guard) Copy(source RoleRow) []access.Key { return g.Visible(source) }

// CheckEdit refuses an edit of a role's text or look: a role the subset rule
// keeps from the caller, or a revision that is not the one the caller read.
func (g Guard) CheckEdit(by Caller, role RoleRow, expected int64) error {
	if !g.CanActOn(by, role) {
		return ErrTargetRoleNotAllowed
	}
	if role.Revision != expected {
		return ErrStaleWrite
	}
	return nil
}

// CheckGrants refuses a matrix save as a whole when any change breaks a rule,
// and otherwise returns, for each role, the keys that really move. roles holds
// every role the changes touch, as locked; expected is the revision the caller
// read of each.
func (g Guard) CheckGrants(by Caller, roles map[string]RoleRow, changes []GrantChange, expected map[string]int64) (map[string]Delta, error) {
	for _, c := range changes {
		role, ok := roles[c.RoleID]
		if !ok {
			return nil, ErrRoleNotFound
		}
		if err := g.checkChange(by, role, c); err != nil {
			return nil, err
		}
	}
	for id, role := range roles {
		if revision, ok := expected[id]; !ok || revision != role.Revision {
			return nil, ErrStaleWrite
		}
	}
	delta := map[string]Delta{}
	for _, c := range changes {
		d := delta[c.RoleID]
		switch held := roles[c.RoleID].Grants.Has(c.Key); {
		case c.Granted && !held:
			d.Granted = append(d.Granted, c.Key)
		case !c.Granted && held:
			d.Revoked = append(d.Revoked, c.Key)
		}
		delta[c.RoleID] = d
	}
	return delta, nil
}

func (g Guard) checkChange(by Caller, role RoleRow, c GrantChange) error {
	switch {
	case !c.Key.Known() || c.Key.Hidden():
		return ErrKeyNotGrantable
	case role.Builtin == access.BuiltinAdmin && c.Key != access.LearningTakeTests:
		return ErrRoleLocked
	case role.Builtin == access.BuiltinStudent && c.Key == access.LearningTakeTests && !c.Granted:
		return ErrStudentNeedsTakeTests
	case role.ID == by.RoleID && (role.Builtin != access.BuiltinAdmin || c.Key != access.LearningTakeTests):
		return ErrCannotActOnSelf
	case !g.CanActOn(by, role) || !access.CanActOn(by.Permissions, access.NewSet(c.Key)):
		return ErrTargetRoleNotAllowed
	}
	return nil
}

// Delta is the keys a matrix save really adds to or removes from one role.
type Delta struct {
	Granted []access.Key
	Revoked []access.Key
}

// Empty reports whether the save changes nothing in the role.
func (d Delta) Empty() bool { return len(d.Granted) == 0 && len(d.Revoked) == 0 }
