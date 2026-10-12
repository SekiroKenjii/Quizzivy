import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import {
  CrumbTailContext,
  trailOf,
  useCrumbs,
  type PageCrumb,
} from "@/layouts/shell/crumbs";

const t = (key: string) => `[${key}]`;

const ASSIGNMENT = [
  { key: "nav.assignments", to: "/teacher/assignments" },
  { key: "assignment.detail", to: "/teacher/assignments/a1" },
  { key: "attempt.review" },
];

describe("trailOf", () => {
  it("translates the handle's keys when the page names nothing", () => {
    for (const tail of [null, undefined, []]) {
      expect(trailOf(ASSIGNMENT, tail, t)).toEqual([
        { label: "[nav.assignments]", to: "/teacher/assignments" },
        { label: "[assignment.detail]", to: "/teacher/assignments/a1" },
        { label: "[attempt.review]" },
      ]);
    }
  });

  it("replaces the last crumb with a tail of one", () => {
    expect(trailOf(ASSIGNMENT, [{ label: "Nguyễn Gia Bảo" }], t)).toEqual([
      { label: "[nav.assignments]", to: "/teacher/assignments" },
      { label: "[assignment.detail]", to: "/teacher/assignments/a1" },
      { label: "Nguyễn Gia Bảo" },
    ]);
  });

  it("replaces the last two with a tail of two, and takes an address the tail gives", () => {
    expect(
      trailOf(
        ASSIGNMENT,
        [
          { label: "Mid-term Reading Mock", to: "/teacher/assignments/a7" },
          { label: "Nguyễn Gia Bảo" },
        ],
        t,
      ),
    ).toEqual([
      { label: "[nav.assignments]", to: "/teacher/assignments" },
      { label: "Mid-term Reading Mock", to: "/teacher/assignments/a7" },
      { label: "Nguyễn Gia Bảo" },
    ]);
  });

  it("keeps the handle's address where the tail gives none", () => {
    expect(
      trailOf(ASSIGNMENT, [{ label: "Mid-term Reading Mock" }, { label: "Bảo" }], t)[1],
    ).toEqual({ label: "Mid-term Reading Mock", to: "/teacher/assignments/a1" });
  });

  it("keeps the trail's length when the tail is longer, using the tail's end", () => {
    expect(
      trailOf(
        [{ key: "nav.classes", to: "/teacher/classes" }, { key: "class.detail" }],
        [{ label: "một" }, { label: "hai" }, { label: "ba" }],
        t,
      ),
    ).toEqual([{ label: "hai", to: "/teacher/classes" }, { label: "ba" }]);
  });

  it("is empty for a route with no crumb, whatever the page names", () => {
    expect(trailOf([], [{ label: "Lớp 7A" }], t)).toEqual([]);
  });
});

function Names({ tail }: Readonly<{ tail: readonly PageCrumb[] | null | undefined }>) {
  useCrumbs(tail);
  return null;
}

function inShell(tail: readonly PageCrumb[] | null | undefined) {
  const set = vi.fn<(tail: readonly PageCrumb[] | null) => void>();
  const view = render(
    <CrumbTailContext value={set}>
      <Names tail={tail} />
    </CrumbTailContext>,
  );
  const show = (next: readonly PageCrumb[] | null | undefined) =>
    view.rerender(
      <CrumbTailContext value={set}>
        <Names tail={next} />
      </CrumbTailContext>,
    );
  return { set, show, unmount: view.unmount };
}

describe("useCrumbs", () => {
  it("hands the shell the tail the page names", () => {
    const { set } = inShell([{ label: "IELTS 6.5 Evening" }]);
    expect(set.mock.calls).toEqual([[[{ label: "IELTS 6.5 Evening" }]]]);
  });

  it("hands over nothing while a record loads", () => {
    for (const tail of [null, undefined, []]) {
      const { set, unmount } = inShell(tail);
      unmount();
      expect(set).not.toHaveBeenCalled();
    }
  });

  it("clears a tail that goes from named to empty, once", () => {
    for (const empty of [null, undefined, []]) {
      const { set, show, unmount } = inShell([{ label: "IELTS 6.5 Evening" }]);
      show(empty);
      expect(set.mock.calls).toEqual([[[{ label: "IELTS 6.5 Evening" }]], [null]]);
      unmount();
      expect(set).toHaveBeenCalledTimes(2);
    }
  });

  it("replaces the tail when the record's name arrives or changes", () => {
    const { set, show } = inShell(null);
    show([{ label: "IELTS 6.5 Evening" }]);
    expect(set).toHaveBeenLastCalledWith([{ label: "IELTS 6.5 Evening" }]);
    show([{ label: "IELTS 7.0 Evening", to: "/teacher/classes/c2" }]);
    expect(set).toHaveBeenLastCalledWith([
      { label: "IELTS 7.0 Evening", to: "/teacher/classes/c2" },
    ]);
  });

  it("does not hand over again a fresh array that says the same", () => {
    const { set, show } = inShell([{ label: "IELTS 6.5 Evening" }]);
    show([{ label: "IELTS 6.5 Evening" }]);
    show([{ label: "IELTS 6.5 Evening" }]);
    expect(set).toHaveBeenCalledTimes(1);
  });

  it("clears the tail when the page unmounts", () => {
    const { set, unmount } = inShell([{ label: "IELTS 6.5 Evening" }]);
    unmount();
    expect(set).toHaveBeenLastCalledWith(null);
    expect(set).toHaveBeenCalledTimes(2);
  });

  it("does nothing outside the shell", () => {
    const view = render(<Names tail={[{ label: "IELTS 6.5 Evening" }]} />);
    expect(view.container).toBeEmptyDOMElement();
    view.unmount();
  });
});
