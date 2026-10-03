import { useCallback, useEffect, useRef, useState } from "react";
import { useBlocker, useNavigate } from "react-router";
import { useTakeTestStore } from "./store";

/**
 * LeavePhase is where leaving the test stands: nobody asked, the student is
 * being asked, the answers still unsaved are being sent, or that save failed.
 */
export type LeavePhase = "idle" | "asking" | "saving" | "failed";

/**
 * Leave is the Leave dialog's state and its three moves: `ask` opens it,
 * `stay` closes it and cancels a blocked navigation, and `confirm` saves and
 * then leaves. `pending` says whether an answer is still unsaved.
 */
export interface Leave {
  phase: LeavePhase;
  pending: boolean;
  ask: () => void;
  stay: () => void;
  confirm: () => void;
}

async function savePending(): Promise<boolean> {
  const store = () => useTakeTestStore.getState();
  const saved = await store().flush();
  if (saved && store().dirty.size > 0) await store().flush();
  return store().lock !== null || store().dirty.size === 0;
}

function phaseOf(step: LeavePhase, blocked: boolean, pending: boolean): LeavePhase {
  if (step === "failed") return pending ? "failed" : "asking";
  if (blocked) return "saving";
  return step;
}

/**
 * useLeave is the one way out of a test that is still open. The ✕ asks first.
 * The browser's back button, or any other navigation while an answer is
 * unsaved, is blocked and taken as the answer "leave". Either way the unsaved
 * answers are sent before the route changes: to `/app` from the ✕, to
 * wherever the blocked navigation was going otherwise. A save that fails
 * keeps the student on the paper with the dialog saying so, and nothing
 * navigates until a retry succeeds or the student stays; if the store's own
 * retry lands first, the dialog goes back to asking. A paper another device
 * took over has nothing this tab can save, so Leave goes without a save; a
 * paper that has ended or run out of time has no timer left to warn about,
 * so the ✕ leaves without asking. Closing the tab with an unsaved answer
 * raises the browser's own prompt.
 */
export function useLeave(): Leave {
  const navigate = useNavigate();
  const dirty = useTakeTestStore((s) => s.dirty.size > 0);
  const locked = useTakeTestStore((s) => s.lock !== null);
  const pending = dirty && !locked;
  const [step, setStep] = useState<LeavePhase>("idle");
  const run = useRef(0);
  const blocker = useBlocker(pending);
  const blocked = blocker.state === "blocked";
  const proceed = blocked ? blocker.proceed : undefined;
  const reset = blocked ? blocker.reset : undefined;

  useEffect(() => {
    const runs = run;
    return () => {
      runs.current += 1;
    };
  }, []);

  useEffect(() => {
    if (proceed === undefined) return;
    const mine = ++run.current;
    void savePending().then((saved) => {
      if (run.current !== mine) return;
      if (saved) proceed();
      else setStep("failed");
    });
  }, [proceed]);

  useEffect(() => {
    if (!dirty) return;
    const beforeUnload = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirty]);

  const ask = useCallback(() => {
    const { lock } = useTakeTestStore.getState();
    if (lock === "closed" || lock === "deadline") void navigate("/app");
    else setStep("asking");
  }, [navigate]);

  const stay = useCallback(() => {
    run.current += 1;
    setStep("idle");
    reset?.();
  }, [reset]);

  const confirm = useCallback(() => {
    const mine = ++run.current;
    setStep("saving");
    void savePending().then((saved) => {
      if (run.current !== mine) return;
      if (!saved) {
        setStep("failed");
        return;
      }
      setStep("idle");
      if (proceed === undefined) void navigate("/app");
      else proceed();
    });
  }, [navigate, proceed]);

  return { phase: phaseOf(step, blocked, pending), pending, ask, stay, confirm };
}
