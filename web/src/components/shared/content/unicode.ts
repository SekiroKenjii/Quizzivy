/** contentStringLength counts Unicode scalar values; malformed surrogates and NUL exceed every content budget. */
export function contentStringLength(value: string): number {
  let length = 0;
  for (const character of value) {
    const point = character.codePointAt(0)!;
    if (point === 0 || (point >= 0xd800 && point <= 0xdfff))
      return Number.POSITIVE_INFINITY;
    length++;
  }
  return length;
}
