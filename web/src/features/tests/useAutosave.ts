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

/** §8's 1.5s debounced autosave. */
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
      setStatus({ kind: "dirty" });
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => void run(), delay);
    },
    [delay, run],
  );

  // Resolves once nothing is left to save -- used before publishing.
  const flush = useCallback(async () => {
    if (timer.current) clearTimeout(timer.current);
    if (pending.current !== null) {
      await run();
      return;
    }
    await inFlight.current;
  }, [run]);

  // Unmounting must not drop an edit made inside the debounce window.
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
