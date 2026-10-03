import { useEffect, useReducer, useState } from "react";
import { useTranslation } from "react-i18next";
import { Timer } from "lucide-react";
import { countdown } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { remainingMs, takeTestStore, useTakeTestStore } from "../store";

const LOW_MS = 5 * 60_000;
const LAST_MS = 60_000;

function minutesToSay(
  before: number,
  left: number,
  said: number | null,
): number | null {
  if (left > LOW_MS) return null;
  if (before > LAST_MS && left <= LAST_MS && left > 0) return 1;
  if (before > LOW_MS && left > LAST_MS) return Math.ceil(left / 60_000);
  return said;
}

/**
 * Clock is the engine's timer pill, read from the server's time rather than
 * the device's. Under five minutes it takes the danger tones. It is a
 * `timer`, which a screen reader does not read as it ticks; a polite live
 * region beside it says the time left once when it passes five minutes and
 * once when it passes one, and again if a later deadline lets it pass them
 * again. A paper opened with less than that left is not told a time it never
 * crossed.
 */
export function Clock() {
  const { t } = useTranslation();
  const deadlineAt = useTakeTestStore((s) => s.deadlineAt);
  const offsetMs = useTakeTestStore((s) => s.offsetMs);
  const [said, setSaid] = useState<number | null>(null);
  const [, repaint] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    let before = remainingMs(takeTestStore.getState());
    const tick = setInterval(() => {
      const was = before;
      const left = remainingMs(takeTestStore.getState());
      before = left;
      setSaid((current) => minutesToSay(was, left, current));
      repaint();
    }, 1000);
    return () => clearInterval(tick);
  }, []);
  const left = remainingMs({ deadlineAt, offsetMs });
  return (
    <>
      <span
        role="timer"
        aria-label={t("takeTest.timeLeft")}
        className={cn(
          "text-md mx-auto inline-flex h-9 flex-none items-center gap-[7px] rounded-full px-3.5 font-semibold whitespace-nowrap tabular-nums",
          left < LOW_MS ? "bg-danger-soft text-danger-ink" : "bg-muted text-fg",
        )}
      >
        <Timer aria-hidden="true" className="size-4" />
        {countdown(left)}
      </span>
      <span role="status" className="sr-only">
        {said === null ? "" : t("takeTest.minutesLeft", { count: said })}
      </span>
    </>
  );
}
