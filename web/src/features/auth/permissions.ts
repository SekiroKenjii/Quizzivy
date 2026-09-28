import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";

type User = components["schemas"]["CurrentUser"];

/** PermissionKey is one row of the permission catalogue (70 §4.1). */
export type PermissionKey = components["schemas"]["PermissionKey"];

/** Workspace is a console the signed-in user may enter. */
export type Workspace = components["schemas"]["Workspace"];

const grants = new WeakMap<Pick<User, "permissions">, ReadonlySet<PermissionKey>>();

function grantsOf(user: Pick<User, "permissions">): ReadonlySet<PermissionKey> {
  let set = grants.get(user);
  if (!set) {
    set = new Set(user.permissions);
    grants.set(user, set);
  }
  return set;
}

/**
 * can reports whether the user's role holds key. The set it reads is built
 * once per user object, so a check in a render path costs one lookup.
 */
export function can(
  user: Pick<User, "permissions"> | null | undefined,
  key: PermissionKey,
): boolean {
  return user ? grantsOf(user).has(key) : false;
}

/** hasWorkspace reports whether the user may enter workspace. */
export function hasWorkspace(
  user: Pick<User, "workspaces"> | null | undefined,
  workspace: Workspace,
): boolean {
  return user?.workspaces.includes(workspace) ?? false;
}

/**
 * learnsOnly reports whether the student app is the user's only workspace,
 * which is how the server's strict student targets see a user: a role that
 * can take tests and do nothing else.
 */
export function learnsOnly(user: Pick<User, "workspaces"> | null | undefined): boolean {
  return user?.workspaces.length === 1 && user.workspaces[0] === "app";
}

/** useCan subscribes to whether the signed-in user's role holds key. */
export function useCan(key: PermissionKey): boolean {
  return useAuthStore((s) => can(s.user, key));
}

/** useWorkspace subscribes to whether the signed-in user may enter workspace. */
export function useWorkspace(workspace: Workspace): boolean {
  return useAuthStore((s) => hasWorkspace(s.user, workspace));
}
