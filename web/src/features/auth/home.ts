import type { User } from "./api";

/**
 * Where a signed-in user belongs (§3's route trees).
 *
 * Every post-sign-in navigation goes through this. Sending people to "/"
 * instead does not work: the index route redirects anonymous visitors to
 * /login, so a fresh sign-in that navigates to "/" lands straight back on the
 * form it just completed.
 */
export function homePathFor(user: Pick<User, "role"> | null | undefined): string {
  return user?.role === "admin" ? "/admin" : "/app";
}

/**
 * Resolves the `?next=` a guard attached, falling back to the user's home.
 *
 * `next` reaches us through the URL, so it is untrusted even though we put it
 * there: a same-origin PATH only. Without the check, `?next=https://evil.test`
 * turns sign-in into an open redirect, and `//evil.test` is a protocol-relative
 * URL that `startsWith("/")` alone would happily accept.
 */
export function destinationAfterSignIn(
  next: string | null | undefined,
  user: Pick<User, "role"> | null | undefined,
): string {
  if (next && next.startsWith("/") && !next.startsWith("//")) return next;
  return homePathFor(user);
}
