import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { focusManager } from "@tanstack/react-query";
import { Link } from "react-router";
import { useLayoutEffect, useRef } from "react";
import { http } from "msw";
import StudentLayout from "@/layouts/StudentLayout";
import StudentHomePage from "@/features/assignments/pages/StudentHomePage";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import StudentClassesPage from "@/features/classes/pages/StudentClassesPage";
import StudentSettingsPage from "@/features/auth/pages/StudentSettingsPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import { useAuthStore } from "@/stores/auth";
import { skeletonFor } from "@/app/boot/handoff";
import { writeThemePreference } from "@/lib/theme";
import { ASSIGNMENT, BASE, card, detail, renderAt } from "./support";
import "@/lib/i18n";

const flags = vi.hoisted(() => ({
  notifications: false,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
}));
vi.mock("@/app/modules", () => ({ modules: flags }));

const SETTINGS = { detail: { titleKey: "nav.settings", back: "/app" } };
const NAV = "Điều hướng chính";

const CLASS = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation",
  description: "Thứ 3 và thứ 5, 19:30–21:00.",
  teacherName: "Cô Thương",
  joinedAt: "2026-06-01T00:00:00Z",
};

function serveStudent(
  sections: { dueNow?: unknown[]; upcoming?: unknown[]; completed?: unknown[] },
  classes: unknown[] = [CLASS],
) {
  server.use(
    http.get(`${BASE}/app/assignments`, () =>
      contractJson("/app/assignments", "get", 200, {
        dueNow: sections.dueNow ?? [],
        upcoming: sections.upcoming ?? [],
        completed: sections.completed ?? [],
      }),
    ),
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items: classes }),
    ),
  );
}

const shell = (
  path: string,
  page: React.ReactElement,
  handle?: object,
  route: string = path,
) =>
  renderAt(path, [
    {
      element: <StudentLayout />,
      children: [{ path: route, element: page, ...(handle ? { handle } : {}) }],
    },
  ]);

beforeEach(() => {
  viewport("desktop");
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  flags.learn = false;
  flags.grades = false;
  writeThemePreference("light");
});

function TakesFocus() {
  const field = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => field.current?.focus(), []);
  return <input ref={field} aria-label="Mã lớp" />;
}

function destinations() {
  return within(screen.getByRole("navigation", { name: NAV }))
    .getAllByRole("link")
    .map((link) => link.textContent);
}

describe("the shell from 768", () => {
  it("draws the logo, the destinations that exist and the account button", async () => {
    serveStudent({});
    shell("/app", <p>trang</p>);
    await screen.findByRole("navigation", { name: NAV });

    expect(destinations()).toEqual(["Trang chủ", "Lớp"]);
    const nav = within(screen.getByRole("navigation", { name: NAV }));
    expect(nav.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(nav.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(nav.getByRole("link", { name: "Lớp" })).toHaveAttribute(
      "href",
      "/app/classes",
    );
    expect(nav.getByRole("link", { name: "Lớp" })).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("link", { name: "Trang chủ Quizzivy" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    ).toHaveTextContent("AN");
    expect(screen.getAllByRole("navigation")).toHaveLength(1);
    expect(screen.queryByRole("link", { name: "Tôi" })).toBeNull();
  });

  it("is a deck surface and opens the deck's account menu", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    serveStudent({});
    shell("/app", <p>trang</p>);
    const account = await screen.findByRole("button", {
      name: "Tài khoản của Nguyễn Văn An",
    });
    expect(account.closest("[data-scale='deck']")).toBe(
      screen.getByRole("banner").parentElement,
    );
    expect(screen.getByRole("main").closest("[data-scale='deck']")).not.toBeNull();

    await user.click(account);
    const menu = screen.getByRole("menu");
    expect(menu.dataset["scale"]).toBe("deck");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Cài đặt", "Chế độ tối", "Đăng xuất"]);
    expect(within(menu).getByRole("menuitem", { name: "Cài đặt" })).toHaveAttribute(
      "href",
      "/app/settings",
    );
  });

  it("keeps the bar on a detail screen, lights no destination and offers no back arrow", async () => {
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, SETTINGS);
    await screen.findByRole("navigation", { name: NAV });

    expect(destinations()).toEqual(["Trang chủ", "Lớp"]);
    for (const link of within(
      screen.getByRole("navigation", { name: NAV }),
    ).getAllByRole("link")) {
      expect(link).not.toHaveAttribute("aria-current");
    }
    expect(screen.queryByRole("link", { name: "Quay lại" })).toBeNull();
    expect(
      screen.getByRole("link", { name: "Trang chủ Quizzivy" }),
    ).toBeInTheDocument();
  });

  it("adds a destination when its module ships", async () => {
    flags.learn = true;
    serveStudent({});
    shell("/app", <p>trang</p>);
    await screen.findByRole("navigation", { name: NAV });
    expect(destinations()).toEqual(["Trang chủ", "Lớp", "Học tập"]);
  });

  it("puts Grades after Learn when both have shipped", async () => {
    flags.learn = true;
    flags.grades = true;
    serveStudent({});
    shell("/app", <p>trang</p>);
    await screen.findByRole("navigation", { name: NAV });
    expect(destinations()).toEqual(["Trang chủ", "Lớp", "Học tập", "Điểm số"]);
  });

  it("follows the chosen theme and draws the mark for it", async () => {
    writeThemePreference("dark");
    serveStudent({});
    shell("/app", <p>trang</p>);
    const logo = await screen.findByRole("link", { name: "Trang chủ Quizzivy" });

    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(logo.querySelector("img")).toHaveAttribute(
      "src",
      "/brand/quizzivy-mark-on-dark.svg",
    );
  });

  it("gives the keyboard the page: main takes focus on a new route, unless the page took it", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    serveStudent({});
    renderAt("/app", [
      {
        element: <StudentLayout />,
        children: [
          { path: "/app", element: <p>trang chủ</p> },
          { path: "/app/classes", element: <TakesFocus /> },
        ],
      },
    ]);
    const main = await screen.findByRole("main");
    expect(main).toHaveFocus();
    expect(main).toHaveAttribute("tabindex", "-1");

    await user.click(screen.getByRole("link", { name: "Lớp" }));
    expect(await screen.findByRole("textbox", { name: "Mã lớp" })).toHaveFocus();
  });

  it("returns to the top when the route changes", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    serveStudent({});
    renderAt("/app", [
      {
        element: <StudentLayout />,
        children: [
          { path: "/app", element: <p>trang chủ</p> },
          { path: "/app/classes", element: <p>các lớp</p> },
        ],
      },
    ]);
    const main = await screen.findByRole("main");
    main.scrollTop = 240;
    await user.click(screen.getByRole("link", { name: "Lớp" }));
    await screen.findByText("các lớp");
    expect(main.scrollTop).toBe(0);
  });

  it("stays where it is when a page moves between its own sections", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    serveStudent({});
    renderAt("/app/settings", [
      {
        element: <StudentLayout />,
        children: [
          {
            path: "/app/settings/:section?",
            handle: SETTINGS,
            element: <Link to="/app/settings/security">mục bảo mật</Link>,
          },
        ],
      },
    ]);
    const main = await screen.findByRole("main");
    main.scrollTop = 240;
    await user.click(screen.getByRole("link", { name: "mục bảo mật" }));
    expect(main.scrollTop).toBe(240);
  });
});

describe("the splash hand-off", () => {
  it("gives the splash a student frame to fade onto, on its own deck surface", () => {
    const { container } = render(<>{skeletonFor("/app/classes")}</>);
    const frame = container.firstElementChild as HTMLElement;
    expect(frame.dataset["scale"]).toBe("deck");
    expect(frame.className).toContain("bg-bg");
    expect(frame.querySelectorAll("[data-slot='skeleton']").length).toBeGreaterThan(10);
    expect(within(frame).queryAllByRole("link")).toEqual([]);
  });

  it("has no frame for the test itself or for a console not yet rebuilt", () => {
    expect(
      skeletonFor("/app/attempts/018f0000-0000-7000-8000-0000000000e1"),
    ).toBeNull();
    expect(skeletonFor("/admin")).toBeNull();
    expect(
      skeletonFor("/app/attempts/018f0000-0000-7000-8000-0000000000e1/result"),
    ).not.toBeNull();
  });
});

describe("what the shell asks the server", () => {
  it("reads the lists once, never on a timer, and again when the tab comes back stale", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        asked += 1;
        return contractJson("/app/assignments", "get", 200, {
          dueNow: [],
          upcoming: [],
          completed: [],
        });
      }),
    );
    shell("/app", <p>trang</p>);
    await waitFor(() => expect(asked).toBe(1));

    await act(() => vi.advanceTimersByTimeAsync(5 * 60 * 1000));
    expect(asked).toBe(1);

    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(asked).toBe(2));
    focusManager.setFocused();
  });
});

describe("the Home badge (DG-82)", () => {
  it("counts the papers due within seven days and says what the number is", async () => {
    serveStudent({
      dueNow: [card()],
      upcoming: [card({ closesAt: "2026-09-20T00:00:00Z" })],
    });
    shell("/app", <p>trang</p>);

    const home = await screen.findByRole("link", {
      name: /^Trang chủ\s*1 bài đến hạn trong 7 ngày$/,
    });
    expect(within(home).getByText("1")).toHaveAttribute("aria-hidden", "true");
  });

  it("shows nothing when no paper is due that soon", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        asked += 1;
        return contractJson("/app/assignments", "get", 200, {
          dueNow: [card({ closesAt: "2026-09-20T00:00:00Z" })],
          upcoming: [],
          completed: [card()],
        });
      }),
    );
    shell("/app", <p>trang</p>);
    await waitFor(() => expect(asked).toBeGreaterThan(0));
    expect(await screen.findByRole("link", { name: "Trang chủ" })).toBeInTheDocument();
  });

  it("shows nothing when the lists cannot be read", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        asked += 1;
        return contractJson("/app/assignments", "get", 403, {
          error: {
            code: "FORBIDDEN",
            message: "Không có quyền.",
            requestId: ASSIGNMENT,
          },
        });
      }),
    );
    shell("/app", <p>trang</p>);
    await waitFor(() => expect(asked).toBeGreaterThan(0));
    expect(screen.getByRole("link", { name: "Trang chủ" })).toBeInTheDocument();
  });

  it("carries the count on the phone's Home tab too", async () => {
    viewport("phone");
    serveStudent({ dueNow: [card()] });
    shell("/app", <p>trang</p>);
    expect(
      await screen.findByRole("link", {
        name: /^1 bài đến hạn trong 7 ngày\s*Trang chủ$/,
      }),
    ).toHaveAttribute("href", "/app");
  });
});

describe("the shell below 768", () => {
  it("moves the destinations to a tab bar that ends in Me", async () => {
    viewport("phone");
    serveStudent({});
    shell("/app", <p>trang</p>);
    await screen.findByRole("navigation", { name: NAV });

    expect(destinations()).toEqual(["Trang chủ", "Lớp", "Tôi"]);
    const tabs = within(screen.getByRole("navigation", { name: NAV }));
    expect(tabs.getByRole("link", { name: "Trang chủ" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(tabs.getByRole("link", { name: "Tôi" })).toHaveAttribute(
      "href",
      "/app/settings",
    );
    expect(screen.getAllByRole("navigation")).toHaveLength(1);
    expect(
      screen.getByRole("link", { name: "Trang chủ Quizzivy" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    ).toBeInTheDocument();
  });

  it("lights only the tab of the screen on show", async () => {
    viewport("phone");
    serveStudent({});
    shell("/app/classes", <p>các lớp</p>);
    const tabs = within(await screen.findByRole("navigation", { name: NAV }));
    expect(tabs.getByRole("link", { name: "Lớp" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(tabs.getByRole("link", { name: "Trang chủ" })).not.toHaveAttribute(
      "aria-current",
    );
    expect(tabs.getByRole("link", { name: "Tôi" })).not.toHaveAttribute("aria-current");
  });

  it("adds a shipped module's tab before Me", async () => {
    viewport("phone");
    flags.grades = true;
    serveStudent({});
    shell("/app", <p>trang</p>);
    await screen.findByRole("navigation", { name: NAV });
    expect(destinations()).toEqual(["Trang chủ", "Lớp", "Điểm số", "Tôi"]);
  });

  it("swaps the logo for a way back and the title on a detail screen, and hides the tabs", async () => {
    viewport("phone");
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, SETTINGS);

    expect(await screen.findByRole("link", { name: "Quay lại" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(
      within(screen.getAllByRole("banner")[0]!).getByText("Cài đặt"),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Cài đặt");
    expect(screen.queryByRole("navigation", { name: NAV })).toBeNull();
    expect(screen.queryByRole("link", { name: "Trang chủ Quizzivy" })).toBeNull();
    expect(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    ).toBeInTheDocument();
  });
});

describe("crossing 768", () => {
  it("changes the chrome and keeps what the page holds", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const screenWidth = viewport("desktop");
    serveStudent({});
    shell("/app", <input aria-label="Ghi chú" />);
    const note = await screen.findByRole("textbox", { name: "Ghi chú" });
    await user.type(note, "đang viết");
    expect(destinations()).toEqual(["Trang chủ", "Lớp"]);

    act(() => screenWidth.resize("phone"));

    expect(destinations()).toEqual(["Trang chủ", "Lớp", "Tôi"]);
    expect(screen.getByRole("textbox", { name: "Ghi chú" })).toBe(note);
    expect(note).toHaveValue("đang viết");

    act(() => screenWidth.resize("desktop"));
    expect(destinations()).toEqual(["Trang chủ", "Lớp"]);
    expect(note).toHaveValue("đang viết");
  });
});

describe("home", () => {
  it("keeps all assignment groups in the main reading flow", async () => {
    serveStudent({
      dueNow: [card({ className: "IELTS Foundation" })],
      upcoming: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d2",
          testTitle: "Listening practice 03",
          status: "scheduled",
          opensAt: "2026-09-01T01:00:00Z",
        }),
      ],
      completed: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d3",
          testTitle: "Unit 4 — Passive voice",
          status: "closed",
          className: "IELTS Foundation",
          lastAttemptId: "018f0000-0000-7000-8000-0000000000e3",
          lastSubmittedAt: "2026-08-26T13:14:00Z",
        }),
      ],
    });
    shell("/app", <StudentHomePage />);
    await screen.findByText("Unit 5 — Present perfect");

    const main = within(screen.getByRole("main"));
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(
      main.getAllByRole("heading", { level: 2 }).map((heading) => heading.textContent),
    ).toEqual(["Sắp tới", "Kết quả gần đây"]);
    expect(main.getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      `/app/assignments/${card().id}`,
      "/app/assignments/018f0000-0000-7000-8000-0000000000d2",
      "/app/attempts/018f0000-0000-7000-8000-0000000000e3/result",
    ]);
  });

  it("draws no panel when there is nothing to put in it", async () => {
    serveStudent({ dueNow: [card()] }, []);
    shell("/app", <StudentHomePage />);
    await screen.findByText("Unit 5 — Present perfect");
    expect(screen.queryByRole("complementary")).toBeNull();
  });
});

describe("the intro", () => {
  it("keeps the facts and start action together, with a way back", async () => {
    server.use(
      http.get(`${BASE}/app/assignments/${ASSIGNMENT}`, () =>
        contractJson("/app/assignments/{id}", "get", 200, detail()),
      ),
    );
    shell(
      `/app/assignments/${ASSIGNMENT}`,
      <AssignmentIntroPage />,
      { detail: { titleKey: "student.shell.test", back: "/app" } },
      "/app/assignments/:id",
    );
    await screen.findByRole("heading", { level: 1, name: "Unit 5 — Present perfect" });

    const panel = within(screen.getByRole("main"));
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(panel.getByText("Thời lượng").nextElementSibling).toHaveTextContent(
      "45 phút",
    );
    expect(panel.getByRole("button", { name: "Bắt đầu làm bài" })).toBeInTheDocument();
    expect(
      within(screen.getByRole("main")).getByRole("link", { name: "Bài của tôi" }),
    ).toHaveAttribute("href", "/app");
    expect(screen.getByRole("heading", { name: "Khi làm bài" })).toBeInTheDocument();
  });
});

describe("classes", () => {
  it("draws each class with what comes next and one join action", async () => {
    serveStudent({ dueNow: [card({ classId: CLASS.id, className: CLASS.name })] });
    shell("/app/classes", <StudentClassesPage />);
    expect(
      await screen.findByRole("heading", { level: 2, name: "IELTS Foundation" }),
    ).toBeInTheDocument();
    expect(await screen.findByText(/^Unit 5 — Present perfect · /)).toBeInTheDocument();

    const main = within(screen.getByRole("main"));
    expect(main.getByRole("button", { name: "Tham gia lớp" })).toBeInTheDocument();
    expect(main.queryAllByRole("link")).toEqual([]);
    expect(screen.queryByRole("complementary")).toBeNull();
  });
});

describe("settings", () => {
  it("keeps the account, editable forms and sign-out in one flow", async () => {
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, SETTINGS);
    const panel = within(screen.getByRole("main"));
    expect(panel.getByText("Nguyễn Văn An")).toBeInTheDocument();
    expect(panel.getByText("an@example.com")).toBeInTheDocument();
    expect(panel.getByText(/Học viên ·/)).toBeInTheDocument();
    expect(panel.getByRole("button", { name: "Đăng xuất" })).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(
      screen.getByRole("heading", { level: 1, name: "Cài đặt" }),
    ).toBeInTheDocument();
  });
});
