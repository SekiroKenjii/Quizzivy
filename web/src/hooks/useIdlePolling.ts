import { useEffect, useEffectEvent, useSyncExternalStore } from "react";

/**
 * IDLE_AFTER_MS is how long a user may go without pointer or keyboard input
 * before a poll stops for them.
 */
export const IDLE_AFTER_MS = 600_000;

const INPUTS = [
  "pointerdown",
  "pointermove",
  "keydown",
  "wheel",
  "touchstart",
] as const;
const LISTENING = { capture: true, passive: true } as const;

const watchers = new Set<() => void>();
const resumers = new Set<() => void>();
let idle = false;
let hidden = false;
let lastInputAt = 0;
let timer: ReturnType<typeof setTimeout> | undefined;

function tell(listeners: Set<() => void>) {
  for (const listener of [...listeners]) listener();
}

function expire() {
  const wait = lastInputAt + IDLE_AFTER_MS - Date.now();
  if (wait > 0) {
    timer = setTimeout(expire, wait);
    return;
  }
  idle = true;
  tell(watchers);
}

function onInput() {
  lastInputAt = Date.now();
  if (!idle) return;
  idle = false;
  timer = setTimeout(expire, IDLE_AFTER_MS);
  tell(watchers);
  tell(resumers);
}

function onVisibility() {
  const was = hidden;
  hidden = document.visibilityState === "hidden";
  if (was === hidden) return;
  if (!hidden) {
    lastInputAt = Date.now();
    if (idle) {
      idle = false;
      timer = setTimeout(expire, IDLE_AFTER_MS);
    }
  }
  tell(watchers);
  if (!hidden) tell(resumers);
}

function attach() {
  hidden = document.visibilityState === "hidden";
  lastInputAt = Date.now();
  timer = setTimeout(expire, IDLE_AFTER_MS);
  for (const type of INPUTS) window.addEventListener(type, onInput, LISTENING);
  document.addEventListener("visibilitychange", onVisibility);
}

function detach() {
  clearTimeout(timer);
  idle = false;
  for (const type of INPUTS) window.removeEventListener(type, onInput, LISTENING);
  document.removeEventListener("visibilitychange", onVisibility);
}

function join(listeners: Set<() => void>, listener: () => void) {
  if (watchers.size + resumers.size === 0) attach();
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (watchers.size + resumers.size === 0) detach();
  };
}

const watch = (listener: () => void) => join(watchers, listener);
const unwatched = () => () => {};
const awake = () => !idle && document.visibilityState !== "hidden";
const awakeOnServer = () => true;

/**
 * useIdlePolling is the `refetchInterval` of a query that should poll only for
 * somebody who is there: `ms` while `enabled`, the tab is visible and the user
 * has pressed a key, moved or pressed the pointer, turned the wheel or touched
 * the screen within IDLE_AFTER_MS, and `false` otherwise. The clock starts
 * when the first consumer mounts and again when a hidden tab becomes visible.
 * Every consumer shares one set of window listeners and one timer, and
 * re-renders only when the answer flips, never on an input.
 */
export function useIdlePolling(ms: number, enabled = true): number | false {
  const there = useSyncExternalStore(enabled ? watch : unwatched, awake, awakeOnServer);
  return enabled && there ? ms : false;
}

/**
 * useRefetchOnResume calls `refetch` once each time the user comes back while
 * `enabled`: at the first input after idleness, and when a hidden tab becomes
 * visible again. It never calls it on mount, because `enabled` changed, or on
 * an input from a user who never left, and it calls the `refetch` of the
 * latest render. The query client does not refetch on focus, so a poll that
 * useIdlePolling stopped would otherwise leave old data on screen for one
 * more interval after the user returns.
 */
export function useRefetchOnResume(refetch: () => unknown, enabled = true): void {
  const resume = useEffectEvent(() => {
    refetch();
  });
  useEffect(() => {
    if (!enabled) return;
    return join(resumers, () => resume());
  }, [enabled]);
}
