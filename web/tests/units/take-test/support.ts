import { createElement, type ReactElement, type ReactNode } from "react";
import { render } from "@testing-library/react";
import type { AttemptSession } from "@/features/take-test/api";
import { PaperSourceContext, useLivePaperSource } from "@/features/take-test/paper";

/** A payload the store can hydrate from, with only the parts it reads. */
export function session(over: {
  serverTime: string;
  deadlineAt: string;
  answers?: AttemptSession["answers"];
  status?: AttemptSession["attempt"]["status"];
}): AttemptSession {
  return {
    attempt: {
      id: "att-1",
      assignmentId: "asg-1",
      studentId: "stu-1",
      testVersionId: "ver-1",
      attemptNo: 1,
      status: over.status ?? "in_progress",
      startedAt: "2026-09-01T08:00:00.000Z",
      deadlineAt: over.deadlineAt,
    },
    testTitle: "Unit 5 — Present perfect & listening",
    sections: [{ id: "s1", title: "Phần 1", instructions: null }],
    questions: [],
    sessionId: "ses-1",
    beaconToken: "beacon",
    serverTime: over.serverTime,
    audioPlays: {},
    answers: over.answers ?? {},
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
}

export function text(value: string) {
  return { type: "text", value } as const;
}

export { viewport } from "@tests/support/viewport";

function LivePaper({ children }: Readonly<{ children: ReactNode }>) {
  return createElement(PaperSourceContext, { value: useLivePaperSource() }, children);
}

/**
 * renderWithPaper renders an engine component inside the live paper source,
 * filled from the stores exactly as TakeTestPage fills it.
 */
export function renderWithPaper(ui: ReactElement) {
  return render(ui, { wrapper: LivePaper });
}
