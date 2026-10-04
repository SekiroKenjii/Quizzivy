import { useSyncExternalStore } from "react";

const STORAGE_KEY = "quizzivy.testText";
const LARGER = "large";
const DEFAULT = "default";

const listeners = new Set<() => void>();
let unsaved: boolean | null = null;

/**
 * readLargerTestText reports whether the student chose "Larger text in
 * tests". It is false when nothing valid is stored or storage is unavailable.
 * A choice that storage refused to keep is returned for as long as the page
 * lives.
 */
export function readLargerTestText(): boolean {
  if (unsaved !== null) return unsaved;
  try {
    return localStorage.getItem(STORAGE_KEY) === LARGER;
  } catch {
    return false;
  }
}

/**
 * writeLargerTestText stores the choice in `localStorage['quizzivy.testText']`
 * as `large` or `default`, and tells every reader on the page at once.
 */
export function writeLargerTestText(larger: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, larger ? LARGER : DEFAULT);
    unsaved = null;
  } catch {
    unsaved = larger;
  }
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === STORAGE_KEY || event.key === null) listener();
  };
  listeners.add(listener);
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

/**
 * useLargerTestText returns the choice and re-renders when it changes, on
 * this page or in another tab.
 */
export function useLargerTestText(): boolean {
  return useSyncExternalStore(subscribe, readLargerTestText, () => false);
}
