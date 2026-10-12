import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import TeacherDashboardPage from "@/features/dashboard/pages/teacher/TeacherDashboardPage";
import { getDashboard } from "@/features/dashboard/api";
import { dashboardKeys } from "@/features/dashboard/keys";
import { useAuthStore } from "@/stores/auth";
import i18n from "@/lib/i18n";
import { teacherUser } from "@tests/support/fixtures";
import { contractJson } from "@tests/support/contractResponse";
import {
  dashboard23,
  dashboard23Assignments,
  dashboard23Draft,
  DASHBOARD23_IDS,
} from "@tests/support/dashboard23";
import { server } from "@tests/support/server";
import { viewport } from "@tests/support/viewport";
import type { DashboardRange } from "@/features/dashboard/view";

const BASE = "http://localhost:8080";
let reads: string[];
let creations: number;

beforeEach(() => {
  reads = [];
  creations = 0;
  useAuthStore.getState().setSession("token", teacherUser);
  server.use(
    http.get(`${BASE}/teacher/dashboard`, ({ request }) => {
      const url = new URL(request.url);
      reads.push(url.pathname + url.search);
      const range = (url.searchParams.get("range") ?? "14d") as DashboardRange;
      return contractJson("/teacher/dashboard", "get", 200, dashboard23(range));
    }),
    http.get(`${BASE}/teacher/assignments`, ({ request }) => {
      const url = new URL(request.url);
      reads.push(url.pathname + url.search);
      return contractJson("/teacher/assignments", "get", 200, dashboard23Assignments());
    }),
    http.post(`${BASE}/teacher/tests`, () => {
      creations += 1;
      return contractJson("/teacher/tests", "post", 201, dashboard23Draft());
    }),
  );
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
  void i18n.changeLanguage("vi");
});

function board(
  search = "",
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  }),
) {
  const router = createMemoryRouter(
    [
      {
        path: "/teacher",
        element: (
          <main data-scale="deck">
            <TeacherDashboardPage />
          </main>
        ),
      },
      { path: "/teacher/tests/:id/edit", element: <p>builder</p> },
      { path: "/teacher/assignments/new", element: <p>assignment wizard</p> },
      { path: "/teacher/assignments/:id", element: <p>assignment destination</p> },
      { path: "/teacher/grading", element: <p>grading destination</p> },
    ],
    { initialEntries: [`/teacher${search}`] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...view, router, client };
}

function error() {
  return HttpResponse.json(
    {
      error: {
        code: "INTERNAL",
        message: "Máy chủ gặp lỗi.",
        requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
      },
    },
    { status: 500 },
  );
}
function region(name: string) {
  return screen.getByRole("region", { name });
}

describe("actual Teacher dashboard readings and actions", () => {
  it("cancels an outstanding home request when its mounted consumer leaves", async () => {
    let signal: AbortSignal | undefined;
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/teacher/dashboard`, async ({ request }) => {
        signal = request.signal;
        await held;
        return contractJson("/teacher/dashboard", "get", 200, dashboard23());
      }),
    );
    const { unmount } = board();
    await waitFor(() => expect(signal).toBeDefined());
    unmount();
    await waitFor(() => expect(signal?.aborted).toBe(true));
    release();
  });
  it("updates the staged app-zone greeting and date at minute boundaries with a safe given-name fallback", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2025-09-24T04:59:00+07:00"));
    useAuthStore
      .getState()
      .setSession("token", { ...teacherUser, fullName: "  Nguyễn Đức Minh  " });
    board();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Chào buổi tối, Minh",
    );
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Chào buổi sáng, Minh",
    );
    act(() =>
      useAuthStore.getState().setSession("token", { ...teacherUser, fullName: "" }),
    );
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Chào buổi sáng, bạn",
    );
  });
  it("retries a home card without treating the independent live card as failed", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () => {
        asked++;
        return error();
      }),
    );
    board();
    const chart = region("Bài nộp");
    await within(chart).findByRole("alert");
    expect(
      await within(region("Bài đang mở")).findByText("Unit 4 · Listening"),
    ).toBeInTheDocument();
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () => {
        asked++;
        return contractJson("/teacher/dashboard", "get", 200, dashboard23());
      }),
    );
    fireEvent.click(within(chart).getByRole("button", { name: "Thử lại" }));
    await screen.findByRole("link", { name: /Bài bị gắn cờ/ });
    expect(asked).toBe(2);
    expect(
      reads.filter((path) => path.startsWith("/teacher/assignments")),
    ).toHaveLength(1);
  });
  it("renders real readings and permission-backed destinations without retired or wide requests", async () => {
    board();
    const flagged = await screen.findByRole("link", { name: /Bài bị gắn cờ/ });
    expect(flagged).toHaveAttribute(
      "href",
      `/teacher/assignments/${DASHBOARD23_IDS.assignment}?tab=students&attempt=${DASHBOARD23_IDS.attempt}`,
    );
    expect(screen.getByRole("link", { name: /Đang làm bài/ })).toHaveAttribute(
      "href",
      `/teacher/assignments/${DASHBOARD23_IDS.taking}?tab=students`,
    );
    expect(screen.getByRole("link", { name: /Đóng trong 24 giờ/ })).toHaveAttribute(
      "href",
      `/teacher/assignments/${DASHBOARD23_IDS.assignment}?tab=students`,
    );
    expect(screen.getByRole("link", { name: /Chờ chấm/ })).toHaveAttribute(
      "href",
      "/teacher/grading",
    );
    expect(
      within(region("Bài đang mở")).getByText("2 bài chờ chấm"),
    ).toBeInTheDocument();
    expect(screen.queryByText("9 bài chờ chấm")).toBeNull();
    expect(screen.queryByText("999")).toBeNull();
    expect(reads).toEqual([
      "/teacher/dashboard?range=14d",
      "/teacher/assignments?status=open&limit=10",
    ]);
  });
  it("starts no read, write or cached private presentation without the Teacher workspace", async () => {
    useAuthStore.getState().setSession("token", { ...teacherUser, workspaces: [] });
    const client = new QueryClient();
    client.setQueryData(dashboardKeys.home("14d"), dashboard23());
    board("", client);
    await act(() => Promise.resolve());
    expect(reads).toEqual([]);
    expect(creations).toBe(0);
    expect(screen.queryByRole("heading")).toBeNull();
  });
  it("leaves denied positive counts visible with no arrow, link or added tabstop", async () => {
    useAuthStore.getState().setSession("token", { ...teacherUser, permissions: [] });
    board();
    await screen.findByText("Bài bị gắn cờ");
    expect(screen.queryByRole("link", { name: /Bài bị gắn cờ|Chờ chấm/ })).toBeNull();
    expect(
      screen.getByText("Bài bị gắn cờ").closest('div[class~="group/kpi"]'),
    ).not.toHaveAttribute("tabindex");
    expect(screen.queryByRole("button", { name: "Đề thi mới" })).toBeNull();
    expect(screen.queryByRole("link", { name: /^Giao bài$/ })).toBeNull();
    expect(screen.getByRole("link", { name: /Đang làm bài/ })).toBeInTheDocument();
  });
  it("keeps null destinations and confirmed zero work noninteractive", async () => {
    const data = dashboard23();
    data.newestFlaggedAttempt = null;
    data.takingNow.assignmentId = null;
    data.awaitingGrading = 0;
    data.closingSoon = 0;
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () =>
        contractJson("/teacher/dashboard", "get", 200, data),
      ),
    );
    board();
    await screen.findByText("Bài bị gắn cờ");
    expect(
      screen.queryByRole("link", {
        name: /Bài bị gắn cờ|Chờ chấm|Đang làm bài|Đóng trong 24 giờ/,
      }),
    ).toBeNull();
    expect(screen.queryByText("Xem lại")).toBeNull();
    expect(screen.getByText("3")).toBeInTheDocument();
  });
  it("allows intervention-only flagged review and uses its exact destination URL", async () => {
    useAuthStore.getState().setSession("token", {
      ...teacherUser,
      permissions: ["teaching.attempts.intervene"],
    });
    const { router } = board();
    const link = await screen.findByRole("link", { name: /Bài bị gắn cờ/ });
    fireEvent.click(link);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        `/teacher/assignments/${DASHBOARD23_IDS.assignment}`,
      ),
    );
    expect(router.state.location.search).toBe(
      `?tab=students&attempt=${DASHBOARD23_IDS.attempt}`,
    );
    expect(screen.queryByRole("link", { name: /Chờ chấm/ })).toBeNull();
  });
  it("preserves the original default helper request and forwards explicit range independently", async () => {
    await getDashboard();
    await getDashboard(undefined, "7d");
    expect(reads).toEqual(["/teacher/dashboard", "/teacher/dashboard?range=7d"]);
  });
  it("changes only range, retains query focus and prevents old-range totals under a new label", async () => {
    let release!: () => void;
    let cancelled = false;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/teacher/dashboard`, async ({ request }) => {
        request.signal.addEventListener("abort", () => {
          cancelled = true;
        });
        const range = new URL(request.url).searchParams.get("range") as DashboardRange;
        if (range === "7d") await held;
        return contractJson("/teacher/dashboard", "get", 200, dashboard23(range));
      }),
    );
    const { router } = board("?other=A&other=B#chart");
    await screen.findByText(/14 trong 14 ngày qua/);
    const seven = screen.getByRole("button", { name: "7 ngày" });
    seven.focus();
    fireEvent.click(seven);
    await waitFor(() =>
      expect(router.state.location.search).toBe("?other=A&other=B&range=7d"),
    );
    expect(router.state.location.hash).toBe("#chart");
    expect(seven).toHaveFocus();
    expect(screen.queryByText(/14 trong 7 ngày qua/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "30 ngày" }));
    await screen.findByText(/30 trong 30 ngày qua/);
    await act(async () => {
      release();
      await held;
    });
    expect(screen.getByRole("button", { name: "30 ngày" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.queryByText(/7 trong 30 ngày qua/)).toBeNull();
    expect(cancelled).toBe(true);
  });
  it.each(["", "?range=invalid", "?range=14d"])(
    "does not unnecessarily navigate default range %s",
    async (search) => {
      const { router } = board(search);
      await screen.findByText(/14 trong 14 ngày qua/);
      expect(router.state.location.search).toBe(search);
      expect(router.state.historyAction).toBe("POP");
    },
  );
  it("creates once on immediate double activation and opens the builder only after success", async () => {
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post(`${BASE}/teacher/tests`, async () => {
        creations++;
        await held;
        return contractJson("/teacher/tests", "post", 201, dashboard23Draft());
      }),
    );
    const { router } = board();
    const button = screen.getByRole("button", { name: "Đề thi mới" });
    act(() => {
      button.click();
      button.click();
    });
    await waitFor(() => expect(creations).toBe(1));
    expect(button).toBeDisabled();
    expect(router.state.location.pathname).toBe("/teacher");
    await act(async () => {
      release();
      await held;
    });
    await screen.findByText("builder");
  });
  it("retains a localized creation failure and permits a real retry without navigating on failure", async () => {
    server.use(
      http.post(`${BASE}/teacher/tests`, () => {
        creations++;
        return error();
      }),
    );
    const { router } = board();
    fireEvent.click(screen.getByRole("button", { name: "Đề thi mới" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không thể tạo đề thi");
    expect(router.state.location.pathname).toBe("/teacher");
    server.use(
      http.post(`${BASE}/teacher/tests`, () => {
        creations++;
        return contractJson("/teacher/tests", "post", 201, dashboard23Draft());
      }),
    );
    fireEvent.click(within(alert).getByRole("button", { name: "Thử lại" }));
    await screen.findByText("builder");
    expect(creations).toBe(2);
  });
  it("preserves null score as unavailable, true zero score as zero and every calendar day", async () => {
    const data = dashboard23("7d");
    data.submissions.averagePercent = null;
    data.submissions.days = data.submissions.days.map((day) => ({ ...day, count: 0 }));
    data.submissions.total = 0;
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () =>
        contractJson("/teacher/dashboard", "get", 200, data),
      ),
    );
    const { client } = board("?range=7d");
    await screen.findByText(/Chưa có điểm/);
    expect(within(region("Bài nộp")).getAllByRole("row")).toHaveLength(8);
    data.submissions.averagePercent = 0;
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    expect(await screen.findByText(/điểm trung bình 0%/)).toBeInTheDocument();
  });
  it.each([
    [0, "Chưa có bài giao nào."],
    [3, "Chưa có bài đang mở."],
  ])("distinguishes global %s from live emptiness", async (all, message) => {
    server.use(
      http.get(`${BASE}/teacher/assignments`, () =>
        contractJson(
          "/teacher/assignments",
          "get",
          200,
          dashboard23Assignments(Number(all), false),
        ),
      ),
    );
    board();
    expect(
      await within(region("Bài đang mở")).findByText(String(message)),
    ).toBeInTheDocument();
  });
  it("retries the independently failed live card without refetching home", async () => {
    let tries = 0;
    server.use(
      http.get(`${BASE}/teacher/assignments`, () => {
        tries++;
        return error();
      }),
    );
    board();
    await screen.findByRole("link", { name: /Bài bị gắn cờ/ });
    const live = region("Bài đang mở");
    await within(live).findByRole("alert");
    server.use(
      http.get(`${BASE}/teacher/assignments`, () => {
        tries++;
        return contractJson(
          "/teacher/assignments",
          "get",
          200,
          dashboard23Assignments(),
        );
      }),
    );
    fireEvent.click(within(live).getByRole("button", { name: "Thử lại" }));
    await within(live).findByText("Unit 4 · Listening");
    expect(tries).toBe(2);
    expect(reads.filter((path) => path.startsWith("/teacher/dashboard"))).toHaveLength(
      1,
    );
  });
  it("shows unknown hints and counts without inventing zeros or progress", async () => {
    const data = dashboard23();
    delete data.waitingStudents;
    delete data.closingSoon;
    data.nextClosing = null;
    const list = dashboard23Assignments();
    delete list.items[0]!.targetCount;
    delete list.items[0]!.submittedCount;
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () =>
        contractJson("/teacher/dashboard", "get", 200, data),
      ),
      http.get(`${BASE}/teacher/assignments`, () =>
        contractJson("/teacher/assignments", "get", 200, list),
      ),
    );
    board();
    await screen.findByText("Chưa có số liệu bài nộp");
    expect(within(region("Bài đang mở")).queryByRole("progressbar")).toBeNull();
    expect(screen.queryByText(/0 học viên ·/)).toBeNull();
  });
  it("keeps calendar cells keyed through refetch and hides alternate phone labels", async () => {
    viewport("phone");
    const { client } = board("?range=7d");
    await screen.findByText(/7 trong 7 ngày qua/);
    const chart = within(region("Bài nộp")).getByRole("table");
    const cells = within(chart).getAllByRole("row").slice(1);
    await act(() => client.invalidateQueries({ queryKey: dashboardKeys.home("7d") }));
    expect(within(chart).getAllByRole("row").slice(1)).toEqual(cells);
    const visual =
      chart.parentElement!.parentElement!.querySelector('[aria-hidden="true"]')!;
    expect(within(visual as HTMLElement).getByText("19")).toBeInTheDocument();
    expect(within(visual as HTMLElement).queryByText("18")).toBeNull();
  });
});
