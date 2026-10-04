import { useSyncExternalStore } from "react";
import { useTakeTestStore } from "./store";

/**
 * SaveStatus is what the engine says about the student's answers: every one
 * is on the server, a save is on its way, the device is offline, or the last
 * save failed and the store is trying again.
 */
export type SaveStatus = "saved" | "saving" | "offline" | "failed";

const FIRST_RETRY_MS = useTakeTestStore.getInitialState().retryDelayMs;

function subscribe(onChange: () => void) {
  window.addEventListener("online", onChange);
  window.addEventListener("offline", onChange);
  return () => {
    window.removeEventListener("online", onChange);
    window.removeEventListener("offline", onChange);
  };
}

function connected() {
  return navigator.onLine;
}

function assumeConnected() {
  return true;
}

/**
 * useSaveStatus reads the save state from the store and the browser's
 * connection, never from a timer. Nothing dirty is saved, whatever else is in
 * flight. A dirty answer is saving from the keystroke, through the autosave's
 * debounce and its request. It is failed from the first save the store backs
 * off from until one succeeds, a retry in flight included, so the line does
 * not flicker; the store's retry delay is longer than its first one for
 * exactly that span. Offline wins over both while the browser reports it.
 */
export function useSaveStatus(): SaveStatus {
  const dirty = useTakeTestStore((s) => s.dirty.size > 0);
  const backedOff = useTakeTestStore((s) => s.retryDelayMs > FIRST_RETRY_MS);
  const online = useSyncExternalStore(subscribe, connected, assumeConnected);
  if (!dirty) return "saved";
  if (!online) return "offline";
  return backedOff ? "failed" : "saving";
}
