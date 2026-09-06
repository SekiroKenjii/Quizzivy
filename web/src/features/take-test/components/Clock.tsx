import { useEffect, useReducer } from "react";
import { countdown } from "@/lib/i18n/datetime";
import { remainingMs, useTakeTestStore } from "../store";

/** The clock, read from the server's time rather than the device's. */
export function Clock() {
  const deadlineAt = useTakeTestStore((s) => s.deadlineAt);
  const offsetMs = useTakeTestStore((s) => s.offsetMs);
  const [, repaint] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    const tick = setInterval(repaint, 1000);
    return () => clearInterval(tick);
  }, []);
  return (
    <span className="text-sm font-semibold tabular-nums">
      {countdown(remainingMs({ deadlineAt, offsetMs }))}
    </span>
  );
}
