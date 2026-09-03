/** Client-side handling of a join code (§6.1). */

/** §6.1's alphabet, character for character. `I`, `O`, `0` and `1` are absent. */
export const ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

const ALPHABET_SET = new Set(ALPHABET);

export const CODE_LENGTH = 8;

/** Reduces whatever was typed, pasted or scanned to the canonical form. */
export function normalize(input: string): string {
  let out = "";
  for (const ch of input.toUpperCase()) {
    if (ALPHABET_SET.has(ch)) out += ch;
  }
  return out;
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
