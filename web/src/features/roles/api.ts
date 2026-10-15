import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type RoleSummary = components["schemas"]["RoleSummary"];
export type RoleList = components["schemas"]["RoleList"];
export type CatalogueEntry = components["schemas"]["PermissionCatalogueEntry"];
export type RoleIcon = components["schemas"]["RoleIcon"];
export type RoleColor = components["schemas"]["RoleColor"];
export type RolePermissionChange = components["schemas"]["RolePermissionChange"];

export const ROLES_KEY = ["admin", "roles"] as const;

/**
 * rolesQuery reads `GET /admin/roles`: every role with its permissions, head
 * count and revision, and the catalogue of matrix rows.
 */
export function rolesQuery() {
  return queryOptions({
    queryKey: ROLES_KEY,
    queryFn: ({ signal }) => api("get", "/admin/roles", { signal }),
  });
}

/** saveRolePermissions sends one matrix save, refused whole when any change breaks a rule. */
export function saveRolePermissions(
  changes: readonly RolePermissionChange[],
  expectedRevisions: readonly components["schemas"]["RoleRevision"][],
) {
  return api("put", "/admin/roles/permissions", {
    body: { changes: [...changes], expectedRevisions: [...expectedRevisions] },
  });
}

/** NewRole is the body `createRole` sends. */
export type NewRole = components["schemas"]["RoleInput"];

export function createRole(body: NewRole) {
  return api("post", "/admin/roles", { body });
}

/** RoleEdit is the body `updateRole` sends. */
export type RoleEdit = components["schemas"]["RoleUpdate"];

export function updateRole(id: string, body: RoleEdit) {
  return api("patch", "/admin/roles/{id}", { path: { id }, body });
}

/**
 * withRoles returns `list` with each role of `touched` replacing the one of the
 * same id, and a role it did not hold appended, so a save's answer updates the
 * matrix without a second read.
 */
export function withRoles(list: RoleList, touched: readonly RoleSummary[]): RoleList {
  const byId = new Map(touched.map((role) => [role.id, role]));
  const known = new Set(list.roles.map((role) => role.id));
  return {
    ...list,
    roles: [
      ...list.roles.map((role) => byId.get(role.id) ?? role),
      ...touched.filter((role) => !known.has(role.id)),
    ],
  };
}
