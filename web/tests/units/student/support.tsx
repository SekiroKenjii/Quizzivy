import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import { http } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";

export const BASE = "http://localhost:8080";
export const ASSIGNMENT = "018f0000-0000-7000-8000-0000000000d1";
export const ATTEMPT = "018f0000-0000-7000-8000-0000000000e1";

export const STUDENT = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  role: "student" as const,
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
};

export const POLICY = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

export const REVIEW = {
  showScore: true,
  showCorrectAnswers: false,
  showExplanations: true,
};

/** A contract-valid card, open right now, with room to override. */
export function card(over: Record<string, unknown> = {}) {
  return {
    id: ASSIGNMENT,
    testTitle: "Unit 5 — Present perfect",
    status: "open",
    opensAt: "2026-08-29T01:00:00Z",
    closesAt: "2026-08-29T14:00:00Z",
    durationMinutes: 45,
    questionCount: 24,
    totalPoints: 30,
    attemptsUsed: 0,
    maxAttempts: 2,
    hasLiveAttempt: false,
    ...over,
  };
}

export function detail(over: Record<string, unknown> = {}) {
  return {
    ...card(),
    review: REVIEW,
    integrity: POLICY,
    hasAudio: false,
    showsTranscript: false,
    ...over,
  };
}

/** What POST /app/assignments/{id}/attempts answers with: the least valid session. */
export function attemptSession() {
  return {
    attempt: {
      id: ATTEMPT,
      assignmentId: ASSIGNMENT,
      studentId: STUDENT.id,
      testVersionId: "018f0000-0000-7000-8000-0000000000f1",
      attemptNo: 1,
      status: "in_progress",
      startedAt: "2026-08-29T10:00:00Z",
      deadlineAt: "2026-08-29T10:45:00Z",
    },
    questions: [],
    sessionId: "018f0000-0000-7000-8000-0000000000c9",
    beaconToken: "beacon",
    serverTime: "2026-08-29T10:00:00Z",
    audioPlays: {},
    answers: {},
    integrity: POLICY,
  };
}

export function mockStart(status: 200 | 409 = 200) {
  const calls: string[] = [];
  server.use(
    http.post(`${BASE}/app/assignments/${ASSIGNMENT}/attempts`, () => {
      calls.push("start");
      if (status === 409) {
        return contractJson("/app/assignments/{id}/attempts", "post", 409, {
          error: {
            code: "ATTEMPT_LIMIT_REACHED",
            message: "Bạn đã dùng hết số lượt làm bài.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        });
      }
      return contractJson(
        "/app/assignments/{id}/attempts",
        "post",
        200,
        attemptSession(),
      );
    }),
  );
  return calls;
}

export function renderAt(path: string, routes: RouteObject[]) {
  useAuthStore.getState().setSession("token", STUDENT);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      ...routes,
      { path: "/app/attempts/:id", element: <p>engine</p> },
      { path: "/join", element: <p>join page</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}
