import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react";
import { useDebounced } from "@/lib/useDebounced";

/**
 * useDebouncedInput keeps the text of a field that drives a URL parameter. It
 * returns the text and its setter for the field to be controlled by, so a
 * keystroke never waits on a navigation, and gives `apply` what `normalize`
 * makes of the text once it has stopped changing for `delay` ms, only when
 * that differs from what was applied last. A change of `applied` that this
 * hook did not make, such as a cleared filter or a visit through Back, puts
 * the new value in the field, and it stays there however often `apply` or
 * `normalize` changes identity; text that was typed before such a change and
 * settles with it is dropped, not applied over it.
 */
export function useDebouncedInput(
  applied: string,
  apply: (value: string) => void,
  normalize: (typed: string) => string,
  delay = 300,
): [string, (typed: string) => void] {
  const [typed, setTyped] = useState(applied);
  const settled = useDebounced(typed, delay);
  const written = useRef(applied);
  const superseded = useRef<string | null>(null);

  const adopt = useEffectEvent((value: string) => {
    written.current = value;
    superseded.current = typed;
    setTyped(value);
  });

  useEffect(() => {
    if (applied === written.current) return;
    adopt(applied);
  }, [applied]);

  const commit = useEffectEvent((value: string) => {
    const next = normalize(value);
    if (next === written.current) return;
    written.current = next;
    apply(next);
  });

  useEffect(() => {
    if (settled !== typed || settled === superseded.current) return;
    commit(settled);
  }, [settled, typed]);

  const type = useCallback((text: string) => {
    superseded.current = null;
    setTyped(text);
  }, []);

  return [typed, type];
}

/** trimmed is the `normalize` of a search that the server reads without its outer spaces. */
export function trimmed(typed: string): string {
  return typed.trim();
}
