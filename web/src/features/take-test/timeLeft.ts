import { remainingMs, type LockReason } from "./store";

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
