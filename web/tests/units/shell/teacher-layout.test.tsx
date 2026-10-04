import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useLayoutEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { useCrumbs } from "@/layouts/shell/crumbs";
import type { ContentWidth, TeacherHandle } from "@/layouts/shell/handle";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import { viewport } from "@tests/support/viewport";
import {
  crumbed,
  dashboardBody,
  failDashboard,
  home,
  renderShell,
  serveDashboard,
} from "./support";
import "@/lib/i18n";

const NAV = "Điều hướng chính";
const TRAIL = "Đường dẫn";
const STORED = "quizzivy.sidebar";
const QUIET = { ...dashboardBody, openAssignments: 0, awaitingGrading: 0 };

const LIVE = /^Bài giao\s*3 bài giao đang mở$/;
const TO_GRADE = /^Chấm bài\s*6 bài chờ chấm$/;
const NAMES = [
  /^Tổng quan$/,
  LIVE,
  TO_GRADE,
  /^Lớp học$/,
  /^Học viên$/,
  /^Đề thi$/,
  /^Ngân hàng câu hỏi$/,
  /^Thư viện media$/,
];
const LABELS = [
  "Tổng quan",
  "Bài giao",
  "Chấm bài",
  "Lớp học",
  "Học viên",
  "Đề thi",
  "Ngân hàng câu hỏi",
  "Thư viện media",
];

const CLASSES = { path: "classes", handle: crumbed("teacherShell.nav.classes") };
const CLASS: TeacherHandle = {
  crumb: [
    { key: "teacherShell.nav.classes", to: "/teacher/classes" },
    { key: "common.loading" },
  ],
};
const REVIEW = crumbed("teacherShell.nav.tests", {
  sidebar: "collapsed",
  width: "full",
});
const SETTINGS = { path: "settings/:section?", handle: crumbed("nav.settings") };

const sidebar = () => document.getElementById("teacher-sidebar");
const nav = () => screen.getByRole("navigation", { name: NAV });
const links = () => within(nav()).getAllByRole("link");
const badges = () => document.querySelectorAll("[data-slot='nav-count']");
const toggle = (name: string) => screen.getByRole("button", { name });
const trail = () =>
  within(screen.getByRole("navigation", { name: TRAIL })).getAllByRole("listitem");

function TakesFocus() {
  const field = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => field.current?.focus(), []);
  return <input ref={field} aria-label="Tên lớp" />;
}

function ClassPage() {
  const [name, setName] = useState<string | null>(null);
  useCrumbs(name === null ? null : [{ label: name }]);
  return (
    <button type="button" onClick={() => setName("IELTS 6.5 Evening")}>
      tải xong
    </button>
  );
}

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

describe("the teacher shell from 768", () => {
  it("draws the sidebar, the top bar and the page as landmarks on one deck surface", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    const main = await screen.findByRole("main");
    const root = main.closest<HTMLElement>("[data-scale='deck']")!;

    expect(root).toHaveClass("bg-bg", "text-fg", "h-svh", "text-base");
    expect(within(root).getByRole("banner")).toBeInTheDocument();
    expect(screen.getAllByRole("navigation")).toHaveLength(2);
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(sidebar()).toHaveClass("w-62", "bg-sidebar");
    expect(sidebar()).toContainElement(nav());
    expect(within(sidebar()!).getByText("Quizzivy")).toBeInTheDocument();
    expect(within(sidebar()!).getByText("Không gian giáo viên")).toBeInTheDocument();
    expect(within(main).getByText("trang tổng quan")).toBeInTheDocument();
    expect(main).toHaveClass(
      "overflow-x-hidden",
      "overflow-y-auto",
      "px-3.5",
      "pt-4",
      "pb-8",
      "min-[768px]:px-7",
      "min-[768px]:pt-6",
      "min-[768px]:pb-10",
    );
  });

  it("lists the eight destinations under their groups, the current one lit", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher/classes", [home(), { ...CLASSES, element: <p>các lớp</p> }]);
    await screen.findByText("các lớp");

    expect(
      links().map((link) => [link.textContent, link.getAttribute("href")]),
    ).toEqual([
      ["Tổng quan", "/teacher"],
      ["Bài giao", "/teacher/assignments"],
      ["Chấm bài", "/teacher/grading"],
      ["Lớp học", "/teacher/classes"],
      ["Học viên", "/teacher/students"],
      ["Đề thi", "/teacher/tests"],
      ["Ngân hàng câu hỏi", "/teacher/question-bank"],
      ["Thư viện media", "/teacher/media"],
    ]);
    expect(
      within(nav())
        .getAllByRole("list")
        .map((list) => within(list).getAllByRole("link").length),
    ).toEqual([1, 4, 3]);
    expect(within(nav()).getByText("Giảng dạy")).toBeInTheDocument();
    expect(within(nav()).getByText("Nội dung")).toBeInTheDocument();
    expect(
      links()
        .filter((link) => link.getAttribute("aria-current") === "page")
        .map((link) => link.textContent),
    ).toEqual(["Lớp học"]);
    expect(within(nav()).getByRole("link", { name: "Lớp học" })).toHaveClass(
      "bg-hover",
      "text-fg",
      "font-medium",
    );
    expect(within(nav()).getByRole("link", { name: "Đề thi" })).toHaveClass(
      "text-muted-fg",
    );
  });

  it("lights nothing on Settings", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher/settings/security", [
      { ...SETTINGS, element: <p>bảo mật</p> },
    ]);
    await screen.findByText("bảo mật");
    for (const link of links()) expect(link).not.toHaveAttribute("aria-current");
  });

  it("leaves out Students for a role that may not read them", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher", [home()], {
      ...teacherUser,
      permissions: teacherUser.permissions.filter(
        (key) => key !== "people.students.read",
      ),
    });
    await screen.findByRole("main");
    expect(links().map((link) => link.textContent)).toEqual(
      LABELS.filter((label) => label !== "Học viên"),
    );
  });

  it("keeps the workspace block a label, for an admin too, and puts the account at the foot", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher", [home()], adminUser);
    await screen.findByRole("main");

    const block = sidebar()!.querySelector<HTMLElement>("[data-slot='workspace']")!;
    expect(block.tagName).toBe("DIV");
    expect(within(sidebar()!).getAllByRole("button")).toEqual([
      screen.getByRole("button", { name: "Tài khoản của Thuong" }),
    ]);
    expect(within(sidebar()!).getByText("Quản trị viên")).toBeInTheDocument();
  });
});

describe("the sidebar's counts", () => {
  it("shows live assignments quietly and papers to grade in the accent, and says what each counts", async () => {
    serveDashboard();
    renderShell("/teacher", [home()]);

    const live = await within(nav()).findByRole("link", { name: LIVE });
    const grade = within(nav()).getByRole("link", { name: TO_GRADE });
    expect(within(live).getByText("3")).toHaveAttribute("aria-hidden", "true");
    expect(within(grade).getByText("6")).toHaveAttribute("aria-hidden", "true");
    expect(live.querySelector("[data-slot='nav-count']")).toHaveClass(
      "bg-muted",
      "text-muted-fg",
    );
    expect(grade.querySelector("[data-slot='nav-count']")).toHaveClass(
      "bg-brand",
      "text-brand-fg",
    );
    expect(badges()).toHaveLength(2);
  });

  it("draws no badge at zero, for either figure", async () => {
    serveDashboard(QUIET);
    const { client } = renderShell("/teacher", [home()]);
    await waitFor(() =>
      expect(client.getQueryState(["admin-dashboard"])?.status).toBe("success"),
    );
    expect(badges()).toHaveLength(0);
    expect(within(nav()).getByRole("link", { name: "Bài giao" })).toBeInTheDocument();
    expect(within(nav()).getByRole("link", { name: "Chấm bài" })).toBeInTheDocument();
  });

  it("draws the one that is not zero", async () => {
    serveDashboard({ ...dashboardBody, openAssignments: 0, awaitingGrading: 1 });
    renderShell("/teacher", [home()]);
    await within(nav()).findByRole("link", { name: /^Chấm bài\s*1 bài chờ chấm$/ });
    expect(within(nav()).getByRole("link", { name: "Bài giao" })).toBeInTheDocument();
    expect(badges()).toHaveLength(1);
  });

  it("draws none, and no error, when the request fails", async () => {
    const seen = failDashboard();
    const { client } = renderShell("/teacher", [home()]);
    await waitFor(() =>
      expect(client.getQueryState(["admin-dashboard"])?.status).toBe("error"),
    );
    expect(seen.asked).toBe(1);
    expect(badges()).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(links().map((link) => link.textContent)).toEqual(LABELS);
  });

  it("takes them away when a later request fails, and follows a write that invalidates the dashboard", async () => {
    serveDashboard();
    const { client } = renderShell("/teacher", [home()]);
    await within(nav()).findByRole("link", { name: LIVE });

    serveDashboard({ ...dashboardBody, openAssignments: 4 });
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    expect(
      await within(nav()).findByRole("link", {
        name: /^Bài giao\s*4 bài giao đang mở$/,
      }),
    ).toBeInTheDocument();

    failDashboard();
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    await waitFor(() => expect(badges()).toHaveLength(0));
  });

  it("asks again every minute, stops after ten idle minutes and asks once when the user comes back", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const seen = serveDashboard();
    renderShell("/teacher", [home()]);
    await waitFor(() => expect(seen.asked).toBe(1));

    await act(() => vi.advanceTimersByTimeAsync(61_000));
    await waitFor(() => expect(seen.asked).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    await waitFor(() => expect(seen.asked).toBe(3));

    await act(() => vi.advanceTimersByTimeAsync(10 * 60_000));
    const whenIdle = seen.asked;
    await act(() => vi.advanceTimersByTimeAsync(5 * 60_000));
    expect(seen.asked).toBe(whenIdle);

    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "a" }));
    });
    await waitFor(() => expect(seen.asked).toBe(whenIdle + 1));
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(seen.asked).toBe(whenIdle + 1);
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    await waitFor(() => expect(seen.asked).toBe(whenIdle + 2));
  });
});

describe("collapsing the sidebar", () => {
  it("leaves icons that keep their names, and remembers the choice", async () => {
    const user = userEvent.setup();
    serveDashboard();
    const first = renderShell("/teacher", [home()]);
    await within(nav()).findByRole("link", { name: LIVE });

    for (const link of links()) expect(link).not.toHaveAttribute("title");
    const collapse = toggle("Thu gọn thanh bên");
    expect(collapse).toHaveAttribute("aria-expanded", "true");
    expect(collapse).toHaveAttribute("aria-controls", "teacher-sidebar");
    await user.click(collapse);

    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(sidebar()).toHaveClass("w-15");
    expect(within(nav()).queryByText("Giảng dạy")).toBeNull();
    expect(within(nav()).queryByText("Nội dung")).toBeNull();
    expect(within(sidebar()!).queryByText("Không gian giáo viên")).toBeNull();
    expect(within(sidebar()!).queryByText("Trần Thị Bình")).toBeNull();
    expect(badges()).toHaveLength(0);
    for (const name of NAMES)
      expect(within(nav()).getByRole("link", { name })).toBeInTheDocument();
    expect(links().map((link) => link.getAttribute("title"))).toEqual(LABELS);
    for (const link of links()) expect(link).toHaveClass("justify-center");
    expect(within(links()[0]!).getByText("Tổng quan")).toHaveClass("sr-only");
    expect(
      screen.getByRole("button", { name: "Tài khoản của Trần Thị Bình" }),
    ).toBeInTheDocument();
    expect(sidebar()!.querySelector("img")).toHaveAttribute("alt", "Quizzivy");
    expect(localStorage.getItem(STORED)).toBe("collapsed");
    expect(toggle("Mở rộng thanh bên")).toHaveAttribute("aria-expanded", "false");

    first.unmount();
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");

    await user.click(toggle("Mở rộng thanh bên"));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem(STORED)).toBe("expanded");
    expect(within(nav()).getByText("Giảng dạy")).toBeInTheDocument();
  });

  it("never collapses by itself, however narrow the window from 768", async () => {
    viewport(768);
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem(STORED)).toBeNull();
  });

  const pages = [
    home(<Link to="/teacher/review">mở trang rà soát</Link>),
    {
      path: "review",
      handle: REVIEW,
      element: <Link to="/teacher">về tổng quan</Link>,
    },
  ];

  it("starts a route that asks for it collapsed, for the visit only, and leaves the stored choice alone", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", pages);
    await screen.findByRole("main");
    expect(sidebar()).toHaveAttribute("data-state", "expanded");

    await user.click(screen.getByRole("link", { name: "mở trang rà soát" }));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(localStorage.getItem(STORED)).toBeNull();

    await user.click(toggle("Mở rộng thanh bên"));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem(STORED)).toBeNull();
    await user.click(toggle("Thu gọn thanh bên"));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(localStorage.getItem(STORED)).toBeNull();

    await user.click(toggle("Mở rộng thanh bên"));
    await user.click(screen.getByRole("link", { name: "về tổng quan" }));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem(STORED)).toBeNull();

    await user.click(screen.getByRole("link", { name: "mở trang rà soát" }));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
  });

  it("returns to a stored collapse on leaving, whatever the visit did", async () => {
    const user = userEvent.setup();
    writeSidebarState("collapsed");
    serveDashboard(QUIET);
    renderShell("/teacher/review", pages);
    await screen.findByRole("main");
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");

    await user.click(toggle("Mở rộng thanh bên"));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem(STORED)).toBe("collapsed");

    await user.click(screen.getByRole("link", { name: "về tổng quan" }));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(localStorage.getItem(STORED)).toBe("collapsed");

    await user.click(toggle("Mở rộng thanh bên"));
    expect(localStorage.getItem(STORED)).toBe("expanded");
    await user.click(screen.getByRole("link", { name: "mở trang rà soát" }));
    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    await user.click(screen.getByRole("link", { name: "về tổng quan" }));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
  });
});

describe("the breadcrumb trail and the document title", () => {
  it("links the ancestors, marks the last crumb current and names the tab after it", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher/classes/c1", [
      home(),
      { path: "classes/:id", handle: CLASS, element: <ClassPage /> },
    ]);
    await screen.findByRole("button", { name: "tải xong" });

    const [ancestor, current] = trail();
    expect(trail()).toHaveLength(2);
    expect(within(ancestor!).getByRole("link", { name: "Lớp học" })).toHaveAttribute(
      "href",
      "/teacher/classes",
    );
    expect(ancestor).toHaveClass("hidden", "min-[768px]:flex");
    expect(within(ancestor!).getByRole("link")).not.toHaveAttribute("aria-current");
    expect(within(current!).queryByRole("link")).toBeNull();
    expect(within(current!).getByText("Đang tải…")).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(current).toHaveClass("flex");
    expect(current).not.toHaveClass("hidden");
    expect(document.title).toBe("Đang tải… · Quizzivy");
  });

  it("puts the record's name in the last crumb and the title once the page knows it", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    const view = renderShell("/teacher/classes/c1", [
      home(),
      { path: "classes/:id", handle: CLASS, element: <ClassPage /> },
    ]);
    await user.click(await screen.findByRole("button", { name: "tải xong" }));

    expect(trail().map((item) => item.textContent)).toEqual([
      "Lớp học",
      "IELTS 6.5 Evening",
    ]);
    expect(within(trail()[1]!).getByText("IELTS 6.5 Evening")).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(document.title).toBe("IELTS 6.5 Evening · Quizzivy");

    await user.click(within(nav()).getByRole("link", { name: "Tổng quan" }));
    expect(trail().map((item) => item.textContent)).toEqual(["Tổng quan"]);
    expect(document.title).toBe("Tổng quan · Quizzivy");

    view.unmount();
    expect(document.title).toBe("Quizzivy");
  });

  it("never names the tab after the page the user left", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher/classes/c1", [
      home(),
      { path: "classes/:id", handle: CLASS, element: <ClassPage /> },
    ]);
    await user.click(await screen.findByRole("button", { name: "tải xong" }));
    expect(document.title).toBe("IELTS 6.5 Evening · Quizzivy");

    const named = vi.spyOn(document, "title", "set");
    await user.click(within(nav()).getByRole("link", { name: "Tổng quan" }));
    await screen.findByText("trang tổng quan");
    expect(named.mock.calls).toEqual([["Tổng quan · Quizzivy"]]);
  });

  it("names the tab after the product on a route with no crumb", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher", [
      { index: true, handle: { crumb: [] }, element: <p>trang</p> },
    ]);
    await screen.findByText("trang");
    expect(
      within(screen.getByRole("navigation", { name: TRAIL })).queryAllByRole(
        "listitem",
      ),
    ).toEqual([]);
    expect(document.title).toBe("Quizzivy");
  });
});

describe("the page's frame", () => {
  it.each([
    [undefined, "max-w-330"],
    [1320, "max-w-330"],
    [1080, "max-w-270"],
    [860, "max-w-215"],
    [720, "max-w-180"],
  ] as [ContentWidth | undefined, string][])(
    "centres a page of width %s under %s",
    async (width, limit) => {
      serveDashboard(QUIET);
      const handle = crumbed("teacherShell.nav.tests", width ? { width } : {});
      renderShell("/teacher", [{ index: true, handle, element: <p>trang</p> }]);
      const page = (await screen.findByText("trang")).parentElement!;
      expect(page).toHaveAttribute("data-slot", "page");
      expect(page.parentElement).toBe(screen.getByRole("main"));
      expect(page).toHaveClass("mx-auto", "w-full", limit);
      expect(page.className.match(/max-w-/g)).toHaveLength(1);
    },
  );

  it("sets no limit on a full-width page", async () => {
    serveDashboard(QUIET);
    renderShell("/teacher", [{ index: true, handle: REVIEW, element: <p>trang</p> }]);
    const page = (await screen.findByText("trang")).parentElement!;
    expect(page).toHaveClass("mx-auto", "w-full");
    expect(page.className).not.toContain("max-w-");
  });

  it("registers main before a page first renders, and unregisters it on the way out", async () => {
    serveDashboard(QUIET);
    const seen: boolean[] = [];
    function Measures() {
      seen.push(useContentWidthAtLeast(860));
      return <p>trang đo</p>;
    }
    const view = renderShell("/teacher", [home(<Measures />)]);
    await screen.findByText("trang đo");
    expect(seen.length).toBeGreaterThan(0);
    expect(seen).not.toContain(true);

    view.unmount();
    expect(renderHook(() => useContentWidthAtLeast(860)).result.current).toBe(true);
  });

  it("gives the keyboard the page: main takes focus on a new route, unless the page took it", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [
      home(),
      { ...CLASSES, element: <TakesFocus /> },
      { path: "tests", handle: crumbed("teacherShell.nav.tests"), element: <p>đề</p> },
    ]);
    const main = await screen.findByRole("main");
    expect(main).toHaveFocus();
    expect(main).toHaveAttribute("tabindex", "-1");
    expect(main).toHaveClass("outline-none!");

    await user.click(within(nav()).getByRole("link", { name: "Lớp học" }));
    expect(await screen.findByRole("textbox", { name: "Tên lớp" })).toHaveFocus();

    await user.click(within(nav()).getByRole("link", { name: "Đề thi" }));
    await screen.findByText("đề");
    expect(main).toHaveFocus();
  });

  it("returns to the top when the route changes", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [home(), { ...CLASSES, element: <p>các lớp</p> }]);
    const main = await screen.findByRole("main");
    main.scrollTop = 240;
    await user.click(within(nav()).getByRole("link", { name: "Lớp học" }));
    await screen.findByText("các lớp");
    expect(main.scrollTop).toBe(0);
  });

  it("neither scrolls nor takes focus when only a parameter changes", async () => {
    serveDashboard(QUIET);
    const { router } = renderShell("/teacher/settings", [
      { ...SETTINGS, element: <p>cài đặt</p> },
    ]);
    const main = await screen.findByRole("main");
    main.scrollTop = 240;
    const outside = toggle("Thu gọn thanh bên");
    outside.focus();

    await act(() => router.navigate("/teacher/settings/security"));
    expect(router.state.location.pathname).toBe("/teacher/settings/security");
    expect(main.scrollTop).toBe(240);
    expect(outside).toHaveFocus();
  });

  it("keeps what a page holds when the width crosses 768, either way", async () => {
    const user = userEvent.setup();
    const width = viewport(1280);
    serveDashboard(QUIET);
    renderShell("/teacher", [home(<input aria-label="Ghi chú" />)]);
    const note = await screen.findByRole("textbox", { name: "Ghi chú" });
    await user.type(note, "đang viết");

    act(() => width.resize(360));
    expect(sidebar()).toBeNull();
    expect(screen.getByRole("textbox", { name: "Ghi chú" })).toBe(note);
    expect(note).toHaveValue("đang viết");

    act(() => width.resize(1280));
    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(screen.getByRole("textbox", { name: "Ghi chú" })).toBe(note);
    expect(note).toHaveValue("đang viết");
  });
});

describe("the top bar", () => {
  it("switches the theme and remembers it", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    await user.click(screen.getByRole("button", { name: "Chế độ tối" }));
    expect(document.documentElement).toHaveClass("dark");
    expect(localStorage.getItem("quizzivy.theme")).toBe("dark");

    await user.click(screen.getByRole("button", { name: "Chế độ sáng" }));
    expect(document.documentElement).not.toHaveClass("dark");
    expect(localStorage.getItem("quizzivy.theme")).toBe("light");
  });

  it("stays dark under a stored dark theme, with the mark drawn for it", async () => {
    writeThemePreference("dark");
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    expect(document.documentElement).toHaveClass("dark");
    expect(screen.getByRole("button", { name: "Chế độ sáng" })).toBeInTheDocument();
    expect(sidebar()!.querySelector("img")).toHaveAttribute(
      "src",
      "/brand/quizzivy-mark-on-dark.svg",
    );
  });

  it("opens the command palette from the search button and with Ctrl+K, outside the deck surface", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    const search = screen.getByRole("button", { name: "Tìm kiếm…" });
    expect(within(search).getByText("Tìm kiếm…")).toHaveClass(
      "hidden",
      "min-[768px]:block",
    );
    expect(within(search).getByText("Ctrl").parentElement).toHaveClass(
      "hidden",
      "min-[768px]:flex",
    );
    expect(within(search).getByText("K")).toBeInTheDocument();
    expect(search).toHaveClass("w-8.5", "min-[768px]:w-50", "min-[1100px]:w-65");

    await user.click(search);
    const palette = await screen.findByRole("dialog", { name: "Tìm kiếm và lệnh" });
    expect(palette.closest("[data-scale='deck']")).toBeNull();
    expect(palette).not.toHaveAttribute("data-scale");

    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(
      await screen.findByRole("dialog", { name: "Tìm kiếm và lệnh" }),
    ).toBeInTheDocument();
  });

  it("keeps the sidebar in the page's flow at 800", async () => {
    viewport(800);
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(toggle("Thu gọn thanh bên")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tìm kiếm…" })).toHaveClass(
      "min-[768px]:w-50",
    );
  });
});

describe("the teacher shell below 768", () => {
  beforeEach(() => {
    viewport(360);
  });

  async function openDrawer(user: ReturnType<typeof userEvent.setup>) {
    await user.click(toggle("Mở menu"));
    return screen.findByRole("dialog", { name: NAV });
  }

  it("has no sidebar in the page's flow, and a toggle that opens the menu", async () => {
    serveDashboard();
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    expect(sidebar()).toBeNull();
    expect(screen.queryByRole("navigation", { name: NAV })).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(toggle("Mở menu")).toHaveAttribute("aria-expanded", "false");
    expect(toggle("Mở menu")).toHaveAttribute("aria-controls", "teacher-sidebar");
    expect(screen.queryByRole("button", { name: "Thu gọn thanh bên" })).toBeNull();
  });

  it("opens a drawer with labels and badges, whatever is stored", async () => {
    const user = userEvent.setup();
    writeSidebarState("collapsed");
    serveDashboard();
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    const drawer = await openDrawer(user);

    expect(drawer).toBe(sidebar());
    expect(drawer).toHaveAttribute("data-state", "open");
    expect(drawer).toHaveAttribute("data-scale", "deck");
    expect(drawer).toHaveClass("w-68", "bg-sidebar", "z-(--z-sheet)", "text-base");
    expect(drawer).toContainElement(document.activeElement as HTMLElement);
    expect(within(drawer).getByText("Giảng dạy")).toBeInTheDocument();
    expect(within(drawer).getByText("Không gian giáo viên")).toBeInTheDocument();
    expect(within(drawer).getByText("Trần Thị Bình")).toBeInTheDocument();
    expect(await within(drawer).findByRole("link", { name: LIVE })).toBeInTheDocument();
    expect(badges()).toHaveLength(2);
    for (const link of within(drawer).getAllByRole("link"))
      expect(link).not.toHaveAttribute("title");
    expect(localStorage.getItem(STORED)).toBe("collapsed");
  });

  it("closes on Escape and gives focus back to the toggle", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    await openDrawer(user);

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(toggle("Mở menu")).toHaveFocus());
    expect(toggle("Mở menu")).toHaveAttribute("aria-expanded", "false");
  });

  it("closes when a destination is chosen, and leaves focus on main", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    renderShell("/teacher", [home(), { ...CLASSES, element: <p>các lớp</p> }]);
    await screen.findByRole("main");
    const drawer = await openDrawer(user);

    await user.click(within(drawer).getByRole("link", { name: "Lớp học" }));
    await screen.findByText("các lớp");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("closes on a navigation that keeps the route, and still leaves focus on main", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    const { router } = renderShell("/teacher/settings", [
      { ...SETTINGS, element: <p>cài đặt</p> },
    ]);
    await screen.findByRole("main");
    await openDrawer(user);

    await act(() => router.navigate("/teacher/settings/security"));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("stays closed when the user goes back to the entry it was opened at", async () => {
    const user = userEvent.setup();
    serveDashboard(QUIET);
    const { router } = renderShell("/teacher", [
      home(),
      { ...CLASSES, element: <p>các lớp</p> },
    ]);
    await screen.findByRole("main");
    const drawer = await openDrawer(user);
    await user.click(within(drawer).getByRole("link", { name: "Lớp học" }));
    await screen.findByText("các lớp");

    await act(() => router.navigate(-1));
    await screen.findByText("trang tổng quan");
    expect(router.state.location.pathname).toBe("/teacher");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("closes when the window grows past 768, and stays closed when it shrinks again", async () => {
    const user = userEvent.setup();
    const width = viewport(360);
    serveDashboard(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    await openDrawer(user);

    act(() => width.resize(1280));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sidebar()).toHaveAttribute("data-state", "expanded");

    act(() => width.resize(360));
    expect(sidebar()).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
