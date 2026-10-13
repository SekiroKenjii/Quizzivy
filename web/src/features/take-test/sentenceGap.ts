import { nfc } from "@/lib/nfc";
import type { StudentQuestion } from "./api";

/**
 * SentenceGap is a choice prompt drawn as the deck's sentence: the text
 * before its one gap and the text after it, composed to NFC as the prompt's
 * Markdown would be.
 */
export type SentenceGap = { before: string; after: string };

const TYPES: ReadonlySet<StudentQuestion["type"]> = new Set([
  "single_choice",
  "true_false",
]);
const RUN = /_{3,}/g;
const LINE_BREAK = /[\r\n]/;
const INLINE_MARKER = /[*_`~[\]<>\\]|&(?:#\d+|#x[\da-f]+|[a-z][\da-z]*);/i;
const BLOCK_MARKER = /^(?:#{1,6}(?:\s|$)|[-+]\s|\d{1,9}[.)](?:\s|$))/;

/**
 * sentenceGap reads the gap a choice question's prompt draws, or null when
 * the prompt prints as written. A gap is a single-choice or true/false
 * question whose prompt has no rich content and is one line holding exactly
 * one run of three or more underscores and no other Markdown marker: no
 * other underscore, no emphasis, code, strikethrough, link, HTML, escape
 * or entity, and no heading or list at its start. The stored prompt
 * is never changed; only its drawing is.
 */
export function sentenceGap(
  question: Pick<StudentQuestion, "type" | "prompt" | "promptContent">,
): SentenceGap | null {
  if (!TYPES.has(question.type) || question.promptContent != null) return null;
  const text = nfc(question.prompt).trim();
  if (LINE_BREAK.test(text) || BLOCK_MARKER.test(text)) return null;
  const runs = [...text.matchAll(RUN)];
  if (runs.length !== 1) return null;
  const run = runs[0]!;
  const before = text.slice(0, run.index);
  const after = text.slice(run.index + run[0].length);
  if (INLINE_MARKER.test(before) || INLINE_MARKER.test(after)) return null;
  return { before, after };
}
