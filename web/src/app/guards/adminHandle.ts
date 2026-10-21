import type { Requirement } from "@/features/auth/permissions";
import type { TeacherHandle } from "@/layouts/shell/handle";

/**
 * AdminHandle is the handle of a route of the Admin console: a teacher
 * handle, rendered in the same page frame (DG-205), plus the `permission` a
 * user must hold to open the route, which `RequirePermission` enforces.
 */
export interface AdminHandle extends TeacherHandle {
  permission: Requirement;
}

/**
 * adminHandleOf returns `handle` when it is an Admin handle, an object with a
 * `crumb` trail that names a permission, and null for anything else. It lives
 * beside the guard, outside the shell, so the entry chunk loads none of the
 * shell for it.
 */
export function adminHandleOf(handle: unknown): AdminHandle | null {
  if (typeof handle !== "object" || handle === null) return null;
  if (!("crumb" in handle) || !Array.isArray(handle.crumb)) return null;
  if (!("permission" in handle) || !handle.permission) return null;
  return handle as AdminHandle;
}
