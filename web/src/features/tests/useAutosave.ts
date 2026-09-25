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
  /** The save was superseded. No further saves are attempted. */
  | { kind: "stale" };

interface AutosaveOptions<T> {
  save: (value: T) => Promise<void>;
  delay?: number;
  isStale?: (cause: unknown) => boolean;
}

function staleWrite(cause: unknown): boolean {
  return cause instanceof ApiError && cause.code === "STALE_WRITE";
}

/** Merges the statuses of several autosaves into the one the topbar shows. */
export function mergeAutosave(statuses: AutosaveStatus[]): AutosaveStatus {
  const stale = statuses.find((s) => s.kind === "stale");
  if (stale) return stale;

  const failed = statuses.find((s) => s.kind === "failed");
  if (failed) return failed;

  if (statuses.some((s) => s.kind === "saving")) return { kind: "saving" };

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
 * useAutosave is §8's 1.5s debounced autosave. A save failure that `isStale`
 * accepts, STALE_WRITE by default, marks the status stale and stops further
 * saves.
 */
export function useAutosave<T>({
  save,
  delay = AUTOSAVE_DELAY_MS,
  isStale = staleWrite,
}: AutosaveOptions<T>) {
  const [status, setStatus] = useState<AutosaveStatus>({ kind: "idle" });
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pending = useRef<T | null>(null);
  const lastValue = useRef<T | null>(null);
  const inFlight = useRef<Promise<void> | null>(null);
  const latestSave = useRef(save);
  const latestIsStale = useRef(isStale);
  const stale = useRef(false);
  const failure = useRef<unknown>(null);

  useEffect(() => {
    latestSave.current = save;
    latestIsStale.current = isStale;
  }, [save, isStale]);

  const run = useCallback(async function drain(): Promise<void> {
    if (inFlight.current !== null) {
      await inFlight.current;
      return drain();
    }
    const value = pending.current;
    pending.current = null;
    if (value === null || stale.current) return;

    failure.current = null;
    setStatus({ kind: "saving" });
    const attempt = (async () => {
      try {
        await latestSave.current(value);
        setStatus(
          pending.current === null
            ? { kind: "saved", at: new Date() }
            : { kind: "dirty" },
        );
      } catch (cause) {
        failure.current = cause;
        if (latestIsStale.current(cause)) {
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
      lastValue.current = value;
      pending.current = value;
      setStatus({ kind: "dirty" });
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => void run(), delay);
    },
    [delay, run],
  );

  const flush = useCallback(async () => {
    if (timer.current) clearTimeout(timer.current);
    await run();
    while (pending.current !== null && !stale.current) await run();
    if (failure.current !== null) throw failure.current;
  }, [run]);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
      void run();
    };
  }, [run]);

  const retry = useCallback(() => {
    if (lastValue.current === null || stale.current) return;
    pending.current = lastValue.current;
    void run();
  }, [run]);

  return { status, schedule, flush, retry };
}
