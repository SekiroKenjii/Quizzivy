import { useEffect, useRef, useState } from "react";
import { useDebounced } from "@/lib/useDebounced";

/**
 * useDebouncedInput keeps the text of a field that drives a URL parameter. It
 * returns the text and its setter for the field to be controlled by, so a
 * keystroke never waits on a navigation, and gives `apply` what `normalize`
 * makes of the text once it has stopped changing for `delay` ms, only when
 * that differs from what was applied last. A change of `applied` that this
 * hook did not make, such as a cleared filter or a visit through Back, puts
 * the new value in the field. `normalize` must be stable.
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

  useEffect(() => {
    if (applied === written.current) return;
    written.current = applied;
    setTyped(applied);
  }, [applied]);

  useEffect(() => {
    if (settled !== typed) return;
    const next = normalize(settled);
    if (next === written.current) return;
    written.current = next;
    apply(next);
  }, [settled, typed, normalize, apply]);

  return [typed, setTyped];
}
