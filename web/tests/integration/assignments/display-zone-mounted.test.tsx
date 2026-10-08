import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Monitor } from "@/features/attempts/components/Monitor";
import { assignment, defaultRows, monitor } from "@tests/units/attempts/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { ReopenDialog } from "@/features/assignments/components/ReopenDialog";
import { StudentRulesPreview } from "@/features/assignments/components/StudentRulesPreview";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import AssignmentFormPage from "@/features/assignments/pages/teacher/AssignmentFormPage";
import type { components } from "@/lib/api/schema";
import {
  APP_TIME_ZONE,
  setDisplayTimeZone,
  fromDateTimeInput,
} from "@/lib/i18n/datetime";
import i18n from "@/lib/i18n";
import "@/features/auth/accountPreferences";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  ASSIGNMENT,
  BASE,
  STUDENT,
  detail,
  renderAt,
} from "@tests/units/student/support";

const ID = ASSIGNMENT;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const saved: components["schemas"]["Assignment"] = {
  id: ID,
  testId: TEST_ID,
  testVersionId: VERSION_ID,
  testVersion: 3,
  testTitle: "Unit 5 — Present perfect & listening",
  targets: {
    classes: [{ id: CLASS_ID, name: "IELTS Foundation", studentCount: 18 }],
    students: [],
  },
  publishedAt: "2026-08-27T00:00:00Z",
  updatedAt: "2026-08-27T02:12:00Z",
  window: {
    opensAt: "2020-09-07T01:00:00Z",
    closesAt: "2099-09-09T14:00:00Z",
    closedAt: null,
  },
  durationMinutes: 90,
  maxAttempts: 2,
  shuffleQuestions: false,
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
    maxFocusLoss: 0,
    onLimitExceeded: "flag",
    minAwayMs: 3000,
  },
  status: "open",
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
  useAuthStore.getState().setSession("a", teacherUser);
});
afterEach(async () => {
  cleanup();
  vi.useRealTimers();
  useAuthStore.getState().clearSession();
  setDisplayTimeZone(APP_TIME_ZONE);
  await i18n.changeLanguage("vi");
});

it("keeps the same dirty reopen date, reason and focus across account zone, language and theme changes with exact HCM input UTC", async () => {
  let sent: unknown;
  server.use(
    http.post(`${BASE}/teacher/assignments/${ID}/reopen`, async ({ request }) => {
      sent = await request.json();
      return HttpResponse.json(saved);
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ReopenDialog
        assignment={saved}
        choice="pick"
        open
        onOpenChange={() => undefined}
        onDone={() => undefined}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const reason = screen.getByRole("textbox");
  const time = screen.getByRole("button", { name: /giờ/i });
  await user.click(time);
  await user.click(
    within(screen.getByRole("listbox", { name: "Giờ" })).getByRole("option", {
      name: "19",
    }),
  );
  await user.click(
    within(screen.getByRole("listbox", { name: "Phút" })).getByRole("option", {
      name: "35",
    }),
  );
  await user.type(reason, "Retained reason");
  reason.focus();
  await act(async () => {
    useAuthStore.getState().setUser({
      ...teacherUser,
      locale: "en",
      timeZone: "America/New_York",
      preferences: { theme: "dark" },
    });
  });
  expect(screen.getByRole("textbox")).toBe(reason);
  expect(reason).toHaveValue("Retained reason");
  expect(reason).toHaveFocus();
  expect(time).toHaveTextContent("19:35");
  expect(document.documentElement).toHaveClass("dark");
  await user.click(screen.getByRole("button", { name: "Reopen" }));
  await waitFor(() =>
    expect(sent).toEqual({
      closesAt: "2026-08-30T12:35:00.000Z",
      reason: "Retained reason",
    }),
  );
});

it("keeps mounted dirty assignment inputs fixed to HCM while display changes propagate", async () => {
  server.use(
    http.get(`${BASE}/teacher/assignments/${ID}`, () => HttpResponse.json(saved)),
    http.get(`${BASE}/teacher/tests/${TEST_ID}/versions`, () =>
      HttpResponse.json({
        items: [
          {
            id: VERSION_ID,
            version: 3,
            totalPoints: 30,
            questionCount: 24,
            audioCount: 4,
            manualCount: 2,
            publishedAt: "2026-08-20T00:00:00Z",
            publishedBy: "Teacher",
          },
        ],
      }),
    ),
    http.get(`${BASE}/teacher/classes`, () =>
      HttpResponse.json({
        items: [],
        page: 1,
        pageSize: 20,
        total: 0,
        facets: { all: 0, joinable: 0, archived: 0, students: 0 },
      }),
    ),
    http.get(`${BASE}/teacher/students`, () =>
      HttpResponse.json({
        items: [],
        page: 1,
        pageSize: 20,
        total: 0,
        facets: { total: 0, activeLast7Days: 0 },
      }),
    ),
  );
  const router = createMemoryRouter(
    [{ path: "/teacher/assignments/:id/edit", element: <AssignmentFormPage /> }],
    { initialEntries: [`/teacher/assignments/${ID}/edit`] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const title = await screen.findByRole("heading", { name: "Chỉnh sửa bài giao" });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const time = within(screen.getByRole("group", { name: "Đóng lúc" })).getByRole(
    "button",
    { name: /giờ/i },
  );
  await user.click(time);
  await user.click(
    within(screen.getByRole("listbox", { name: "Giờ" })).getByRole("option", {
      name: "19",
    }),
  );
  await user.click(
    within(screen.getByRole("listbox", { name: "Phút" })).getByRole("option", {
      name: "35",
    }),
  );
  time.focus();
  await act(async () => {
    useAuthStore.getState().setUser({
      ...teacherUser,
      locale: "en",
      timeZone: "UTC",
      preferences: { theme: "dark" },
    });
  });
  expect(screen.getByRole("heading", { name: "Edit assignment" })).toBe(title);
  expect(time).toHaveTextContent("19:35");
  expect(time).toHaveFocus();
  expect(fromDateTimeInput("2099-09-09T19:35").toISOString()).toBe(
    "2099-09-09T12:35:00.000Z",
  );
});

it("keeps preview policy dates fixed to HCM while the actual mounted student intro uses the actor zone", async () => {
  const draft = {
    review: saved.review,
    integrity: saved.integrity,
    opensAt: "2026-08-29T08:00",
    closesAt: "2026-08-29T21:00",
  };
  setDisplayTimeZone("UTC");
  render(<StudentRulesPreview draft={draft} />);
  expect(screen.getByText("· Bài mở đến 21:00 hôm nay.")).toBeInTheDocument();
  cleanup();
  server.use(
    http.get(`${BASE}/app/assignments/${ID}`, () =>
      HttpResponse.json(
        detail({ opensAt: "2026-08-29T01:00:00Z", closesAt: "2026-08-29T14:00:00Z" }),
      ),
    ),
  );
  renderAt(`/app/assignments/${ID}`, [
    { path: "/app/assignments/:id", element: <AssignmentIntroPage /> },
  ]);
  await act(async () => {
    useAuthStore.getState().setUser({ ...STUDENT, timeZone: "UTC" });
  });
  const rules = await screen.findByText("Bài mở đến 14:00 hôm nay.");
  act(() => {
    useAuthStore.getState().setUser({ ...STUDENT, timeZone: "America/New_York" });
  });
  expect(rules).toHaveTextContent("Bài mở đến 10:00 hôm nay.");
});

it("refreshes a memoized real Monitor timestamp across zones while preserving row identity", () => {
  contentWidth(1100);
  const row = { ...defaultRows()[1]!, submittedAt: "2026-09-04T02:47:00Z" };
  const router = createMemoryRouter([
    {
      path: "/",
      element: (
        <Monitor
          assignment={assignment()}
          data={monitor([row])}
          selectedAttempt={null}
          onOpen={() => undefined}
          onRefresh={async () => undefined}
        />
      ),
    },
  ]);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const stamp = screen.getByText("09:47 · 04/09");
  const host = stamp.closest('[role="row"]');
  act(() => {
    setDisplayTimeZone("America/New_York");
  });
  expect(stamp).toHaveTextContent("22:47 · 03/09");
  expect(stamp.closest('[role="row"]')).toBe(host);
  act(() => {
    setDisplayTimeZone("UTC");
  });
  expect(stamp).toHaveTextContent("02:47 · 04/09");
});
