import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Link } from "react-router";
import TeacherLayout from "@/layouts/TeacherLayout";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { teacherUser } from "@tests/support/fixtures";
import { crumbed, DASHBOARD, summaryBody, renderRoutes, serveSummary } from "./support";
import "@/lib/i18n";

const QUIET = { ...summaryBody, liveAssignments: 0, answersToGrade: 0 };

const sidebar = () => document.getElementById("console-sidebar");

function tree(children: Parameters<typeof renderRoutes>[1]) {
  return [{ path: "/teacher", element: <TeacherLayout />, children }];
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

describe("the shell every teacher page renders in", () => {
  it("is the deck's, in the chosen theme, titled by the route's crumb", async () => {
    writeThemePreference("dark");
    renderRoutes(
      "/teacher",
      tree([{ index: true, handle: DASHBOARD, element: <p>tổng quan</p> }]),
      { ...teacherUser, preferences: { theme: "dark" } },
    );
    await screen.findByText("tổng quan");

    expect(sidebar()).not.toBeNull();
    expect(document.getElementById("admin-sidebar")).toBeNull();
    expect(document.documentElement).toHaveClass("dark");
    expect(screen.getByRole("main").closest("[data-scale='deck']")).not.toBeNull();
    expect(document.title).toBe("Tổng quan · Quizzivy");
  });

  it("is the deck's for a route with no crumb too, titled Quizzivy", async () => {
    writeThemePreference("dark");
    renderRoutes(
      "/teacher/tests",
      tree([{ path: "tests", element: <p>không có tên</p> }]),
      { ...teacherUser, preferences: { theme: "dark" } },
    );
    await screen.findByText("không có tên");

    expect(sidebar()).not.toBeNull();
    expect(document.documentElement).toHaveClass("dark");
    expect(document.title).toBe("Quizzivy");
  });

  it("keeps one sidebar and one main across two routes", async () => {
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
    const before = sidebar();
    const main = screen.getByRole("main");

    await user.click(screen.getByRole("link", { name: "sang lớp học" }));
    await screen.findByText("các lớp");
    expect(sidebar()).toBe(before);
    expect(screen.getByRole("main")).toBe(main);
  });
});
