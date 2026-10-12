import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { MemoryRouter } from "react-router";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { PageHead } from "@/layouts/shell/PageHead";
import { writeSidebarState } from "@/layouts/shell/sidebarState";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { summaryBody, renderShell, serveSummary } from "./support";
import "@/lib/i18n";

const TRAIL = "Đường dẫn";
const CLASS = {
  crumb: [
    { key: "teacherShell.nav.classes", to: "/teacher/classes" },
    { key: "common.loading" },
  ],
};

function alone(head: React.ReactElement) {
  render(<MemoryRouter>{head}</MemoryRouter>);
  return document.querySelector<HTMLElement>("[data-slot='page-head']")!;
}

function crumbs() {
  return within(screen.getByRole("navigation", { name: TRAIL }))
    .getAllByRole("listitem")
    .map((item) => item.textContent);
}

function NamedPage() {
  const [head, setHead] = useState(false);
  useCrumbs([{ label: "Mid-term Reading Mock" }]);
  return (
    <>
      <button type="button" onClick={() => setHead((shown) => !shown)}>
        đổi tiêu đề
      </button>
      {head && <PageHead title="Nguyễn Gia Bảo" />}
    </>
  );
}

beforeEach(() => {
  viewport(1280);
  document.title = "Quizzivy";
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("PageHead, the list form", () => {
  it("renders without the shell: one h1 at the teacher scale, the description and the actions", () => {
    const head = alone(
      <PageHead
        title="Bài giao"
        description="Các đề bạn đã giao cho lớp và học viên."
        actions={<button type="button">Giao bài mới</button>}
      />,
    );

    expect(screen.getAllByRole("heading")).toHaveLength(1);
    const title = screen.getByRole("heading", { level: 1, name: "Bài giao" });
    expect(title).toHaveClass("text-h1", "min-w-0");
    expect(title).not.toHaveClass("text-h1-student", "text-stat");
    expect(screen.getByText("Các đề bạn đã giao cho lớp và học viên.")).toHaveClass(
      "text-muted-fg",
      "text-base",
      "mt-0.5",
    );
    expect(head).toHaveClass(
      "flex",
      "flex-wrap",
      "items-end",
      "justify-between",
      "gap-3",
    );
    expect(title.parentElement).toBe(head.firstElementChild);
    expect(title.parentElement).not.toHaveClass("flex-[1_1_280px]");
    const actions = screen.getByRole("button", { name: "Giao bài mới" }).parentElement;
    expect(actions).toBe(head.lastElementChild);
    expect(actions).toHaveClass("flex", "flex-wrap", "gap-2");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("draws neither a description nor an actions row it was not given", () => {
    const head = alone(<PageHead title="Lớp học" />);
    expect(head.children).toHaveLength(1);
    expect(head.firstElementChild!.children).toHaveLength(1);
  });

  it("lets a page restate a class of the row", () => {
    const head = alone(<PageHead title="Unit 5" className="items-start gap-3.5" />);
    expect(head).toHaveClass("items-start", "gap-3.5", "flex-wrap");
    expect(head).not.toHaveClass("items-end", "gap-3");
  });
});

describe("PageHead, the detail form", () => {
  it("puts the way back above the title, the status beside it and the page's own line after", () => {
    const head = alone(
      <PageHead
        title="IELTS 6.5 Evening"
        back={{ to: "/teacher/classes", label: "Lớp học" }}
        status={<span>18 học viên</span>}
        actions={<button type="button">Thêm học viên</button>}
      >
        <p>Thứ 3 và thứ 5</p>
      </PageHead>,
    );
    const column = head.firstElementChild!;
    const back = screen.getByRole("link", { name: "Lớp học" });
    const title = screen.getByRole("heading", { level: 1, name: "IELTS 6.5 Evening" });
    const row = title.parentElement!;

    expect(column).toHaveClass(
      "min-w-0",
      "flex",
      "flex-col",
      "gap-1",
      "flex-[1_1_280px]",
    );
    expect(back).toHaveAttribute("href", "/teacher/classes");
    expect(back).toHaveClass("text-muted-fg", "hover:text-fg", "self-start", "text-sm");
    expect([...column.children]).toEqual([
      back,
      row,
      screen.getByText("Thứ 3 và thứ 5"),
    ]);
    expect(row).toHaveClass("flex", "flex-wrap", "items-center", "gap-2.5");
    expect([...row.children]).toEqual([title, screen.getByText("18 học viên")]);
    expect(screen.getAllByRole("heading")).toHaveLength(1);
  });

  it("is the detail form with a way back alone", () => {
    const back = alone(
      <PageHead
        title="Sửa câu hỏi"
        back={{ to: "/teacher/question-bank", label: "Ngân hàng câu hỏi" }}
      />,
    );
    expect(back.firstElementChild).toHaveClass("flex-col", "gap-1");
    expect(screen.getByRole("heading", { level: 1 }).parentElement).toBe(
      back.firstElementChild,
    );
  });

  it("is the detail form with a status alone, and has no link back", () => {
    const head = alone(<PageHead title="Unit 5" status={<span>Đang mở</span>} />);
    expect(head.firstElementChild).toHaveClass("flex-col", "gap-1", "flex-[1_1_280px]");
    expect(screen.queryByRole("link")).toBeNull();
    expect(
      screen.getByRole("heading", { level: 1 }).nextElementSibling,
    ).toHaveTextContent("Đang mở");
  });

  it("gives a description the column's gap in place of its own margin", () => {
    alone(
      <PageHead
        title="Unit 5"
        status={<span>Đang mở</span>}
        description="Lớp 7A · 45 phút"
      />,
    );
    expect(screen.getByText("Lớp 7A · 45 phút")).not.toHaveClass("mt-0.5");
  });
});

describe("PageHead in the shell", () => {
  it("names the page in the trail and the tab when asked to", async () => {
    serveSummary({ ...summaryBody, liveAssignments: 0, answersToGrade: 0 });
    renderShell("/teacher/classes/c1", [
      {
        path: "classes/:id",
        handle: CLASS,
        element: <PageHead title="IELTS 6.5 Evening" crumb />,
      },
    ]);
    await screen.findByRole("heading", { level: 1, name: "IELTS 6.5 Evening" });

    expect(crumbs()).toEqual(["Lớp học", "IELTS 6.5 Evening"]);
    expect(document.title).toBe("IELTS 6.5 Evening · Quizzivy");
  });

  it("leaves the route's own label when not asked", async () => {
    serveSummary({ ...summaryBody, liveAssignments: 0, answersToGrade: 0 });
    renderShell("/teacher/classes/c1", [
      {
        path: "classes/:id",
        handle: CLASS,
        element: <PageHead title="IELTS 6.5 Evening" />,
      },
    ]);
    await screen.findByRole("heading", { level: 1, name: "IELTS 6.5 Evening" });

    expect(crumbs()).toEqual(["Lớp học", "Đang tải…"]);
    expect(document.title).toBe("Đang tải… · Quizzivy");
  });

  it("leaves the name a page gave with useCrumbs when a head without crumb comes and goes", async () => {
    const user = userEvent.setup();
    serveSummary({ ...summaryBody, liveAssignments: 0, answersToGrade: 0 });
    renderShell("/teacher/classes/c1", [
      { path: "classes/:id", handle: CLASS, element: <NamedPage /> },
    ]);
    const swap = await screen.findByRole("button", { name: "đổi tiêu đề" });
    expect(crumbs()).toEqual(["Lớp học", "Mid-term Reading Mock"]);

    await user.click(swap);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Nguyễn Gia Bảo",
    );
    expect(crumbs()).toEqual(["Lớp học", "Mid-term Reading Mock"]);
    expect(document.title).toBe("Mid-term Reading Mock · Quizzivy");

    await user.click(swap);
    expect(screen.queryByRole("heading")).toBeNull();
    expect(crumbs()).toEqual(["Lớp học", "Mid-term Reading Mock"]);
    expect(document.title).toBe("Mid-term Reading Mock · Quizzivy");
  });
});
