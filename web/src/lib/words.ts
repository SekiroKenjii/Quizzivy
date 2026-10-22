const WORD = /[\p{L}\p{N}]/u;

/**
 * countWords counts the words of an answer as the engine's word limit and the
 * grading screen both count them: whitespace-separated tokens that hold at
 * least one letter or digit. A hyphenated word ("well-known") or a number
 * ("3.5", "1,000") is one token, so it counts once; a dash or a bullet
 * standing alone between spaces is not a word. The count never cuts text.
 */
export function countWords(text: string): number {
  let count = 0;
  for (const token of text.split(/\s+/u)) {
    if (WORD.test(token)) count += 1;
  }
  return count;
}
