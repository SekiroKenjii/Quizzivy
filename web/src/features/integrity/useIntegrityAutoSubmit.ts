import { useEffect } from "react";
import { useTakeTestStore } from "@/features/take-test/store";
import { strikeState } from "./strikes";

/** useIntegrityAutoSubmit submits once the focus limit is exceeded and retries while keeping answers locked. */
export function useIntegrityAutoSubmit(count: number): boolean {
  const policy = useTakeTestStore((s) => s.integrity);
  const submit = useTakeTestStore((s) => s.submit);
  const state = useTakeTestStore((s) => s.submitState);
  const reason = useTakeTestStore((s) => s.submitReason);
  const delay = useTakeTestStore((s) => s.retryDelayMs);
  const lock = useTakeTestStore((s) => s.lock);
  const exceeded =
    policy !== null &&
    policy.onLimitExceeded === "auto_submit" &&
    strikeState(policy, count).exceeded;
  const required =
    (exceeded || reason === "auto_submit") && state !== "done" && lock !== "superseded";
  useEffect(() => {
    if (!required || state !== "idle") return;
    const timer = setTimeout(
      () => {
        void submit("auto_submit");
      },
      reason === "auto_submit" ? delay : 0,
    );
    return () => clearTimeout(timer);
  }, [required, state, reason, delay, submit]);
  return required;
}
