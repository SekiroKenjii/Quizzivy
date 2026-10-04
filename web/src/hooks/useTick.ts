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

const idle = () => () => {};
const second = () => Math.floor(Date.now() / 1000);
const minute = () => Math.floor(Date.now() / 60_000);
const frozen = () => 0;

/**
 * The current second, re-rendering once a second while `live`. One interval
 * for every subscriber, and a stable snapshot within a second so React does
 * not see a store that changes on every read.
 */
export function useTick(live: boolean): number {
  return useSyncExternalStore(live ? subscribe : idle, live ? second : frozen);
}

/**
 * useMinute is the current minute, re-rendering when it changes while `live`.
 * It shares useTick's interval, so a page that only has to notice the hour or
 * the day passing does not repaint every second.
 */
export function useMinute(live: boolean): number {
  return useSyncExternalStore(live ? subscribe : idle, live ? minute : frozen);
}
