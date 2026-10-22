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

const TFNG = ["trueFalse.true", "trueFalse.false", "trueFalse.notGiven"] as const;

/**
 * tfngLabelKey is the translation key that names the option at `index` of a
 * true / false / not given question. The three options keep their authored
 * order and the server sets their texts, so the label follows the position,
 * never the stored text; a position past the third has none.
 */
export function tfngLabelKey(index: number): (typeof TFNG)[number] | null {
  return TFNG[index] ?? null;
}
