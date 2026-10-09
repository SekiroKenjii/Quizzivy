const CANONICAL: Readonly<Record<string, "trueFalse.true" | "trueFalse.false">> = {
  True: "trueFalse.true",
  False: "trueFalse.false",
};

/**
 * trueFalseLabelKey is the translation key that names a true/false option in
 * the reader's language, or null when the option keeps its stored text. Only
 * the canonical plain texts the editor and the import write, exactly "True" and
 * "False", are translated; any other text, or an option with content, is shown
 * as stored. The key follows the text, not the position, because options can
 * be shuffled.
 */
export function trueFalseLabelKey(
  type: string | undefined,
  text: string,
  content: unknown,
): "trueFalse.true" | "trueFalse.false" | null {
  if (type !== "true_false" || content != null) return null;
  return Object.hasOwn(CANONICAL, text) ? CANONICAL[text]! : null;
}
