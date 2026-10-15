import { ADMIN_HOME } from "@/app/adminPaths";
import type { User } from "./api";
import { hasWorkspace } from "./permissions";

/** HomeUser is what `homePathFor` reads of the signed-in user. */
export type HomeUser = Pick<User, "workspaces"> &
  Partial<Pick<User, "permissions" | "preferences">>;

/**
 * Where a signed-in user belongs (§3's route trees). A built-in Admin, the
 * one holder of `scope.all`, opens the console they last switched to
 * (`preferences.workspace`) and the Admin console the first time. Anyone else
 * with a teacher workspace opens it, a user with only the admin workspace
 * opens the Admin console, and a student the student app.
 */
export function homePathFor(user: HomeUser | null | undefined): string {
  const teacher = hasWorkspace(user, "teacher");
  const admin = hasWorkspace(user, "admin");
  if (teacher && admin && user?.permissions?.includes("scope.all"))
    return user.preferences?.workspace === "teacher" ? "/teacher" : ADMIN_HOME;
  if (teacher) return "/teacher";
  return admin ? ADMIN_HOME : "/app";
}

/** Resolves the `?next=` a guard attached, falling back to the user's home. */
export function destinationAfterSignIn(
  next: string | null | undefined,
  user: HomeUser | null | undefined,
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
