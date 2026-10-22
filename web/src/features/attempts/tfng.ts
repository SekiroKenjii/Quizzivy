const TFNG = ["trueFalse.true", "trueFalse.false", "trueFalse.notGiven"] as const;

/**
 * tfngLabel names a true / false / not given option by its position, in the
 * reader's language, as the engine does: the server sets the three texts, so
 * the label follows the order and never the stored text.
 */
export function tfngLabel(index: number): (typeof TFNG)[number] | null {
  return TFNG[index] ?? null;
}
