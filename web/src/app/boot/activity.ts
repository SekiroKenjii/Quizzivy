const IDLE_AFTER_MS = 10 * 60_000;
const INPUTS = ["pointerdown", "keydown", "wheel", "touchstart"] as const;

let lastInput = Date.now();
let tracking = false;

function noteInput() {
  lastInput = Date.now();
}

/**
 * trackActivity starts noting the user's input, once per page; later calls do
 * nothing. `isActive` reads what it notes.
 */
export function trackActivity(): void {
  if (tracking) return;
  tracking = true;
  for (const input of INPUTS) {
    window.addEventListener(input, noteInput, { capture: true, passive: true });
  }
}

/**
 * isActive reports whether the tab is visible and the user has touched the page
 * in the last ten minutes. Background checks run only while it holds, so an
 * idle tab stops polling even while it is still on screen.
 */
export function isActive(now = Date.now()): boolean {
  return document.visibilityState === "visible" && now - lastInput < IDLE_AFTER_MS;
}
