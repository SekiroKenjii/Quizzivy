/**
 * `{{1}}`, `{{2}}` — the fill_blank markers, 1-indexed, matching
 * `blanks[].ordinal`. Resolved in docs/plan/40-open-items.md.
 */
const PLACEHOLDER = /\{\{(\d+)\}\}/g;

/** BLANK_GAP is how a blank is drawn in a prompt shown as one line of plain text. */
export const BLANK_GAP = "___";

/** blanksAsGaps draws every `{{n}}` marker in `prompt` as `BLANK_GAP`. */
export function blanksAsGaps(prompt: string): string {
  return prompt.replace(PLACEHOLDER, BLANK_GAP);
}

/** The distinct ordinals a prompt refers to, sorted. */
export function promptPlaceholders(prompt: string): number[] {
  const seen = new Set<number>();
  for (const match of prompt.matchAll(PLACEHOLDER)) {
    const ordinal = Number(match[1]);
    if (Number.isInteger(ordinal) && ordinal >= 1) seen.add(ordinal);
  }
  return [...seen].sort((a, b) => a - b);
}

export interface PlaceholderMismatch {
  /** In the prompt with no blank behind them: they render as literal text. */
  missingBlanks: number[];
  /** Blanks with no placeholder: unreachable from the text that addresses them. */
  unreferencedBlanks: number[];
}

/**
 * Compares the prompt's placeholders with the blank ordinals, in both
 * directions. The server enforces the same rule at save and again at publish;
 * this is what shows it inline before either.
 */
export function comparePlaceholders(
  prompt: string,
  blankOrdinals: number[],
): PlaceholderMismatch {
  const inPrompt = new Set(promptPlaceholders(prompt));
  const inBlanks = new Set(blankOrdinals);

  return {
    missingBlanks: [...inPrompt].filter((n) => !inBlanks.has(n)).sort((a, b) => a - b),
    unreferencedBlanks: [...inBlanks]
      .filter((n) => !inPrompt.has(n))
      .sort((a, b) => a - b),
  };
}

export function hasMismatch(mismatch: PlaceholderMismatch): boolean {
  return mismatch.missingBlanks.length > 0 || mismatch.unreferencedBlanks.length > 0;
}
