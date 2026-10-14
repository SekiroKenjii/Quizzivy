// Package http is the access module's transport: the Roles & permissions
// operations under /admin/roles.
package http

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/command"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

// Access serves the role operations over the access application.
type Access struct {
	app *application.Application
}

// NewAccess returns the transport over app.
func NewAccess(app *application.Application) Access { return Access{app: app} }

func (h Access) caller(ctx context.Context) (domain.Caller, bool) {
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok || h.app == nil || who.ID == "" {
		return domain.Caller{}, false
	}
	principal, _ := httpx.PrincipalFromContext(ctx)
	return domain.Caller{Actor: who, RoleID: principal.Access.RoleID, Permissions: principal.Access.Permissions}, true
}

func keysOf(keys []access.Key) []openapi.PermissionKey {
	out := make([]openapi.PermissionKey, len(keys))
	for i, k := range keys {
		out[i] = openapi.PermissionKey(k)
	}
	return out
}

func builtinOf(b access.Builtin) *openapi.BuiltinRoleKey {
	if b == "" {
		return nil
	}
	return new(openapi.BuiltinRoleKey(b))
}

func roleOf(r model.Role) openapi.RoleSummary {
	return openapi.RoleSummary{
		Id: httpapi.ParseUUID(r.ID), BuiltinKey: builtinOf(r.Builtin), Name: r.Name, Description: r.Description,
		Icon: openapi.RoleIcon(r.Icon), Color: openapi.RoleColor(r.Color), Custom: r.Custom(), UserCount: r.UserCount,
		Permissions: keysOf(r.Permissions), Revision: r.Revision, Editable: r.Editable, InProgressAttempts: r.InProgressAttempts,
	}
}

func rolesOf(roles []model.Role) []openapi.RoleSummary {
	out := make([]openapi.RoleSummary, len(roles))
	for i, r := range roles {
		out[i] = roleOf(r)
	}
	return out
}

// ListRoles implements GET /admin/roles.
func (h Access) ListRoles(ctx context.Context, _ openapi.ListRolesRequestObject) (openapi.ListRolesResponseObject, error) {
	by, ok := h.caller(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	list, err := h.app.Queries.ListRoles.Handle(ctx, query.ListRoles{By: by})
	if err != nil {
		return nil, err
	}
	out := openapi.ListRoles200JSONResponse{Roles: rolesOf(list.Roles), Catalogue: make([]openapi.PermissionCatalogueEntry, len(list.Catalogue))}
	for i, c := range list.Catalogue {
		out.Catalogue[i] = openapi.PermissionCatalogueEntry{Key: openapi.PermissionKey(c.Key), Group: openapi.PermissionCatalogueEntryGroup(c.Group), AdminLocked: c.AdminLocked}
	}
	return out, nil
}

// ListAssignableRoles implements GET /admin/roles/assignable.
func (h Access) ListAssignableRoles(ctx context.Context, _ openapi.ListAssignableRolesRequestObject) (openapi.ListAssignableRolesResponseObject, error) {
	by, ok := h.caller(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	roles, err := h.app.Queries.AssignableRoles.Handle(ctx, query.AssignableRoles{By: by})
	if err != nil {
		return nil, err
	}
	out := make(openapi.ListAssignableRoles200JSONResponse, len(roles))
	for i, r := range roles {
		out[i] = openapi.AssignableRole{
			Id: httpapi.ParseUUID(r.ID), BuiltinKey: builtinOf(r.Builtin), Name: r.Name,
			Icon: openapi.RoleIcon(r.Icon), Color: openapi.RoleColor(r.Color), StudentTarget: r.StudentTarget,
		}
	}
	return out, nil
}

// CreateRole implements POST /admin/roles.
func (h Access) CreateRole(ctx context.Context, request openapi.CreateRoleRequestObject) (openapi.CreateRoleResponseObject, error) {
	by, ok := h.caller(ctx)
	if !ok || request.Body == nil || h.app.Commands.CreateRole == nil {
		return nil, httpx.ErrNotImplemented
	}
	cmd := command.CreateRole{By: by, Name: request.Body.Name, Description: httpapi.DerefString(request.Body.Description)}
	if request.Body.CopyFromRoleId != nil {
		cmd.CopyFrom = request.Body.CopyFromRoleId.String()
	}
	role, err := h.app.Commands.CreateRole.Handle(ctx, cmd)
	if err != nil {
		return failed[openapi.CreateRoleResponseObject](ctx, err)
	}
	return openapi.CreateRole201JSONResponse(roleOf(role)), nil
}

// UpdateRole implements PATCH /admin/roles/{id}.
func (h Access) UpdateRole(ctx context.Context, request openapi.UpdateRoleRequestObject) (openapi.UpdateRoleResponseObject, error) {
	by, ok := h.caller(ctx)
	if !ok || request.Body == nil || h.app.Commands.UpdateRole == nil {
		return nil, httpx.ErrNotImplemented
	}
	cmd := command.UpdateRole{By: by, ID: request.Id.String(), Expected: request.Body.ExpectedRevision, Name: request.Body.Name, Description: request.Body.Description}
	if v := request.Body.Icon; v != nil {
		cmd.Icon = new(string(*v))
	}
	if v := request.Body.Color; v != nil {
		cmd.Color = new(string(*v))
	}
	role, err := h.app.Commands.UpdateRole.Handle(ctx, cmd)
	if err != nil {
		return failed[openapi.UpdateRoleResponseObject](ctx, err)
	}
	return openapi.UpdateRole200JSONResponse(roleOf(role)), nil
}

// SetRolePermissions implements PUT /admin/roles/permissions.
func (h Access) SetRolePermissions(ctx context.Context, request openapi.SetRolePermissionsRequestObject) (openapi.SetRolePermissionsResponseObject, error) {
	by, ok := h.caller(ctx)
	if !ok || request.Body == nil || h.app.Commands.SetRolePermissions == nil {
		return nil, httpx.ErrNotImplemented
	}
	cmd := command.SetRolePermissions{By: by}
	for _, c := range request.Body.Changes {
		cmd.Changes = append(cmd.Changes, domain.GrantChange{RoleID: c.RoleId.String(), Key: access.Key(c.Key), Granted: c.Granted})
	}
	for _, e := range request.Body.ExpectedRevisions {
		cmd.Expected = append(cmd.Expected, command.RoleRevision{RoleID: e.RoleId.String(), Revision: e.Revision})
	}
	roles, err := h.app.Commands.SetRolePermissions.Handle(ctx, cmd)
	if err != nil {
		return failed[openapi.SetRolePermissionsResponseObject](ctx, err)
	}
	return openapi.SetRolePermissions200JSONResponse{Roles: rolesOf(roles)}, nil
}

func failed[R any](ctx context.Context, err error) (R, error) {
	var none R
	f := failureOf(ctx, err)
	if f == nil {
		return none, err
	}
	out, ok := any(f).(R)
	if !ok {
		return none, errors.New("access: a refusal does not answer this operation")
	}
	return out, nil
}
