import type { User } from "./api";
import { hasWorkspace } from "./permissions";

/**
 * Where a signed-in user belongs (§3's route trees): the teacher's tree for
 * the teacher or admin workspace, which share it until R5, and the student
 * app otherwise.
 */
export function homePathFor(user: Pick<User, "workspaces"> | null | undefined): string {
  return hasWorkspace(user, "teacher") || hasWorkspace(user, "admin")
    ? "/teacher"
    : "/app";
}

/** Resolves the `?next=` a guard attached, falling back to the user's home. */
export function destinationAfterSignIn(
  next: string | null | undefined,
  user: Pick<User, "workspaces"> | null | undefined,
): string {
  if (next && next.startsWith("/") && !next.startsWith("//")) return next;
  return homePathFor(user);
}

/**
 * Starts fetching the student home's chunks while the authorization code is
 * still out being exchanged.
 */
export function preloadStudentHome(): void {
  void import("@/layouts/StudentLayout").catch(() => undefined);
  void import("@/features/assignments/pages/StudentHomePage").catch(() => undefined);
}
