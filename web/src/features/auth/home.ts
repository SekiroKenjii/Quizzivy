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

/**
 * Starts fetching the student home's chunks while the authorization code is
 * still out being exchanged.
 *
 * Rendering /app needs two lazy chunks -- the layout, then the page inside
 * it -- and they are nested, so a cold entry fetches them one after the other.
 * A student arriving from a QR code waits for both before the first screen
 * they ever see. Overlapping that with the round trip is the only thing that
 * makes the wait shorter rather than merely tolerated; the alternative,
 * folding a boundary into the entry chunk, hands student code to every
 * anonymous visitor, which §2 forbids and router-chunks.test.ts enforces.
 *
 * Fire-and-forget. A failed prefetch changes nothing: the router's own lazy
 * import fetches again.
 */
export function preloadStudentHome(): void {
  void import("@/layouts/StudentLayout").catch(() => undefined);
  void import("@/features/assignments/pages/StudentHomePage").catch(() => undefined);
}
