import { isPlainOptionContent } from "./optionContent";

const CANONICAL: Readonly<Record<string, "trueFalse.true" | "trueFalse.false">> = {
  True: "trueFalse.true",
  False: "trueFalse.false",
};

/**
 * trueFalseLabelKey is the translation key that names a true/false option in
 * the reader's language, or null when the option keeps its stored text. Only
 * the canonical texts the editor and the import write, exactly "True" and
 * "False", stored plain or as one paragraph of unmarked text, are translated;
 * any other text, or content with marks, is shown as stored. The key follows
 * the text, not the position, because options can be shuffled.
 */
export function trueFalseLabelKey(
  type: string | undefined,
  text: string,
  content: unknown,
): "trueFalse.true" | "trueFalse.false" | null {
  if (type !== "true_false") return null;
  if (content != null && !isPlainOptionContent(content, text)) return null;
  return Object.hasOwn(CANONICAL, text) ? CANONICAL[text]! : null;
}
