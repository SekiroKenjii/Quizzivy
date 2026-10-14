import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import type { RouteObject } from "react-router";
import AdminLayout from "@/layouts/AdminLayout";
import { adminNavFor } from "@/layouts/adminNav";
import type { TeacherHandle } from "@/layouts/shell/handle";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import type { components } from "@/lib/api/schema";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser, studentUser, teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { viewport } from "@tests/support/viewport";
import { BASE, renderRoutes } from "./support";
import "@/lib/i18n";

type Credits = components["schemas"]["AiAdminCredits"];
type User = components["schemas"]["CurrentUser"];

const NAV = "Điều hướng chính";
const TRAIL = "Đường dẫn";

const CRUMB = { key: "adminShell.crumb" } as const;
const OVERVIEW: TeacherHandle = {
  crumb: [CRUMB, { key: "adminShell.nav.overview" }],
  width: 1320,
} as TeacherHandle;
const USERS: TeacherHandle = {
  crumb: [CRUMB, { key: "adminShell.nav.users" }],
  width: 1320,
} as TeacherHandle;
const SETTINGS: TeacherHandle = {
  crumb: [CRUMB, { key: "adminShell.nav.settings" }],
  width: 1080,
} as TeacherHandle;

const pages: RouteObject[] = [
  { index: true, handle: OVERVIEW, element: <p>trang tổng quan</p> },
  { path: "users", handle: USERS, element: <p>trang người dùng</p> },
  { path: "settings/:section?", handle: SETTINGS, element: <p>trang cài đặt</p> },
];

const LABELS = [
  "Tổng quan",
  "Người dùng",
  "Vai trò & quyền",
  "Lớp học",
  "Tín dụng AI",
  "Nhật ký kiểm tra",
  "Tài liệu API",
  "Cài đặt",
];

const adminOnly: User = { ...adminUser, workspaces: ["admin"] };
const usersOnly: User = {
  ...studentUser,
  permissions: ["people.users.manage"],
  workspaces: ["admin"],
};

const credits = (waiting: number): Credits => ({
  month: "2026-10-01",
  pool: { plan: 1000, added: 0, used: 100, reserved: 0, total: 1000, left: 900 },
  daysToGo: 17,
  onPace: 200,
  withinPool: true,
  allowances: 300,
  active: 2,
  idle: 0,
  teachers: [],
  requests: Array.from({ length: waiting }, (_, i) => ({
    id: `019535d9-3df7-79fb-b466-fa907fa17f${(10 + i).toString(16).padStart(2, "0")}`,
    userId: "019535d9-3df7-79fb-b466-fa907fa17fa0",
    teacherName: "Trần Thị Bình",
    month: "2026-10-01",
    amount: 1000,
    reason: "Chấm bài thi cuối kỳ",
    createdAt: "2026-10-13T08:00:00Z",
    used: 100,
    allowance: 100,
  })),
  rules: {
    poolCredits: 1000,
    defaultAllowance: 1000,
    warnPercent: 80,
    onTeacherLimit: "request",
  },
});

function serveCredits(waiting: number, ai = true) {
  const seen = { asked: 0, status: 0 };
  server.use(
    http.get(`${BASE}/public/status`, () => {
      seen.status += 1;
      return contractJson("/public/status", "get", 200, { maintenance: null, ai });
    }),
    http.get(`${BASE}/admin/ai/credits`, () => {
      seen.asked += 1;
      return contractJson("/admin/ai/credits", "get", 200, credits(waiting));
    }),
  );
  return seen;
}

function failCredits() {
  server.use(
    http.get(`${BASE}/public/status`, () =>
      contractJson("/public/status", "get", 200, { maintenance: null, ai: true }),
    ),
    http.get(`${BASE}/admin/ai/credits`, () =>
      HttpResponse.json(
        {
          error: {
            code: "INTERNAL",
            message: "Máy chủ gặp lỗi.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        },
        { status: 500 },
      ),
    ),
  );
}

function renderAdmin(at: string, user: User = adminUser) {
  return renderRoutes(
    at,
    [{ path: "/admin", element: <AdminLayout />, children: pages }],
    user,
  );
}

const sidebar = () => document.getElementById("console-sidebar");
const nav = () => screen.getByRole("navigation", { name: NAV });
const links = () => within(nav()).getAllByRole("link");
const badges = () => document.querySelectorAll("[data-slot='nav-count']");
const toggle = (name: string) => screen.getByRole("button", { name });
const AI = /^Tín dụng AI\s*2 yêu cầu thêm tín dụng đang chờ$/;

beforeEach(() => {
  viewport(1280);
  document.title = "Quizzivy";
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  writeThemePreference("light");
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("the Admin sidebar for each audience", () => {
  it("lists the eight destinations under People, Teaching and System for an Admin, the current one lit", async () => {
    renderAdmin("/admin/users");
    await screen.findByRole("main");

    expect(links().map((link) => link.textContent)).toEqual(LABELS);
    for (const heading of ["Mọi người", "Giảng dạy", "Hệ thống"])
      expect(within(nav()).getByText(heading)).toBeInTheDocument();
    expect(within(nav()).getByRole("link", { name: "Người dùng" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(within(nav()).getByRole("link", { name: "Tổng quan" })).not.toHaveAttribute(
      "aria-current",
    );
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(sidebar()).toHaveClass("w-62", "bg-sidebar");
  });

  it("lights Overview at the console's own address and nothing on an export", async () => {
    renderAdmin("/admin");
    await screen.findByRole("main");
    expect(within(nav()).getByRole("link", { name: "Tổng quan" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("opens only Overview and Users for a custom role holding only people.users.manage", async () => {
    renderAdmin("/admin", usersOnly);
    await screen.findByRole("main");

    expect(links().map((link) => link.textContent)).toEqual([
      "Tổng quan",
      "Người dùng",
    ]);
    expect(screen.queryByText("Giảng dạy")).toBeNull();
    expect(screen.queryByText("Hệ thống")).toBeNull();
  });

  it("opens nothing for a teacher, who holds no console", async () => {
    expect(adminNavFor(teacherUser)).toEqual([]);
    expect(adminNavFor(null)).toEqual([]);
    renderAdmin("/admin", teacherUser);
    await screen.findByRole("main");
    expect(
      screen.queryAllByRole("link", { name: /Tổng quan|Người dùng/ }),
    ).toHaveLength(0);
  });

  it("heads the sidebar with the product, the Admin console and the shield, and puts the account at the foot", async () => {
    renderAdmin("/admin");
    await screen.findByRole("main");

    expect(within(sidebar()!).getByText("Quizzivy")).toBeInTheDocument();
    expect(within(sidebar()!).getByText("Bảng quản trị")).toBeInTheDocument();
    expect(sidebar()!.querySelector("svg.lucide-shield")).not.toBeNull();
    expect(within(sidebar()!).getByText("Thuong")).toBeInTheDocument();
  });
});

describe("the workspace switcher in both layouts", () => {
  it("offers the teacher console from the Admin's block for a user who holds both", async () => {
    const user = userEvent.setup();
    renderAdmin("/admin");
    await screen.findByRole("main");

    await user.click(within(sidebar()!).getByRole("button", { name: /Quizzivy/ }));
    const rows = within(screen.getByRole("menu")).getAllByRole("menuitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "Không gian giáo viên",
      "Bảng quản trị",
    ]);
    expect(rows[0]).toHaveAttribute("href", "/teacher");
    expect(rows[1]).toHaveAttribute("aria-current", "true");
  });

  it("is a label where the user holds only the Admin console", async () => {
    renderAdmin("/admin", adminOnly);
    await screen.findByRole("main");

    expect(within(sidebar()!).queryByRole("button", { name: /Quizzivy/ })).toBeNull();
    expect(sidebar()!.querySelector("[data-slot='workspace']")!.tagName).toBe("DIV");
  });
});

describe("the Admin top bar", () => {
  it("has the toggle, the trail, the theme switch and the teacher view, and no search, bell or language", async () => {
    renderAdmin("/admin/users");
    await screen.findByRole("main");
    const bar = screen.getByRole("banner");

    expect(
      within(bar).getByRole("button", { name: "Thu gọn thanh bên" }),
    ).toBeInTheDocument();
    expect(within(bar).getByRole("button", { name: /Chế độ tối/ })).toBeInTheDocument();
    const view = within(bar).getByRole("link", { name: "Chế độ giáo viên" });
    expect(view).toHaveAttribute("href", "/teacher");
    expect(view).toHaveTextContent("Chế độ giáo viên");
    expect(within(bar).queryByRole("button", { name: "Tìm kiếm…" })).toBeNull();
    expect(within(bar).queryByRole("button", { name: /Thông báo/ })).toBeNull();
    expect(
      within(bar).queryByRole("button", { name: /Ngôn ngữ|English|Tiếng/ }),
    ).toBeNull();
    const trail = within(screen.getByRole("navigation", { name: TRAIL })).getAllByRole(
      "listitem",
    );
    expect(trail.map((item) => item.textContent)).toEqual(["Quản trị", "Người dùng"]);
  });

  it("leaves the teacher view out for a user who holds no teacher workspace", async () => {
    renderAdmin("/admin", adminOnly);
    await screen.findByRole("main");

    expect(screen.queryByRole("link", { name: "Chế độ giáo viên" })).toBeNull();
  });

  it("names the tab after the page", async () => {
    renderAdmin("/admin/users");
    await screen.findByText("trang người dùng");
    await waitFor(() => expect(document.title).toBe("Người dùng · Quizzivy"));
  });

  it("frames the page at 1320px, and at 1080px for Settings", async () => {
    const { router } = renderAdmin("/admin/users");
    const page = async () =>
      (await screen.findByRole("main")).querySelector<HTMLElement>(
        "[data-slot='page']",
      )!;

    expect((await page()).style.maxWidth).toBe("1320px");
    await act(() => router.navigate("/admin/settings/sign-in"));
    await screen.findByText("trang cài đặt");
    expect((await page()).style.maxWidth).toBe("1080px");
  });
});

describe("the AI credits badge", () => {
  it("says how many requests wait, in the accent, and counts them for a screen reader", async () => {
    serveCredits(2);
    renderAdmin("/admin");

    const ai = await within(nav()).findByRole("link", { name: AI });
    const badge = ai.querySelector("[data-slot='nav-count']")!;
    expect(within(ai).getByText("2")).toHaveAttribute("aria-hidden", "true");
    expect(badge).toHaveClass("bg-brand", "text-brand-fg");
    expect(badges()).toHaveLength(1);
  });

  it("is hidden at zero", async () => {
    const seen = serveCredits(0);
    renderAdmin("/admin");

    await waitFor(() => expect(seen.asked).toBe(1));
    await within(nav()).findByRole("link", { name: "Tín dụng AI" });
    expect(badges()).toHaveLength(0);
  });

  it("is not asked for while the server has no assistant", async () => {
    const seen = serveCredits(2, false);
    renderAdmin("/admin");

    await waitFor(() => expect(seen.status).toBe(1));
    await within(nav()).findByRole("link", { name: "Tín dụng AI" });
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
    expect(seen.asked).toBe(0);
    expect(badges()).toHaveLength(0);
  });

  it("is not asked for by a role that lacks ai.credits.manage", async () => {
    const seen = serveCredits(2);
    renderAdmin("/admin", usersOnly);

    await screen.findByRole("main");
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
    expect(seen.asked).toBe(0);
    expect(seen.status).toBe(0);
    expect(screen.queryByRole("link", { name: /Tín dụng AI/ })).toBeNull();
  });

  it("draws none, and no error, when the request fails", async () => {
    failCredits();
    renderAdmin("/admin");

    await within(nav()).findByRole("link", { name: "Tín dụng AI" });
    await act(() => Promise.resolve());
    expect(badges()).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(links().map((link) => link.textContent)).toEqual(LABELS);
  });

  it("is left off while the sidebar is collapsed, and comes back with it", async () => {
    const user = userEvent.setup();
    serveCredits(2);
    renderAdmin("/admin");
    await within(nav()).findByRole("link", { name: AI });

    await user.click(toggle("Thu gọn thanh bên"));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(badges()).toHaveLength(0);
    expect(
      within(nav()).getByRole("link", { name: /Tín dụng AI/ }),
    ).toBeInTheDocument();

    await user.click(toggle("Mở rộng thanh bên"));
    expect(await within(nav()).findByRole("link", { name: AI })).toBeInTheDocument();
  });

  it("asks again every minute", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const seen = serveCredits(2);
    renderAdmin("/admin");
    await waitFor(() => expect(seen.asked).toBe(1));

    await act(() => vi.advanceTimersByTimeAsync(61_000));
    await waitFor(() => expect(seen.asked).toBe(2));
  });
});

describe("collapsing the Admin sidebar", () => {
  it("leaves icons that keep their names, 60px wide, and a separator between the groups", async () => {
    const user = userEvent.setup();
    renderAdmin("/admin");
    await screen.findByRole("main");

    await user.click(toggle("Thu gọn thanh bên"));

    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(sidebar()).toHaveClass("w-15");
    expect(
      links().map((link) => link.getAttribute("aria-label") ?? link.textContent),
    ).toEqual(LABELS);
    expect(within(nav()).queryByText("Mọi người")).toBeNull();
    expect(within(nav()).getAllByRole("separator")).toHaveLength(3);
  });
});

describe("the Admin console below 768", () => {
  beforeEach(() => {
    viewport(360);
  });

  it("has no sidebar in the page's flow and a toggle that opens the menu", async () => {
    renderAdmin("/admin");
    await screen.findByRole("main");

    expect(sidebar()).toBeNull();
    expect(toggle("Mở menu")).toHaveAttribute("aria-expanded", "false");
    expect(toggle("Mở menu")).toHaveAttribute("aria-controls", "console-sidebar");
  });

  it("opens a 272px drawer with labels and the badge, whatever is stored", async () => {
    const user = userEvent.setup();
    writeSidebarState("collapsed");
    serveCredits(2);
    renderAdmin("/admin");
    await screen.findByRole("main");

    await user.click(toggle("Mở menu"));
    const drawer = await screen.findByRole("dialog", { name: NAV });

    expect(drawer).toBe(sidebar());
    expect(drawer).toHaveClass("w-68", "bg-sidebar");
    expect(within(drawer).getByText("Bảng quản trị")).toBeInTheDocument();
    expect(within(drawer).getByText("Hệ thống")).toBeInTheDocument();
    expect(await within(drawer).findByRole("link", { name: AI })).toBeInTheDocument();
  });

  it("closes when a destination is chosen", async () => {
    const user = userEvent.setup();
    const { router } = renderAdmin("/admin");
    await screen.findByRole("main");

    await user.click(toggle("Mở menu"));
    const drawer = await screen.findByRole("dialog", { name: NAV });
    await user.click(within(drawer).getByRole("link", { name: "Người dùng" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.pathname).toBe("/admin/users");
  });

  it("shows the teacher view as an icon alone, still named", async () => {
    renderAdmin("/admin");
    await screen.findByRole("main");

    const view = screen.getByRole("link", { name: "Chế độ giáo viên" });
    expect(view).toHaveClass("max-[767px]:w-8.5");
    expect(view.querySelector("span")).toHaveClass("hidden", "min-[768px]:inline");
  });
});
