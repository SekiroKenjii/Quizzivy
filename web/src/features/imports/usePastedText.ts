import { useDeferredValue, useMemo, useState } from "react";
import {
  normalizePaste,
  pasteLength,
  pastedTitle,
  scanPastedText,
  type PasteScan,
} from "./paste";

/**
 * PasteState is where pasted text stands against Start processing: nothing
 * pasted, over the character limit, no numbered question found, or ready.
 */
export type PasteState = "empty" | "over" | "none" | "ready";

function stateOf(
  text: string,
  length: number,
  max: number,
  scan: PasteScan,
): PasteState {
  if (text === "") return "empty";
  if (length > max) return "over";
  return scan.questions === 0 ? "none" : "ready";
}

/**
 * usePastedText holds the text of a pasted import against a limit of `max`
 * code points. The length, the quick count and the title from the first line
 * describe a deferred copy, so typing never waits for them. `sendable`
 * re-checks the current text and returns it as the page sends it, or null.
 */
export function usePastedText(max: number) {
  const [text, setText] = useState("");
  const settled = useDeferredValue(text);
  const length = useMemo(() => pasteLength(settled), [settled]);
  const scan = useMemo(() => scanPastedText(settled), [settled]);
  const firstLine = useMemo(() => pastedTitle(settled), [settled]);

  function sendable(): string | null {
    const exact = normalizePaste(text);
    if (pasteLength(exact) > max || scanPastedText(exact).questions === 0) return null;
    return exact;
  }

  return {
    text,
    setText,
    length,
    scan,
    firstLine,
    state: stateOf(text, length, max, scan),
    sendable,
  };
}
