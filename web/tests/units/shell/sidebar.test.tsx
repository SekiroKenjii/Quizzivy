import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import { useAuthStore } from "@/stores/auth";
import { adminUser } from "@tests/support/fixtures";
import { viewport } from "@tests/support/viewport";
import { home, renderShell, serveSummary, summaryBody } from "./support";
import "@/lib/i18n";

const NAV = "Điều hướng chính";
const QUIET = { ...summaryBody, liveAssignments: 0, answersToGrade: 0 };

const sidebar = () => document.getElementById("teacher-sidebar")!;
const nav = () => screen.getByRole("navigation", { name: NAV });
const separators = (scope: HTMLElement) => within(scope).queryAllByRole("separator");

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("the sidebar's groups (V-01)", () => {
  it("draws a 1px separator in place of each group label while collapsed", async () => {
    viewport(1024);
    writeSidebarState("collapsed");
    serveSummary(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    expect(sidebar()).toHaveAttribute("data-state", "collapsed");
    expect(within(nav()).queryByText("Giảng dạy")).toBeNull();
    expect(within(nav()).queryByText("Nội dung")).toBeNull();
    const lines = separators(nav());
    expect(lines).toHaveLength(2);
    for (const line of lines) {
      expect(line).toHaveClass("bg-border", "h-px", "mx-2.5", "mb-1");
      expect(line.nextElementSibling?.tagName).toBe("UL");
    }
    const groups = within(nav())
      .getAllByRole("list")
      .map((list) => list.parentElement!);
    expect(groups.map((group) => separators(group).length)).toEqual([0, 1, 1]);
  });

  it("draws the same for an admin in the teacher workspace", async () => {
    viewport(1024);
    writeSidebarState("collapsed");
    serveSummary(QUIET);
    renderShell("/teacher", [home()], adminUser);
    await screen.findByRole("main");

    expect(separators(nav())).toHaveLength(2);
  });

  it("draws labels and no separator while expanded", async () => {
    viewport(1024);
    serveSummary(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");

    expect(sidebar()).toHaveAttribute("data-state", "expanded");
    expect(within(nav()).getByText("Giảng dạy")).toBeInTheDocument();
    expect(within(nav()).getByText("Nội dung")).toBeInTheDocument();
    expect(separators(nav())).toHaveLength(0);
  });

  it("draws labels and no separator in the drawer, whatever is stored", async () => {
    viewport(360);
    writeSidebarState("collapsed");
    serveSummary(QUIET);
    renderShell("/teacher", [home()]);
    await screen.findByRole("main");
    await userEvent.setup().click(screen.getByRole("button", { name: "Mở menu" }));
    const drawer = await screen.findByRole("dialog", { name: NAV });

    expect(within(drawer).getByText("Giảng dạy")).toBeInTheDocument();
    expect(separators(drawer)).toHaveLength(0);
  });
});
