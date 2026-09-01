import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api/errors";

export const AUTOSAVE_DELAY_MS = 1500;

export type AutosaveStatus =
  | { kind: "idle" }
  /** Edited, inside the debounce window. Nothing has been sent yet. */
  | { kind: "dirty" }
  | { kind: "saving" }
  | { kind: "saved"; at: Date }
  | { kind: "failed"; message: string }
  /** A second tab wrote first. Editing here would silently discard theirs. */
  | { kind: "stale" };

interface AutosaveOptions<T> {
  save: (value: T) => Promise<void>;
  delay?: number;
}

/**
 * Merges the statuses of several autosaves into the one the topbar shows.
 *
 * A-04 has a single "Đã lưu 14:32" in the topbar, and the builder runs two
 * autosaves -- the outline and the open question. Two badges saying different
 * things is worse than one: the teacher wants to know whether it is safe to
 * close the tab, which is a question about all of it.
 */
export function mergeAutosave(statuses: AutosaveStatus[]): AutosaveStatus {
  const stale = statuses.find((s) => s.kind === "stale");
  if (stale) return stale;

  const failed = statuses.find((s) => s.kind === "failed");
  if (failed) return failed;

  if (statuses.some((s) => s.kind === "saving")) return { kind: "saving" };

  // Below "saving" because a request already going out is the more informative
  // answer, but above "saved": one unsent edit makes the whole screen unsaved.
  if (statuses.some((s) => s.kind === "dirty")) return { kind: "dirty" };

  // The most recent save is the honest answer: an older one says less.
  let newest: AutosaveStatus = { kind: "idle" };
  for (const status of statuses) {
    if (status.kind !== "saved") continue;
    if (newest.kind !== "saved" || status.at > newest.at) newest = status;
  }
  return newest;
}

/**
 * §8's 1.5s debounced autosave.
 *
 * Debounced rather than throttled, and coalescing rather than queueing: typing
 * a title produces one request when the typing stops, not one per keystroke and
 * not a backlog that arrives out of order.
 *
 * A `STALE_WRITE` is terminal for the session. §1.3 says one admin edits at a
 * time, so the second tab has nothing useful to retry -- retrying would
 * overwrite whatever the first tab saved, which is precisely the loss the
 * version guard exists to prevent.
 */
export function useAutosave<T>({
  save,
  delay = AUTOSAVE_DELAY_MS,
}: AutosaveOptions<T>) {
  const [status, setStatus] = useState<AutosaveStatus>({ kind: "idle" });
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pending = useRef<T | null>(null);
  const inFlight = useRef<Promise<void> | null>(null);
  const latestSave = useRef(save);
  const stale = useRef(false);

  useEffect(() => {
    latestSave.current = save;
  }, [save]);

  const run = useCallback(async () => {
    const value = pending.current;
    pending.current = null;
    if (value === null || stale.current) return;

    setStatus({ kind: "saving" });
    const attempt = (async () => {
      try {
        await latestSave.current(value);
        setStatus({ kind: "saved", at: new Date() });
      } catch (cause) {
        if (cause instanceof ApiError && cause.code === "STALE_WRITE") {
          stale.current = true;
          setStatus({ kind: "stale" });
          return;
        }
        setStatus({
          kind: "failed",
          message: cause instanceof ApiError ? cause.message : "",
        });
      }
    })();

    inFlight.current = attempt;
    try {
      await attempt;
    } finally {
      if (inFlight.current === attempt) inFlight.current = null;
    }
  }, []);

  const schedule = useCallback(
    (value: T) => {
      if (stale.current) return;
      pending.current = value;
      // Reported synchronously: until this lands, "Đã lưu 14:32" would be the
      // previous save's timestamp, and §8's badge answers "is it safe to close
      // the tab" -- which during this window it is not.
      setStatus({ kind: "dirty" });
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => void run(), delay);
    },
    [delay, run],
  );

  /**
   * Resolves once nothing is left to save -- used before publishing.
   *
   * Awaiting the in-flight request matters as much as flushing the pending
   * one. The debounce puts the save exactly where the teacher's hand is going:
   * they stop typing, it fires at 1.5s, and they reach for Publish. Returning
   * while that PATCH is still on the wire lets publish overtake it, and a
   * version that froze without the last edit cannot be repaired -- it is
   * immutable.
   */
  const flush = useCallback(async () => {
    if (timer.current) clearTimeout(timer.current);
    if (pending.current !== null) {
      await run();
      return;
    }
    await inFlight.current;
  }, [run]);

  // Unmounting must not drop an edit made inside the debounce window.
  //
  // The builder swaps the question editor as soon as another question is
  // selected, so "type, then click the next question" is the ordinary way to
  // work — and clearing the timer without saving loses exactly that edit,
  // silently, while the indicator still says the last thing it saved.
  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
      const value = pending.current;
      pending.current = null;
      if (value !== null && !stale.current) void latestSave.current(value);
    };
  }, []);

  return { status, schedule, flush };
}
