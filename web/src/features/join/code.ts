/** Client-side handling of a join code (§6.1). */

/** §6.1's alphabet, character for character. `I`, `O`, `0` and `1` are absent. */
export const ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

const ALPHABET_SET = new Set(ALPHABET);

const EXCLUDED = new Set("0O1I");

export const CODE_LENGTH = 8;

/** EXAMPLE_CODE is the field's placeholder, spelt from the alphabet. */
export const EXAMPLE_CODE = "K7QM-2PXA";

/** Reduces whatever was typed, pasted or scanned to the canonical form. */
export function normalize(input: string): string {
  let out = "";
  for (const ch of input.toUpperCase()) {
    if (ALPHABET_SET.has(ch)) out += ch;
  }
  return out;
}

/**
 * clean reduces what was typed or pasted into the code field to at most eight
 * upper-case letters and digits. Unlike `normalize` it keeps the characters the
 * alphabet excludes, so the field can say why a code cannot be right.
 */
export function clean(input: string): string {
  let out = "";
  for (const ch of input.toUpperCase()) {
    if (out.length === CODE_LENGTH) break;
    if ((ch >= "A" && ch <= "Z") || (ch >= "0" && ch <= "9")) out += ch;
  }
  return out;
}

/** hasExcluded reports whether a cleaned code holds a character no code uses. */
export function hasExcluded(code: string): boolean {
  for (const ch of code) if (EXCLUDED.has(ch)) return true;
  return false;
}

/** group shows a cleaned code as `XXXX-XXXX`, the dash appearing with the fifth character. */
export function group(code: string): string {
  if (code.length <= 4) return code;
  return `${code.slice(0, 4)}-${code.slice(4, CODE_LENGTH)}`;
}

/** Groups for display: `XXXX-XXXX` (§6.1). */
export function format(code: string): string {
  return group(normalize(code));
}

/** Whether a code is the right shape to be worth sending. */
export function isComplete(code: string): boolean {
  return normalize(code).length === CODE_LENGTH;
}
