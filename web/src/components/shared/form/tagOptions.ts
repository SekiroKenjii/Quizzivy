import { fold } from "@/lib/fold";

import { chipText } from "./chips";

/** TagSuggestion is a tag a field can offer, with a note such as how many questions carry it. */
export type TagSuggestion = { tag: string; meta?: string | undefined };

/**
 * TagOption is one row of a tag field's list: a suggestion, with the span of
 * its text that matches the draft, or the row that creates the draft as a new
 * tag, which has no span.
 */
export type TagOption = {
  tag: string;
  create: boolean;
  meta?: string | undefined;
  match: readonly [start: number, end: number] | null;
};

/** TAG_OPTION_LIMIT is how many suggestions a tag field lists for one draft. */
export const TAG_OPTION_LIMIT = 6;

function foldedWithOrigins(text: string) {
  let folded = "";
  const starts: number[] = [];
  const ends: number[] = [];
  let index = 0;
  for (const char of text) {
    const end = index + char.length;
    const part = fold(char);
    if (part === "" && ends.length > 0) ends[ends.length - 1] = end;
    for (let unit = 0; unit < part.length; unit++) {
      starts.push(index);
      ends.push(end);
    }
    folded += part;
    index = end;
  }
  return { folded, starts, ends };
}

/**
 * matchSpan finds where `draft` occurs in `text` when case and accents are
 * ignored, and answers with the start and end of that part in `text` itself,
 * so "nghe" marks "Nghé" in "Nghé con", combining marks included. It is null
 * when the draft is blank or does not occur.
 */
export function matchSpan(
  text: string,
  draft: string,
): readonly [start: number, end: number] | null {
  const needle = fold(chipText(draft));
  if (needle === "") return null;
  const { folded, starts, ends } = foldedWithOrigins(text);
  const at = folded.indexOf(needle);
  const start = starts[at];
  const end = ends[at + needle.length - 1];
  return start === undefined || end === undefined ? null : [start, end];
}

/**
 * tagOptions lists what a tag field offers for `draft`: the suggestions that
 * contain it when case and accents are ignored and are not among `chosen`,
 * those that start with it first and otherwise in the order given, six at
 * most; then the row that creates the draft as a new tag, unless a suggestion
 * or a chosen tag already equals it. A blank draft lists nothing. Two
 * suggestions that are equal under `fold` are one, the first.
 */
export function tagOptions(
  draft: string,
  chosen: readonly string[],
  suggestions: readonly TagSuggestion[],
): TagOption[] {
  const entry = chipText(draft);
  const needle = fold(entry);
  if (needle === "") return [];
  const taken = new Set(chosen.map(fold));
  const seen = new Set<string>();
  const leading: TagOption[] = [];
  const containing: TagOption[] = [];
  let exact = taken.has(needle);
  for (const { tag, meta } of suggestions) {
    const key = fold(tag);
    if (key === needle) exact = true;
    if (seen.has(key) || taken.has(key) || !key.includes(needle)) continue;
    seen.add(key);
    const option = { tag, create: false, meta, match: matchSpan(tag, entry) };
    if (key.startsWith(needle)) leading.push(option);
    else containing.push(option);
  }
  const options = [...leading, ...containing].slice(0, TAG_OPTION_LIMIT);
  if (!exact) options.push({ tag: entry, create: true, match: null });
  return options;
}
