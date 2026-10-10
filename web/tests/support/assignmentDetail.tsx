import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/teacher/AssignmentDetailPage";
import { Toaster } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";

/** DETAIL_API is the stubbed API origin the assignment detail calls. */
export const DETAIL_API = "http://localhost:8080";
/** DETAIL_ID is the assignment every stub answers for. */
export const DETAIL_ID = "018f0000-0000-7000-8000-0000000000d1";
/** DETAIL_TEST_ID is the assignment's test. */
export const DETAIL_TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
/** DETAIL_VERSION_ID is the assignment's pinned version. */
export const DETAIL_VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
/** DETAIL_CLASS_ID is the class the assignment targets. */
export const DETAIL_CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";

type Assignment = components["schemas"]["Assignment"];
type MonitorRow = components["schemas"]["MonitorRow"];
type ItemAnalysis = components["schemas"]["ItemAnalysis"];

/** detailAssignment is an open assignment with `over` applied. */
export function detailAssignment(over: Partial<Assignment> = {}): Assignment {
  return {
    id: DETAIL_ID,
    testId: DETAIL_TEST_ID,
    testVersionId: DETAIL_VERSION_ID,
    testVersion: 3,
    testTitle: "Unit 5 — Present perfect & listening",
    targets: {
      classes: [{ id: DETAIL_CLASS_ID, name: "IELTS Foundation", studentCount: 18 }],
      students: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T02:12:00Z",
    window: {
      opensAt: "2020-09-07T01:00:30Z",
      closesAt: "2099-09-09T14:00:30Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: true,
    shuffleOptions: true,
    review: {
      showScore: true,
      showCorrectAnswers: false,
      showExplanations: false,
      release: "on_submit",
      showClassAverage: false,
    },
    studentNote: null,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    status: "open",
    submittedCount: 1,
    targetCount: 18,
    flaggedCount: 0,
    pendingGradingCount: 0,
    ...over,
  };
}

/** detailRow is a monitor row for `fullName`, not started unless `over` says otherwise. */
export function detailRow(
  studentId: string,
  fullName: string,
  over: Partial<MonitorRow> = {},
): MonitorRow {
  return {
    studentId,
    fullName,
    state: "not_started",
    flagged: false,
    audioOverLimit: false,
    ...over,
  };
}

/** DetailCalls collects what the page sent: PATCH bodies and override bodies. */
export type DetailCalls = { patches: unknown[]; overrides: unknown[] };

/**
 * serveDetail stubs the assignment, its versions, its monitor with `rows`,
 * its item analysis and the writes the page makes. `patchStatus` answers
 * every PATCH with that status instead, as `ASSIGNMENT_LOCKED` when 409.
 */
export function serveDetail(
  a: Assignment,
  {
    rows = [],
    analysis = { handedIn: 0, items: [] },
    patchStatus = 200,
  }: {
    rows?: MonitorRow[];
    analysis?: ItemAnalysis;
    patchStatus?: 200 | 409;
  } = {},
): DetailCalls {
  const calls: DetailCalls = { patches: [], overrides: [] };
  server.use(
    http.get(`${DETAIL_API}/teacher/assignments/${DETAIL_ID}`, () =>
      contractJson("/teacher/assignments/{id}", "get", 200, a),
    ),
    http.get(`${DETAIL_API}/teacher/tests/${DETAIL_TEST_ID}/versions`, () =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, {
        items: [
          {
            id: DETAIL_VERSION_ID,
            version: 3,
            totalPoints: 30,
            questionCount: 24,
            audioCount: 4,
            manualCount: 2,
            skills: ["grammar"],
            assignmentCount: 0,
            changeNote: null,
            publishedAt: "2026-08-20T00:00:00Z",
            publishedBy: "Thuong",
          },
        ],
      }),
    ),
    http.get(`${DETAIL_API}/teacher/assignments/${DETAIL_ID}/attempts`, () =>
      contractJson("/teacher/assignments/{id}/attempts", "get", 200, {
        serverTime: "2026-09-04T02:10:00Z",
        questionCount: 24,
        rows,
      }),
    ),
    http.get(`${DETAIL_API}/teacher/assignments/${DETAIL_ID}/item-analysis`, () =>
      contractJson("/teacher/assignments/{id}/item-analysis", "get", 200, analysis),
    ),
    http.patch(
      `${DETAIL_API}/teacher/assignments/${DETAIL_ID}`,
      async ({ request }) => {
        calls.patches.push(await request.json());
        if (patchStatus === 409)
          return contractJson("/teacher/assignments/{id}", "patch", 409, {
            error: {
              code: "ASSIGNMENT_LOCKED",
              message: "Bài đang mở nên không đổi được thời gian làm.",
              requestId: "018f0000-0000-7000-8000-0000000009a1",
            },
          });
        return contractJson("/teacher/assignments/{id}", "patch", 200, a);
      },
    ),
    http.put(
      `${DETAIL_API}/teacher/assignments/${DETAIL_ID}/student-overrides`,
      async ({ request }) => {
        calls.overrides.push(await request.json());
        return contractJson("/teacher/assignments/{id}/student-overrides", "put", 200, {
          items: [],
        });
      },
    ),
  );
  return calls;
}

/** renderDetail mounts the detail page on `tab`, with a toaster, and returns a user-event session. */
export function renderDetail(tab = "students") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/assignments/:id", element: <AssignmentDetailPage /> },
      { path: "/teacher/assignments/:id/edit", element: <p>edit form</p> },
    ],
    { initialEntries: [`/teacher/assignments/${DETAIL_ID}?tab=${tab}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}
