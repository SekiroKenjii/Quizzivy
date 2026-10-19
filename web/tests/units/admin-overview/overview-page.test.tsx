import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import OverviewPage from "@/features/admin-overview/pages/admin/OverviewPage";
import type { AdminOverview } from "@/features/admin-overview/api";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";
import { entry } from "@tests/units/audit/support";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

const DAYS = Array.from({ length: 14 }, (_, index) => ({
  date: `2026-10-${String(6 + index).padStart(2, "0")}`,
  students: 100 + index,
  staff: 10,
}));

const FULL: AdminOverview = {
  organizationName: "Quizzivy English Centre",
  people: {
    total: 312,
    activeThisWeek: 264,
    teachers: 9,
    teachersDisabled: 1,
    students: 300,
    studentsNewThisMonth: 18,
  },
  classes: { active: 14, archived: 1 },
  dailyActive: { days: DAYS, average: 184 },
  attention: [
    { kind: "must_change_password", count: 4, detail: null },
    {
      kind: "never_signed_in",
      count: 11,
      detail: { oldestCreatedAt: "2026-09-01T00:00:00Z" },
    },
    {
      kind: "disabled_teacher_classes",
      count: 1,
      detail: { className: "Summer Camp 2026", teacherName: "Lý Thanh Vân" },
    },
    { kind: "storage", count: 1, detail: { mediaPercent: 85 } },
    {
      kind: "ai_credit_requests",
      count: 2,
      detail: { teacherName: "Trần Thị Bình", used: 900, allowance: 1000 },
    },
  ],
  recentChanges: [1, 2, 3, 4, 5].map((id) => entry({ id })),
  storage: {
    plan: null,
    seats: { used: 312, limit: 400 },
    teachers: { used: 9, limit: null },
    media: { bytes: 1_342_177_280, limit: 5_368_709_120 },
    imports: { bytes: 644_245_094, count: 7, limit: 2_147_483_648 },
    attemptsKept: { count: 18420 },
    aiCredits: { used: 400, pool: 1000 },
  },
};

function serve(overview: AdminOverview) {
  const seen = { asked: 0 };
  server.use(
    http.get(`${BASE}/admin/overview`, () => {
      seen.asked += 1;
      return contractJson("/admin/overview", "get", 200, overview);
    }),
  );
  return seen;
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/admin", element: <OverviewPage /> }], {
    initialEntries: ["/admin"],
  });
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

const loaded = () => screen.findByText("Quizzivy English Centre · 312 người");

beforeEach(() => {
  contentWidth(1200);
});

describe("the Overview's header and figures", () => {
  it("names the centre and the people in it", async () => {
    serve(FULL);
    renderPage();
    expect(
      await screen.findByText("Quizzivy English Centre · 312 người"),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Tổng quan" })).toBeVisible();
  });

  it("links four tiles to where their figures live", async () => {
    serve(FULL);
    renderPage();
    const active = await screen.findByRole("link", { name: /Hoạt động tuần này/ });
    expect(active).toHaveAttribute("href", "/admin/users");
    expect(active).toHaveTextContent("264");
    expect(active).toHaveTextContent("85% trên 312 người");
    expect(screen.getByRole("link", { name: /Giáo viên.*9/ })).toHaveAttribute(
      "href",
      "/admin/users?tab=teachers",
    );
    expect(
      screen.getByRole("link", { name: /Giáo viên.*1 bị vô hiệu hóa/ }),
    ).toBeVisible();
    const students = screen.getByRole("link", { name: /Học viên.*300/ });
    expect(students).toHaveAttribute("href", "/admin/users?tab=students");
    expect(students).toHaveTextContent("+18 trong tháng này");
    const classes = screen.getByRole("link", { name: /Lớp học.*14/ });
    expect(classes).toHaveAttribute("href", "/admin/classes");
    expect(classes).toHaveTextContent("1 đã lưu trữ");
  });

  it("lays the tiles in four columns from 900 and two below", async () => {
    serve(FULL);
    contentWidth(899);
    const view = renderPage();
    const link = await screen.findByRole("link", { name: /Hoạt động tuần này/ });
    expect(link.parentElement).toHaveStyle({
      gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    });
    view.unmount();
    contentWidth(900);
    renderPage();
    const wide = await screen.findByRole("link", { name: /Hoạt động tuần này/ });
    expect(wide.parentElement).toHaveStyle({
      gridTemplateColumns: "repeat(4, minmax(0, 1fr))",
    });
  });
});

describe("the daily active card", () => {
  it("states the average and offers the days as a table with a legend", async () => {
    serve(FULL);
    renderPage();
    await loaded();
    const card = screen.getByRole("region", { name: "Người hoạt động mỗi ngày" });
    expect(card).toHaveTextContent("14 ngày qua · 184 người mỗi ngày");
    expect(within(card).getByText("184").tagName).toBe("STRONG");
    const table = within(card).getByRole("table");
    expect(within(table).getAllByRole("row")).toHaveLength(15);
    expect(
      within(table)
        .getAllByRole("columnheader")
        .map((header) => header.textContent),
    ).toEqual(["Ngày", "Học viên", "Giáo viên", "Tổng"]);
    expect(within(card).getAllByText("Học viên", { selector: "li" })).toHaveLength(1);
    expect(card.querySelector("[title$='06/10/2026']")).toHaveAttribute(
      "title",
      "110 người (100 học viên, 10 giáo viên) · 06/10/2026",
    );
  });
});

describe("the needs-attention card", () => {
  it("sends each row to where it is dealt with", async () => {
    serve(FULL);
    renderPage();
    await loaded();
    const card = screen.getByRole("region", { name: "Cần xử lý" });
    const hrefs = within(card)
      .getAllByRole("link")
      .map((link) => link.getAttribute("href"));
    expect(hrefs).toEqual([
      "/admin/users?status=must_change_password",
      "/admin/users?status=never_signed_in",
      "/admin/classes?teacher=disabled",
      "/admin/settings/plan",
      "/admin/ai",
    ]);
    expect(card).toHaveTextContent("4 người cần đổi mật khẩu");
    expect(card).toHaveTextContent("Summer Camp 2026 · Lý Thanh Vân");
    expect(card).toHaveTextContent("Dung lượng: phương tiện 85%");
    expect(card).toHaveTextContent("2 giáo viên xin thêm tín dụng AI");
  });

  it("says so when nothing needs doing", async () => {
    serve({ ...FULL, attention: [] });
    renderPage();
    expect(await screen.findByText("Không có gì cần xử lý.")).toBeInTheDocument();
  });
});

describe("recent changes and storage", () => {
  it("lists five entries and links the log", async () => {
    serve(FULL);
    renderPage();
    await loaded();
    const card = screen.getByRole("region", { name: "Thay đổi gần đây" });
    expect(within(card).getAllByRole("listitem")).toHaveLength(5);
    expect(within(card).getAllByText("Giàu Bùi")).toHaveLength(5);
    expect(within(card).getByRole("link", { name: "Nhật ký" })).toHaveAttribute(
      "href",
      "/admin/audit",
    );
  });

  it("draws the meters, and the AI credits only while the server reports them", async () => {
    serve(FULL);
    const view = renderPage();
    await loaded();
    const card = screen.getByRole("region", { name: "Dung lượng và mức dùng" });
    expect(within(card).getAllByRole("meter")).toHaveLength(4);
    expect(within(card).getByRole("meter", { name: "Phương tiện" })).toHaveAttribute(
      "aria-valuetext",
      "1.25\u00a0GB trên 5\u00a0GB",
    );
    expect(card).toHaveTextContent("18.420 lượt");
    expect(card).toHaveTextContent("400 trên 1.000 trong tháng này");
    view.unmount();

    server.resetHandlers();
    serve({ ...FULL, storage: { ...FULL.storage!, aiCredits: null } });
    renderPage();
    await screen.findByText("Quizzivy English Centre · 312 người");
    const bare = screen.getByRole("region", { name: "Dung lượng và mức dùng" });
    expect(within(bare).getAllByRole("meter")).toHaveLength(3);
    expect(bare).not.toHaveTextContent("Tín dụng AI");
  });
});

describe("a section the caller cannot open", () => {
  it("is left out whole", async () => {
    serve({
      ...FULL,
      people: null,
      classes: null,
      dailyActive: null,
      recentChanges: null,
      storage: null,
      attention: [],
    });
    renderPage();
    expect(await screen.findByText("Không có gì cần xử lý.")).toBeInTheDocument();
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
    for (const name of [
      "Người hoạt động mỗi ngày",
      "Thay đổi gần đây",
      "Dung lượng và mức dùng",
    ])
      expect(screen.queryByRole("region", { name })).toBeNull();
    expect(screen.queryByRole("link", { name: /Hoạt động tuần này/ })).toBeNull();
  });
});

describe("loading and failing", () => {
  it("shows a skeleton in each card while the first read is out", () => {
    server.use(http.get(`${BASE}/admin/overview`, () => new Promise(() => undefined)));
    renderPage();
    expect(screen.getAllByRole("status", { name: "Đang tải…" })).toHaveLength(4);
  });

  it("gives every card an error with a retry, and one retry reads again", async () => {
    const user = userEvent.setup({ delay: null });
    let failing = true;
    const seen = { asked: 0 };
    server.use(
      http.get(`${BASE}/admin/overview`, () => {
        seen.asked += 1;
        return failing
          ? HttpResponse.json(
              {
                error: {
                  code: "INTERNAL",
                  message: "Máy chủ gặp lỗi.",
                  requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
                },
              },
              { status: 500 },
            )
          : contractJson("/admin/overview", "get", 200, FULL);
      }),
    );
    renderPage();
    const retries = await screen.findAllByRole("button", { name: "Thử lại" });
    expect(retries).toHaveLength(4);

    failing = false;
    await user.click(retries[0]!);
    expect(
      await screen.findByText("Quizzivy English Centre · 312 người"),
    ).toBeInTheDocument();
    expect(seen.asked).toBe(2);
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });
});
