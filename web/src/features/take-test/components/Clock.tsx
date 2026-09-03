import { useEffect, useReducer } from "react";
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
      {mmss(remainingMs({ deadlineAt, offsetMs }))}
    </span>
  );
}

function mmss(ms: number): string {
  const total = Math.floor(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
