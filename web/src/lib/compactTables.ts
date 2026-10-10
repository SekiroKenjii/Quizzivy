import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();
let accountValue = false;
let previewValue: boolean | null = null;

function notify() {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * readCompactTables reports whether tables draw their compact rows (DG-37):
 * the choice being saved while one is, otherwise the account's stored
 * `preferences.compactTables`, and false with no account.
 */
export function readCompactTables(): boolean {
  return previewValue ?? accountValue;
}

/** previewCompactTables shows a choice at once, before the server keeps it. */
export function previewCompactTables(compact: boolean): void {
  previewValue = compact;
  notify();
}

/**
 * setAccountCompactTables applies the account's stored choice, or the default
 * when `value` is null, and drops any preview.
 */
export function setAccountCompactTables(value: boolean | null): void {
  accountValue = value ?? false;
  previewValue = null;
  notify();
}

/** useCompactTables returns readCompactTables and re-renders when it changes. */
export function useCompactTables(): boolean {
  return useSyncExternalStore(subscribe, readCompactTables, () => false);
}
