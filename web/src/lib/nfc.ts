/**
 * nfc returns text in Unicode Normalization Form C, the form the server
 * grades in and stores what a person types in; a row written before F-37 may
 * still be decomposed, so a view composes what it draws. Be Vietnam Pro's shipped subsets
 * draw the precomposed Vietnamese letters but not every combining mark
 * (U+0302, U+0306, U+031B), so decomposed text typed with Unikey's
 * "Unicode tổ hợp" or pasted from a PDF renders in a fallback font until it
 * is composed. Never pass a password or a secret through it.
 */
export function nfc(text: string): string {
  return text.normalize("NFC");
}
