import type { User } from "./api";

/** Where a signed-in user belongs (§3's route trees). */
export function homePathFor(user: Pick<User, "role"> | null | undefined): string {
  return user?.role === "admin" ? "/admin" : "/app";
}

/** Resolves the `?next=` a guard attached, falling back to the user's home. */
export function destinationAfterSignIn(
  next: string | null | undefined,
  user: Pick<User, "role"> | null | undefined,
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
