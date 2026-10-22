const NUMERALS: ReadonlyArray<readonly [number, string]> = [
  [1000, "m"],
  [900, "cm"],
  [500, "d"],
  [400, "cd"],
  [100, "c"],
  [90, "xc"],
  [50, "l"],
  [40, "xl"],
  [10, "x"],
  [9, "ix"],
  [5, "v"],
  [4, "iv"],
  [1, "i"],
];

/**
 * matchOptionKey is a matching choice's key, by the server's rule (D-063):
 * its ordinal plus one as a lower-case roman numeral, so 0 is "i" and 11 is
 * "xii". The editor uses it for rows not saved yet; a saved row shows the
 * `key` the server sent. A negative ordinal has no key.
 */
export function matchOptionKey(ordinal: number): string {
  if (ordinal < 0) return "";
  let n = ordinal + 1;
  let key = "";
  for (const [value, symbol] of NUMERALS) {
    while (n >= value) {
      key += symbol;
      n -= value;
    }
  }
  return key;
}

/**
 * matchItemKey is a matching item's key, by the server's rule (D-063): its
 * ordinal as capital letters, so 0 is "A", 19 is "T" and 26 is "AA". A
 * negative ordinal has no key.
 */
export function matchItemKey(ordinal: number): string {
  if (ordinal < 0) return "";
  let key = "";
  for (let n = ordinal + 1; n > 0; n = Math.floor((n - 1) / 26))
    key = String.fromCharCode(65 + ((n - 1) % 26)) + key;
  return key;
}
