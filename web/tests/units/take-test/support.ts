import type { AttemptSession } from "@/features/take-test/api";

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
