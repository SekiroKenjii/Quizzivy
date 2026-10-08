import type { components } from "../../src/lib/api/schema";
import type { DashboardRange } from "../../src/features/dashboard/view";
import { assignment, ASSIGNMENT_ID, ATTEMPT_ID } from "../units/attempts/fixtures";

/** DASHBOARD23_IDS supplies real fixture identities for the two distinct dashboard destinations. */
export const DASHBOARD23_IDS = {
  assignment: ASSIGNMENT_ID,
  attempt: ATTEMPT_ID,
  taking: "018f0000-0000-7000-8000-0000000000d2",
} as const;

/** dashboard23 supplies deterministic calendar readings and required nullable destination fixtures. */
export function dashboard23(
  range: DashboardRange = "14d",
): components["schemas"]["Dashboard"] {
  const length = Number.parseInt(range);
  return {
    openAssignments: 999,
    activeStudents: 999,
    awaitingGrading: 7,
    flaggedAttempts: 3,
    newestFlaggedAttempt: { assignmentId: ASSIGNMENT_ID, attemptId: ATTEMPT_ID },
    recentAttempts: [],
    closingSoon: 2,
    waitingStudents: 4,
    oldestWaitingAt: "2026-09-22T01:00:00Z",
    nextClosing: {
      id: ASSIGNMENT_ID,
      title: "Reading Mock",
      closesAt: "2026-09-24T14:00:00Z",
      submittedCount: 18,
      targetCount: 24,
    },
    takingNow: { students: 5, assignments: 2, assignmentId: DASHBOARD23_IDS.taking },
    submissions: {
      days: Array.from({ length }, (_, index) => ({
        date: new Date(Date.UTC(2026, 8, 25 - length + index))
          .toISOString()
          .slice(0, 10),
        count: index % 3,
      })),
      total: length,
      averagePercent: 72,
    },
    today: [
      {
        assignmentId: ASSIGNMENT_ID,
        kind: "opens",
        at: "2026-09-24T11:00:00Z",
        title: "IELTS Evening",
        notSubmitted: 6,
      },
      {
        assignmentId: ASSIGNMENT_ID,
        kind: "closes",
        at: "2026-09-24T14:00:00Z",
        title: "Reading Mock",
        notSubmitted: 6,
      },
    ],
    recentActivity: [
      {
        kind: "submitted",
        at: "2026-09-24T13:55:00Z",
        studentName: "Trần Minh Anh",
        subject: "Reading Mock",
        flagged: false,
      },
      {
        kind: "started",
        at: "2026-09-24T13:48:00Z",
        studentName: "Lê Hoàng Nam",
        subject: "Listening",
        flagged: true,
      },
    ],
  };
}

/** dashboard23Summary keeps unread notifications independent from nullable teaching counts. */
export function dashboard23Summary(): components["schemas"]["TeacherSummary"] {
  return { liveAssignments: 3, answersToGrade: 6, unreadNotifications: 2 };
}

/** dashboard23Assignments distinguishes an empty open list from an empty assignment collection. */
export function dashboard23Assignments(all = 1, populated = true) {
  return {
    items: populated
      ? [
          assignment({
            testTitle: "Unit 4 · Listening",
            submittedCount: 18,
            targetCount: 24,
            pendingGradingCount: 2,
            pendingManualCount: 9,
          }),
        ]
      : [],
    page: 1,
    pageSize: 10,
    total: populated ? 1 : 0,
    facets: {
      all,
      draft: 0,
      scheduled: populated ? 0 : all,
      open: populated ? 1 : 0,
      closed: 0,
    },
  };
}

/** dashboard23Draft is the created draft response used by public creation and navigation cases. */
export function dashboard23Draft(): components["schemas"]["Test"] {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    id: "018f0000-0000-7000-8000-0000000000a1",
    title: "Đề thi chưa đặt tên",
    description: null,
    status: "draft",
    currentVersion: 0,
    totalPoints: 1,
    questionCount: 0,
    audioCount: 0,
    sections: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}
