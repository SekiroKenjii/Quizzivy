import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Link, Outlet } from "react-router";
import TeacherShell from "@/layouts/TeacherShell";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { teacherUser } from "@tests/support/fixtures";
import { crumbed, DASHBOARD, summaryBody, renderRoutes, serveSummary } from "./support";
import "@/lib/i18n";

const QUIET = { ...summaryBody, liveAssignments: 0, answersToGrade: 0 };

const rebuilt = () => document.getElementById("teacher-sidebar");
const legacy = () => document.getElementById("admin-sidebar");

function tree(children: Parameters<typeof renderRoutes>[1]) {
  return [{ path: "/teacher", element: <TeacherShell />, children }];
}

beforeEach(() => {
  viewport(1440);
  document.title = "Quizzivy";
  serveSummary(QUIET);
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  writeThemePreference("light");
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("the shell a teacher page renders in", () => {
  it("is the old one, forced light and with its slots, for a route with no handle", async () => {
    writeThemePreference("dark");
    renderRoutes(
      "/teacher/tests",
      tree([{ path: "tests", element: <p>đề thi cũ</p> }]),
      { ...teacherUser, preferences: { theme: "dark" } },
    );
    await screen.findByText("đề thi cũ");

    expect(legacy()).not.toBeNull();
    expect(rebuilt()).toBeNull();
    expect(document.documentElement).not.toHaveClass("dark");
    expect(document.querySelectorAll("[data-columns]")).toHaveLength(2);
    expect(screen.getByRole("main").closest("[data-scale='deck']")).toBeNull();
    expect(document.title).toBe("Quizzivy");
  });

  it("is the new one, in the chosen theme, for a route that declares a crumb", async () => {
    writeThemePreference("dark");
    renderRoutes(
      "/teacher",
      tree([{ index: true, handle: DASHBOARD, element: <p>tổng quan mới</p> }]),
      { ...teacherUser, preferences: { theme: "dark" } },
    );
    await screen.findByText("tổng quan mới");

    expect(rebuilt()).not.toBeNull();
    expect(legacy()).toBeNull();
    expect(document.documentElement).toHaveClass("dark");
    expect(screen.getByRole("main").closest("[data-scale='deck']")).not.toBeNull();
    expect(document.title).toBe("Tổng quan · Quizzivy");
  });

  it("swaps with the route, and the theme and the title with it", async () => {
    const user = userEvent.setup();
    writeThemePreference("dark");
    renderRoutes(
      "/teacher",
      tree([
        {
          index: true,
          handle: DASHBOARD,
          element: <Link to="/teacher/tests">sang trang cũ</Link>,
        },
        { path: "tests", element: <Link to="/teacher">về trang mới</Link> },
      ]),
      { ...teacherUser, preferences: { theme: "dark" } },
    );
    await user.click(await screen.findByRole("link", { name: "sang trang cũ" }));

    await screen.findByRole("link", { name: "về trang mới" });
    expect(legacy()).not.toBeNull();
    expect(rebuilt()).toBeNull();
    expect(document.documentElement).not.toHaveClass("dark");
    expect(document.title).toBe("Quizzivy");

    await user.click(screen.getByRole("link", { name: "về trang mới" }));
    await screen.findByRole("link", { name: "sang trang cũ" });
    expect(rebuilt()).not.toBeNull();
    expect(legacy()).toBeNull();
    expect(document.documentElement).toHaveClass("dark");
    expect(document.title).toBe("Tổng quan · Quizzivy");
  });

  it("keeps one sidebar and one main across two rebuilt routes", async () => {
    const user = userEvent.setup();
    renderRoutes(
      "/teacher",
      tree([
        {
          index: true,
          handle: DASHBOARD,
          element: <Link to="/teacher/classes">sang lớp học</Link>,
        },
        {
          path: "classes",
          handle: crumbed("teacherShell.nav.classes"),
          element: <p>các lớp</p>,
        },
      ]),
    );
    await screen.findByRole("link", { name: "sang lớp học" });
    const sidebar = rebuilt();
    const main = screen.getByRole("main");

    await user.click(screen.getByRole("link", { name: "sang lớp học" }));
    await screen.findByText("các lớp");
    expect(rebuilt()).toBe(sidebar);
    expect(screen.getByRole("main")).toBe(main);
  });

  it("is not moved by a handle on a parent route: only the leaf's counts", async () => {
    renderRoutes(
      "/teacher/imports/new",
      tree([
        {
          path: "imports",
          handle: crumbed("teacherShell.nav.tests"),
          element: <Outlet />,
          children: [
            {
              index: true,
              handle: crumbed("teacherShell.nav.tests"),
              element: <p>a</p>,
            },
            { path: "new", element: <p>nhập đề cũ</p> },
          ],
        },
      ]),
    );
    await screen.findByText("nhập đề cũ");
    expect(legacy()).not.toBeNull();
    expect(rebuilt()).toBeNull();
  });

  it("follows the leaf under a parent with no handle", async () => {
    renderRoutes(
      "/teacher/imports",
      tree([
        {
          path: "imports",
          element: <Outlet />,
          children: [
            {
              index: true,
              handle: crumbed("teacherShell.nav.tests"),
              element: <p>lịch sử nhập mới</p>,
            },
          ],
        },
      ]),
    );
    await screen.findByText("lịch sử nhập mới");
    expect(rebuilt()).not.toBeNull();
    expect(legacy()).toBeNull();
  });

  it.each([
    ["a student detail handle", { detail: { titleKey: "nav.settings", back: "/app" } }],
    ["a crumb that is one object", { crumb: { key: "nav.tests" } }],
  ])("stays the old one for %s", async (_what, handle) => {
    renderRoutes(
      "/teacher/tests",
      tree([{ path: "tests", handle, element: <p>cũ</p> }]),
    );
    await screen.findByText("cũ");
    expect(legacy()).not.toBeNull();
    expect(rebuilt()).toBeNull();
  });
});
