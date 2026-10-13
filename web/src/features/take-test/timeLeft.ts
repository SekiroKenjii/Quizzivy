import type { LockReason } from "./store";

/**
 * remainingMs is the time left until `deadlineAt` on the SERVER's clock: the
 * device's time corrected by `offsetMs`, never Date.now() alone, and never
 * below zero.
 */
export function remainingMs(state: { deadlineAt: number; offsetMs: number }): number {
  return Math.max(0, state.deadlineAt - (Date.now() + state.offsetMs));
}

/**
 * timeLeft is the time the student still has, in milliseconds on the server's
 * clock: zero once the server has refused a save because the time is up,
 * whatever this device's own deadline says.
 */
export function timeLeft(state: {
  deadlineAt: number;
  offsetMs: number;
  lock: LockReason | null;
}): number {
  return state.lock === "deadline" ? 0 : remainingMs(state);
}
