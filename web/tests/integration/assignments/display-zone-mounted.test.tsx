import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Monitor } from "@/features/attempts/components/Monitor";
import { assignment, defaultRows, monitor } from "@tests/units/attempts/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { ExtendAssignmentsDialog } from "@/features/assignments/pages/teacher/ExtendAssignmentsDialog";
import { previewRules } from "@/features/assignments/studentRules";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import type { components } from "@/lib/api/schema";
import {
  APP_TIME_ZONE,
  fromDateTimeInput,
  setDisplayTimeZone,
} from "@/lib/i18n/datetime";
import i18n from "@/lib/i18n";
import "@/features/auth/accountPreferences";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { installWizardServer } from "@tests/support/assignmentWizard";
import { DeckScale } from "@/components/ui/deck-scale";
import AssignmentWizardPage from "@/features/assignments/pages/teacher/AssignmentWizardPage";
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

it("keeps the closed Extend's choice, reason and focus across account zone, language and theme changes, and reopens from now", async () => {
  let sent: unknown;
  server.use(
    http.post(`${BASE}/teacher/assignments/${ID}/reopen`, async ({ request }) => {
      sent = await request.json();
      return HttpResponse.json(saved);
    }),
  );
  const closed = {
    ...saved,
    window: { ...saved.window, closesAt: "2026-08-28T14:00:00Z" },
    status: "closed" as const,
  };
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ExtendAssignmentsDialog
        items={[closed]}
        when="21:00 · 28/08"
        open
        onOpenChange={() => undefined}
        onExtended={() => undefined}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  await user.click(screen.getByRole("button", { name: "1 ngày" }));
  const reason = screen.getByRole("textbox");
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
  expect(screen.getByRole("button", { name: "1 day" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.getByRole("dialog")).toHaveTextContent(
    "Opens again until tomorrow, 06:00.",
  );
  expect(document.documentElement).toHaveClass("dark");
  await user.click(screen.getByRole("button", { name: "Extend" }));
  await waitFor(() => expect(sent).toBeDefined());
  const body = sent as { closesAt: string; reason: string };
  expect(body.reason).toBe("Retained reason");
  expect(
    Math.abs(new Date(body.closesAt).getTime() - Date.parse("2026-08-30T10:00:00Z")),
  ).toBeLessThan(60_000);
});

it("keeps the wizard's mounted dirty close time fixed to HCM while the account's zone and language change", async () => {
  installWizardServer();
  server.use(
    http.get(`${BASE}/teacher/assignments/${ID}`, () => HttpResponse.json(saved)),
  );
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/assignments/:id/edit",
        element: (
          <DeckScale>
            <AssignmentWizardPage />
          </DeckScale>
        ),
      },
    ],
    { initialEntries: [`/teacher/assignments/${ID}/edit?step=3`] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const title = await screen.findByRole("heading", {
    level: 1,
    name: "Chỉnh sửa bài giao",
  });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const closes = await screen.findByRole("button", { name: /^Đóng .*21:00$/ });
  await user.click(closes);
  await user.click(screen.getByRole("button", { name: "Giảm giờ" }));
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(closes).toHaveAccessibleName(/20:05$/);
  closes.focus();
  await act(async () => {
    useAuthStore.getState().setUser({
      ...teacherUser,
      locale: "en",
      timeZone: "UTC",
      preferences: { theme: "dark" },
    });
  });
  expect(screen.getByRole("heading", { level: 1, name: "Edit assignment" })).toBe(
    title,
  );
  expect(closes).toHaveAccessibleName(/^Closes .*20:05$/);
  expect(closes).toHaveFocus();
  expect(fromDateTimeInput("2099-09-09T20:05").toISOString()).toBe(
    "2099-09-09T13:05:00.000Z",
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
  expect(previewRules(draft, i18n.getFixedT("vi"), "vi", new Date())[0]?.text).toBe(
    "Bài mở đến 21:00 hôm nay.",
  );
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
