import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api/errors";

export const AUTOSAVE_DELAY_MS = 1500;

export type AutosaveStatus =
  | { kind: "idle" }
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
  }, []);

  const schedule = useCallback(
    (value: T) => {
      if (stale.current) return;
      pending.current = value;
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => void run(), delay);
    },
    [delay, run],
  );

  /** Saves whatever is pending immediately -- used before publishing. */
  const flush = useCallback(async () => {
    if (timer.current) clearTimeout(timer.current);
    if (pending.current !== null) await run();
  }, [run]);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, []);

  return { status, schedule, flush };
}
