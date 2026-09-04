import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  timer ??= setInterval(() => listeners.forEach((l) => l()), 1000);
  return () => {
    listeners.delete(onChange);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

const second = () => Math.floor(Date.now() / 1000);
const frozen = () => 0;

/**
 * The current second, re-rendering once a second while `live`. One interval
 * for every subscriber, and a stable snapshot within a second so React does
 * not see a store that changes on every read.
 */
export function useTick(live: boolean): number {
  return useSyncExternalStore(
    live ? subscribe : () => () => {},
    live ? second : frozen,
  );
}
