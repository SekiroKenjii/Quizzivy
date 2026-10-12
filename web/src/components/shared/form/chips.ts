import { useState, type KeyboardEvent } from "react";

import { fold } from "@/lib/fold";

/** ChipSame says whether two chip values count as one entry. */
export type ChipSame = (a: string, b: string) => boolean;

/**
 * sameIgnoringCase is the deck's rule for an entry that is already there: the
 * two are equal once lower-cased. Accents count, so "nghe" and "nghé" are two
 * entries.
 */
export function sameIgnoringCase(a: string, b: string): boolean {
  return a.toLowerCase() === b.toLowerCase();
}

/**
 * sameFolded is the rule for tags: the two are equal under `fold`, which
 * ignores case and accents, so "nghe" and "Nghé" are one entry.
 */
export function sameFolded(a: string, b: string): boolean {
  return fold(a) === fold(b);
}

/** chipText is an entry as it is stored: trimmed, in NFC. */
export function chipText(entry: string): string {
  return entry.trim().normalize("NFC");
}

/**
 * withChip returns `values` with `entry` added at the end as `chipText` has
 * it, or null when the entry is blank or is `same` as a value already there.
 */
export function withChip(
  values: readonly string[],
  entry: string,
  same: ChipSame,
): string[] | null {
  const chip = chipText(entry);
  if (chip === "" || values.some((value) => same(value, chip))) return null;
  return [...values, chip];
}

/**
 * useChipEntry holds the draft of a chip field and its keys. `type` takes
 * what the input now holds and, with `commaAdds`, adds every part before a
 * comma; `commit` adds the draft and empties it, for Enter and for blur;
 * `onKeyDown` commits on Enter and, on Backspace in an empty input, removes
 * the last value, once per press. `resolve` maps an entry to the value to
 * add, for a field that prefers a known spelling. Nothing is added twice by
 * `same`.
 */
export function useChipEntry({
  values,
  onChange,
  same,
  commaAdds,
  resolve,
}: Readonly<{
  values: readonly string[];
  onChange: (next: string[]) => void;
  same: ChipSame;
  commaAdds: boolean;
  resolve?: ((entry: string) => string) | undefined;
}>) {
  const [draft, setDraft] = useState("");

  function added(list: readonly string[], entry: string) {
    const chip = chipText(entry);
    const known = chip === "" || resolve === undefined ? chip : resolve(chip);
    return withChip(list, known, same);
  }

  function type(text: string) {
    if (!commaAdds || !text.includes(",")) {
      setDraft(text);
      return;
    }
    const parts = text.split(",");
    const rest = parts.pop() ?? "";
    let next: readonly string[] = values;
    for (const part of parts) next = added(next, part) ?? next;
    if (next !== values) onChange([...next]);
    setDraft(rest);
  }

  function commit() {
    const next = added(values, draft);
    if (next !== null) onChange(next);
    setDraft("");
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter") {
      if (event.nativeEvent.isComposing) return;
      event.preventDefault();
      commit();
    } else if (
      event.key === "Backspace" &&
      !event.repeat &&
      event.currentTarget.value === "" &&
      values.length > 0
    ) {
      onChange(values.slice(0, -1));
    }
  }

  return { draft, setDraft, type, commit, onKeyDown };
}
