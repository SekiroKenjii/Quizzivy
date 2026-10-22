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

const TFNG_TEXTS: Readonly<Record<string, (typeof TFNG)[number]>> = {
  True: "trueFalse.true",
  False: "trueFalse.false",
  "Not given": "trueFalse.notGiven",
};

/**
 * tfngTextLabelKey names a true / false / not given option by its stored text,
 * for a reader that shows the options out of their order, such as the chosen
 * one on a result. The server writes the three texts, so they are always the
 * canonical ones; any other text has none.
 */
export function tfngTextLabelKey(
  type: string | undefined,
  text: string,
): (typeof TFNG)[number] | null {
  if (type !== "tfng") return null;
  return Object.hasOwn(TFNG_TEXTS, text) ? TFNG_TEXTS[text]! : null;
}
