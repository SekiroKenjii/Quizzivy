import type { PermissionKey } from "@/features/auth/permissions";
import type {
  CatalogueEntry,
  RolePermissionChange,
  RoleSummary,
} from "@/features/roles/api";

/**
 * CellLock is why a cell of the matrix cannot be changed: the Admin always
 * holds the permission, the Student always needs it, it is the caller's own
 * role, or the subset rule keeps the caller from the role or from the key.
 */
export type CellLock = "admin" | "student" | "self" | "subset";

/** Caller is who the matrix is drawn for: their role's id, and the keys it holds. */
export interface Caller {
  roleId: string | null;
  holds: ReadonlySet<PermissionKey>;
}

const TAKE_TESTS: PermissionKey = "learning.take_tests";

/**
 * lockOf returns why the caller cannot change `key` of `role`, or null when
 * they can. It is the server's `checkChange` in the order the server asks:
 * the Admin's cells but Take tests, the Student's Take tests, the caller's own
 * role but the Admin's Take tests, then the subset rule, which ignores Take
 * tests.
 */
export function lockOf(
  role: RoleSummary,
  key: PermissionKey,
  caller: Caller,
): CellLock | null {
  if (role.builtinKey === "admin" && key !== TAKE_TESTS) return "admin";
  if (role.builtinKey === "student" && key === TAKE_TESTS) return "student";
  const own = role.id === caller.roleId;
  if (own && !(role.builtinKey === "admin" && key === TAKE_TESTS)) return "self";
  if (!role.editable) return "subset";
  if (key !== TAKE_TESTS && !caller.holds.has(key)) return "subset";
  return null;
}

/** Draft is the cells the Admin has turned on or off and not yet saved, by cell id. */
export type Draft = ReadonlyMap<string, boolean>;

export const NO_DRAFT: Draft = new Map();

/** cellId names one cell of the matrix. */
export function cellId(roleId: string, key: PermissionKey): string {
  return `${roleId}|${key}`;
}

const grants = new WeakMap<RoleSummary, ReadonlySet<PermissionKey>>();

/** grantsOf returns the keys a role holds, as a set built once per role object. */
export function grantsOf(role: RoleSummary): ReadonlySet<PermissionKey> {
  let set = grants.get(role);
  if (!set) {
    set = new Set(role.permissions);
    grants.set(role, set);
  }
  return set;
}

/** isOn reports whether the cell is on as the Admin sees it: the draft's answer, else the server's. */
export function isOn(role: RoleSummary, key: PermissionKey, draft: Draft): boolean {
  return draft.get(cellId(role.id, key)) ?? grantsOf(role).has(key);
}

/**
 * toggled returns `draft` with the cell turned the other way. A cell turned
 * back to what the server holds leaves the draft, so the count is of cells
 * that really differ.
 */
export function toggled(draft: Draft, role: RoleSummary, key: PermissionKey): Draft {
  const next = new Map(draft);
  const id = cellId(role.id, key);
  const wanted = !isOn(role, key, draft);
  if (wanted === grantsOf(role).has(key)) next.delete(id);
  else next.set(id, wanted);
  return next;
}

/**
 * changesOf returns the cells of `draft` that differ from the roles as they
 * stand, in catalogue order and role order, and drops any whose role has gone.
 */
export function changesOf(
  draft: Draft,
  roles: readonly RoleSummary[],
  catalogue: readonly CatalogueEntry[],
): RolePermissionChange[] {
  if (draft.size === 0) return [];
  const changes: RolePermissionChange[] = [];
  for (const role of roles) {
    for (const { key } of catalogue) {
      const granted = draft.get(cellId(role.id, key));
      if (granted !== undefined && granted !== grantsOf(role).has(key))
        changes.push({ roleId: role.id, key, granted });
    }
  }
  return changes;
}

/**
 * revisionsOf returns the revision of every role a change touches, as the
 * matrix last read it.
 */
export function revisionsOf(
  changes: readonly RolePermissionChange[],
  roles: readonly RoleSummary[],
): { roleId: string; revision: number }[] {
  const touched = new Set(changes.map((change) => change.roleId));
  return roles
    .filter((role) => touched.has(role.id))
    .map((role) => ({ roleId: role.id, revision: role.revision }));
}

/**
 * takingNow counts the people who are mid-test and hold a role that loses
 * Take tests in `changes`.
 */
export function takingNow(
  changes: readonly RolePermissionChange[],
  roles: readonly RoleSummary[],
): number {
  const losing = new Set(
    changes
      .filter((change) => change.key === TAKE_TESTS && !change.granted)
      .map((change) => change.roleId),
  );
  return roles
    .filter((role) => losing.has(role.id))
    .reduce((sum, role) => sum + role.inProgressAttempts, 0);
}
