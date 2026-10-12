import { useSyncExternalStore } from "react";

/** SidebarState is the teacher sidebar's width from 768px: 248px or 60px. */
export type SidebarState = "expanded" | "collapsed";

const STORAGE_KEY = "quizzivy.sidebar";

const listeners = new Set<() => void>();
let unsaved: SidebarState | null = null;

/**
 * readSidebarState returns the stored choice, or `expanded` when nothing
 * valid is stored or storage is unavailable. A choice that storage refused
 * to keep is returned for as long as the page lives.
 */
export function readSidebarState(): SidebarState {
  if (unsaved !== null) return unsaved;
  try {
    return localStorage.getItem(STORAGE_KEY) === "collapsed" ? "collapsed" : "expanded";
  } catch {
    return "expanded";
  }
}

/** writeSidebarState stores the choice and tells every reader on the page. */
export function writeSidebarState(state: SidebarState): void {
  try {
    localStorage.setItem(STORAGE_KEY, state);
    unsaved = null;
  } catch {
    unsaved = state;
  }
  for (const listener of [...listeners]) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * useSidebarState returns the stored choice and re-renders when
 * writeSidebarState changes it.
 */
export function useSidebarState(): SidebarState {
  return useSyncExternalStore(subscribe, readSidebarState, () => "expanded");
}
