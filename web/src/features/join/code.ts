/**
 * Client-side handling of a join code (§6.1).
 *
 * This is presentation only: the server normalizes again before hashing, and
 * the rate-limit bucket keys on ITS normalization, not this one. So a bug here
 * cannot weaken anything -- but it can break a perfectly good code by dropping
 * a character the server would have kept, which is why the alphabet is asserted
 * against the Go constant in code.test.ts rather than copied and hoped over.
 */

/** §6.1's alphabet, character for character. `I`, `O`, `0` and `1` are absent. */
export const ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

export const CODE_LENGTH = 8;

/**
 * Reduces whatever was typed, pasted or scanned to the canonical form.
 *
 * Anything outside the alphabet is dropped rather than enumerated: students
 * paste with spaces, and phone keyboards substitute a typographic dash for the
 * hyphen. Uppercasing first means a lowercase `k` survives as `K` instead of
 * being discarded as "not in the alphabet".
 */
export function normalize(input: string): string {
  return [...input.toUpperCase()].filter((ch) => ALPHABET.includes(ch)).join("");
}

/** Groups for display: `XXXX-XXXX` (§6.1). */
export function format(code: string): string {
  const canonical = normalize(code);
  if (canonical.length <= 4) return canonical;
  return `${canonical.slice(0, 4)}-${canonical.slice(4, CODE_LENGTH)}`;
}

/** Whether a code is the right shape to be worth sending. */
export function isComplete(code: string): boolean {
  return normalize(code).length === CODE_LENGTH;
}
