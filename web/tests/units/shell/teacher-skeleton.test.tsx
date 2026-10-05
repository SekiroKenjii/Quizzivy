import { afterEach, describe, expect, it } from "vitest";
import { render, within } from "@testing-library/react";
import { skeletonFor } from "@/app/boot/handoff";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import "@/layouts/TeacherLayout";
import { TeacherSkeleton } from "@/layouts/TeacherSkeleton";

const REBUILT = { crumb: [{ key: "teacherShell.nav.tests" }] };

function draw() {
  const { container } = render(<TeacherSkeleton />);
  const frame = container.firstElementChild as HTMLElement;
  const column = frame.querySelector<HTMLElement>("[data-slot='skeleton-sidebar']")!;
  return { frame, column };
}

afterEach(() => {
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("the teacher frame the splash fades onto", () => {
  it("is its own deck surface of grey blocks, with nothing to read or follow", () => {
    const { frame, column } = draw();

    expect(frame.dataset["scale"]).toBe("deck");
    expect(frame).toHaveClass("bg-bg", "flex", "h-full");
    expect(frame.querySelectorAll("[data-slot='skeleton']").length).toBeGreaterThan(40);
    expect(frame).toHaveTextContent(/^$/);
    expect(within(frame).queryAllByRole("link")).toEqual([]);
    expect(within(frame).queryAllByRole("button")).toEqual([]);
    expect(column).toHaveClass("w-62", "hidden", "min-[768px]:flex", "bg-sidebar");
    expect(column.querySelectorAll("[data-slot='skeleton']")).toHaveLength(18);
  });

  it("draws the sidebar column 60px wide, tile and icons alone, under a stored collapse", () => {
    writeSidebarState("collapsed");
    const { column } = draw();

    expect(column).toHaveClass("w-15", "items-center");
    expect(column).not.toHaveClass("w-62");
    expect(column.querySelectorAll("[data-slot='skeleton']")).toHaveLength(9);
  });
});

describe("the splash hand-off for a teacher route", () => {
  it("draws the frame for a route rebuilt for the new shell", () => {
    const { container } = render(<>{skeletonFor("/teacher/tests", REBUILT)}</>);
    expect(
      container.querySelector("[data-scale='deck'] [data-slot='skeleton-sidebar']"),
    ).not.toBeNull();
  });

  it.each([
    ["no handle", undefined],
    ["an empty handle", {}],
    ["a student detail handle", { detail: { titleKey: "nav.settings", back: "/app" } }],
  ])("draws nothing for a teacher route with %s", (_what, handle) => {
    expect(skeletonFor("/teacher/tests", handle)).toBeNull();
    expect(skeletonFor("/teacher", handle)).toBeNull();
  });

  it("draws nothing for a path outside the teacher tree, whatever the handle", () => {
    expect(skeletonFor("/login", REBUILT)).toBeNull();
    expect(skeletonFor("/teachers", REBUILT)).toBeNull();
  });
});
