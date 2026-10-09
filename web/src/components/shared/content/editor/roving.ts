import { useCallback, useState, type KeyboardEvent } from "react";

const MOVES = new Set(["ArrowLeft", "ArrowRight", "Home", "End"]);

/**
 * useRovingToolbar makes a toolbar one tab stop. Each button carries
 * `data-roving`, takes `tabIndex` from `tabIndexFor` and reports its focus with
 * `remember`; the container's `onKeyDown` moves focus between the enabled
 * buttons with the arrow keys, Home and End.
 */
export function useRovingToolbar(enabled: readonly string[]) {
  const [current, setCurrent] = useState<string | null>(null);
  const stop = current !== null && enabled.includes(current) ? current : enabled[0];
  const onKeyDown = useCallback((event: KeyboardEvent<HTMLElement>) => {
    if (!MOVES.has(event.key)) return;
    const buttons = Array.from(
      event.currentTarget.querySelectorAll<HTMLButtonElement>(
        "button[data-roving]:not(:disabled)",
      ),
    );
    const index = buttons.findIndex((button) => button === document.activeElement);
    if (index < 0) return;
    event.preventDefault();
    let next = buttons.length - 1;
    if (event.key === "Home") next = 0;
    else if (event.key === "ArrowRight") next = (index + 1) % buttons.length;
    else if (event.key === "ArrowLeft")
      next = (index - 1 + buttons.length) % buttons.length;
    buttons[next]?.focus();
  }, []);
  return {
    onKeyDown,
    remember: setCurrent,
    tabIndexFor: (key: string) => (key === stop ? 0 : -1),
  };
}
