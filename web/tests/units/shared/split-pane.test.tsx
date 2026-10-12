import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SplitPane } from "@/components/shared/SplitPane";
import "@/lib/i18n";

const PIXELS = {
  label: "Độ rộng dàn ý",
  unit: "px" as const,
  defaultSize: 300,
  min: 220,
  max: 600,
  minSecond: 360,
};
const PERCENT = {
  label: "Độ rộng cột tài liệu nguồn",
  unit: "percent" as const,
  defaultSize: 42,
  min: 30,
  max: 65,
  handle: "line" as const,
};
function Counter({ name }: Readonly<{ name: string }>) {
  const [count, setCount] = useState(0);
  return (
    <button onClick={() => setCount(count + 1)}>
      {name} {count}
    </button>
  );
}
function measured(root: Element, width = 1000, left = 100) {
  vi.spyOn(root, "getBoundingClientRect").mockReturnValue({
    width,
    left,
    right: left + width,
    top: 0,
    bottom: 480,
    height: 480,
    x: left,
    y: 0,
    toJSON: () => ({}),
  });
}
function begin(handle: HTMLElement, x = 400, id = 1) {
  fireEvent.pointerDown(handle, { button: 0, clientX: x, pointerId: id });
}

beforeEach(() => {
  localStorage.clear();
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SplitPane", () => {
  it("exposes pixel values and the Vietnamese grip hint", () => {
    const { container } = render(
      <SplitPane {...PIXELS} first={<p>first</p>} second={<p>second</p>} />,
    );
    const handle = screen.getByRole("separator", { name: PIXELS.label });
    expect(handle).toHaveAttribute("aria-orientation", "vertical");
    expect(handle).toHaveAttribute("aria-valuenow", "300");
    expect(handle).toHaveAttribute("aria-valuemin", "220");
    expect(handle).toHaveAttribute("aria-valuemax", "600");
    expect(handle).toHaveAttribute("tabindex", "0");
    expect(handle).toHaveAttribute("title", "Kéo để đổi độ rộng · bấm đúp để đặt lại");
    expect(container.firstElementChild?.firstElementChild).toHaveStyle({
      flex: "0 0 clamp(220px, 300px, calc(100% - 14px - 360px))",
    });
    expect(container.firstElementChild?.lastElementChild).toHaveStyle({
      flex: "1 1 0",
    });
  });
  it("captures a primary drag and writes exactly once on release", () => {
    const { container } = render(
      <SplitPane {...PIXELS} storageKey="split" first="first" second="second" />,
    );
    measured(container.firstElementChild!);
    const handle = screen.getByRole("separator");
    const capture = vi.spyOn(handle, "setPointerCapture"),
      release = vi.spyOn(handle, "releasePointerCapture"),
      write = vi.spyOn(Storage.prototype, "setItem");
    begin(handle);
    fireEvent.pointerMove(handle, { clientX: 430, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 460, pointerId: 1 });
    expect(handle).toHaveAttribute("aria-valuenow", "360");
    expect(write).not.toHaveBeenCalled();
    expect(capture).toHaveBeenCalledWith(1);
    expect(document.body.style.cursor).toBe("col-resize");
    expect(document.body.style.userSelect).toBe("none");
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(write).toHaveBeenCalledExactlyOnceWith("split", "360");
    expect(release).toHaveBeenCalledWith(1);
    expect(handle).not.toHaveAttribute("data-dragging");
    expect(document.body.style.cursor).toBe("");
    expect(document.body.style.userSelect).toBe("");
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(write).toHaveBeenCalledTimes(1);
  });
  it("ignores a right-button press and unrelated pointer events", () => {
    render(<SplitPane {...PIXELS} storageKey="split" first="first" second="second" />);
    const handle = screen.getByRole("separator");
    const capture = vi.spyOn(handle, "setPointerCapture");
    fireEvent.pointerDown(handle, { button: 2, clientX: 400, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 460, pointerId: 1 });
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(capture).not.toHaveBeenCalled();
    expect(handle).toHaveAttribute("aria-valuenow", "300");
    expect(localStorage.getItem("split")).toBeNull();
    begin(handle);
    fireEvent.pointerMove(handle, { clientX: 460, pointerId: 2 });
    fireEvent.pointerUp(handle, { pointerId: 2 });
    expect(handle).toHaveAttribute("aria-valuenow", "300");
    expect(handle).toHaveAttribute("data-dragging", "true");
    fireEvent.pointerCancel(handle, { pointerId: 1 });
  });
  it("steps, reaches endpoints and writes each handled key while passing other keys through", () => {
    render(<SplitPane {...PIXELS} storageKey="split" first="first" second="second" />);
    const handle = screen.getByRole("separator");
    const write = vi.spyOn(Storage.prototype, "setItem");
    for (const [key, value] of [
      ["ArrowRight", 316],
      ["ArrowLeft", 300],
      ["Home", 220],
      ["End", 600],
    ] as const) {
      expect(fireEvent.keyDown(handle, { key })).toBe(false);
      expect(handle).toHaveAttribute("aria-valuenow", String(value));
      expect(localStorage.getItem("split")).toBe(String(value));
    }
    expect(write).toHaveBeenCalledTimes(4);
    expect(fireEvent.keyDown(handle, { key: "a" })).toBe(true);
    expect(write).toHaveBeenCalledTimes(4);
  });
  it("reports the rendered width and the real ceiling from the first paint", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      width: 600,
      left: 0,
      right: 600,
      top: 0,
      bottom: 480,
      height: 480,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });
    render(<SplitPane {...PIXELS} first="first" second="second" />);
    const handle = screen.getByRole("separator");
    expect(handle).toHaveAttribute("aria-valuemax", "226");
    expect(handle).toHaveAttribute("aria-valuenow", "226");
  });
  it("uses the measured pixel ceiling on keys and on a drag", () => {
    const { container } = render(
      <SplitPane {...PIXELS} first="first" second="second" />,
    );
    const root = container.firstElementChild!,
      handle = screen.getByRole("separator");
    measured(root, 720);
    fireEvent.keyDown(handle, { key: "End" });
    expect(handle).toHaveAttribute("aria-valuenow", "346");
    expect(handle).toHaveAttribute("aria-valuemax", "346");
    begin(handle);
    fireEvent.pointerMove(handle, { clientX: 1000, pointerId: 1 });
    expect(handle).toHaveAttribute("aria-valuenow", "346");
    fireEvent.pointerUp(handle, { pointerId: 1 });
    vi.mocked(root.getBoundingClientRect).mockReturnValue({
      width: 400,
      left: 0,
      right: 400,
      top: 0,
      bottom: 0,
      height: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });
    fireEvent.keyDown(handle, { key: "End" });
    expect(handle).toHaveAttribute("aria-valuenow", "220");
    expect(handle).toHaveAttribute("aria-valuemax", "220");
  });
  it("resets to the default and forgets the stored size", () => {
    localStorage.setItem("split", "250");
    const read = vi.spyOn(Storage.prototype, "getItem");
    const { rerender } = render(
      <SplitPane {...PIXELS} storageKey="split" first="first" second="second" />,
    );
    const handle = screen.getByRole("separator");
    expect(handle).toHaveAttribute("aria-valuenow", "250");
    localStorage.setItem("split", "500");
    rerender(
      <SplitPane {...PIXELS} storageKey="split" first="updated" second="second" />,
    );
    expect(read).toHaveBeenCalledTimes(1);
    expect(handle).toHaveAttribute("aria-valuenow", "250");
    fireEvent.doubleClick(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "300");
    expect(localStorage.getItem("split")).toBeNull();
  });
  it("resizes despite refused storage reads, writes and resets", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("refused");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("refused");
    });
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
      throw new Error("refused");
    });
    render(<SplitPane {...PIXELS} storageKey="split" first="first" second="second" />);
    const handle = screen.getByRole("separator");
    fireEvent.keyDown(handle, { key: "ArrowRight" });
    expect(handle).toHaveAttribute("aria-valuenow", "316");
    begin(handle);
    fireEvent.pointerMove(handle, { clientX: 460, pointerId: 1 });
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(handle).toHaveAttribute("aria-valuenow", "376");
    fireEvent.doubleClick(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "300");
  });
  it("never reads, writes or removes storage without a key", () => {
    const get = vi.spyOn(Storage.prototype, "getItem"),
      set = vi.spyOn(Storage.prototype, "setItem"),
      remove = vi.spyOn(Storage.prototype, "removeItem");
    render(<SplitPane {...PIXELS} first="first" second="second" />);
    const handle = screen.getByRole("separator");
    fireEvent.keyDown(handle, { key: "ArrowRight" });
    begin(handle);
    fireEvent.pointerMove(handle, { clientX: 460, pointerId: 1 });
    fireEvent.pointerUp(handle, { pointerId: 1 });
    fireEvent.doubleClick(handle);
    expect(get).not.toHaveBeenCalled();
    expect(set).not.toHaveBeenCalled();
    expect(remove).not.toHaveBeenCalled();
  });
  it("keeps both wrapper identities and child state through stacking and dragging", () => {
    const first = <Counter name="first" />,
      second = <Counter name="second" />;
    const { container, rerender } = render(
      <SplitPane {...PIXELS} first={first} second={second} />,
    );
    const a = container.firstElementChild?.firstElementChild,
      b = container.firstElementChild?.lastElementChild;
    fireEvent.click(screen.getByRole("button", { name: "first 0" }));
    fireEvent.click(screen.getByRole("button", { name: "second 0" }));
    rerender(<SplitPane {...PIXELS} split={false} first={first} second={second} />);
    expect(screen.queryByRole("separator")).not.toBeInTheDocument();
    expect(container.firstElementChild?.firstElementChild).toBe(a);
    expect(container.firstElementChild?.lastElementChild).toBe(b);
    expect(a).toHaveStyle({ flex: "1 1 100%" });
    expect(b).toHaveStyle({ flex: "1 1 100%" });
    expect(screen.getByRole("button", { name: "first 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "second 1" })).toBeInTheDocument();
    rerender(<SplitPane {...PIXELS} first={first} second={second} />);
    begin(screen.getByRole("separator"));
    fireEvent.pointerMove(screen.getByRole("separator"), {
      clientX: 460,
      pointerId: 1,
    });
    fireEvent.pointerUp(screen.getByRole("separator"), { pointerId: 1 });
    expect(container.firstElementChild?.firstElementChild).toBe(a);
    expect(container.firstElementChild?.lastElementChild).toBe(b);
    expect(screen.getByRole("button", { name: "first 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "second 1" })).toBeInTheDocument();
  });
  it.each(["release", "cancel", "lost", "unmount", "stack"])(
    "restores previous body styles on %s",
    (end) => {
      document.body.style.cursor = "crosshair";
      document.body.style.userSelect = "text";
      const { unmount, rerender } = render(
        <SplitPane {...PIXELS} first="first" second="second" />,
      );
      const handle = screen.getByRole("separator"),
        release = vi.spyOn(handle, "releasePointerCapture");
      begin(handle);
      if (end === "release") fireEvent.pointerUp(handle, { pointerId: 1 });
      else if (end === "cancel") fireEvent.pointerCancel(handle, { pointerId: 1 });
      else if (end === "lost") fireEvent.lostPointerCapture(handle, { pointerId: 1 });
      else if (end === "unmount") unmount();
      else
        rerender(<SplitPane {...PIXELS} split={false} first="first" second="second" />);
      expect(document.body.style.cursor).toBe("crosshair");
      expect(document.body.style.userSelect).toBe("text");
      expect(release).toHaveBeenCalledWith(1);
    },
  );
  it("supports percent drag, keyboard endpoints, reset and a line without a title", () => {
    const { container } = render(
      <SplitPane {...PERCENT} storageKey="percent" first="source" second="result" />,
    );
    measured(container.firstElementChild!);
    const handle = screen.getByRole("separator", { name: PERCENT.label });
    expect(handle).not.toHaveAttribute("title");
    expect(handle).toHaveAttribute("aria-valuenow", "42");
    expect(handle).toHaveAttribute("aria-valuemin", "30");
    expect(handle).toHaveAttribute("aria-valuemax", "65");
    begin(handle);
    for (const [x, value] of [
      [400, 30],
      [520, 42],
      [750, 65],
      [2000, 65],
      [0, 30],
    ] as const) {
      fireEvent.pointerMove(handle, { clientX: x, pointerId: 1 });
      expect(handle).toHaveAttribute("aria-valuenow", String(value));
    }
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(localStorage.getItem("percent")).toBe("30");
    for (const [key, value] of [
      ["ArrowRight", 32],
      ["ArrowLeft", 30],
      ["End", 65],
      ["Home", 30],
    ] as const) {
      fireEvent.keyDown(handle, { key });
      expect(localStorage.getItem("percent")).toBe(String(value));
    }
    fireEvent.doubleClick(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "42");
    expect(localStorage.getItem("percent")).toBeNull();
    expect(container.firstElementChild?.firstElementChild).toHaveStyle({
      flex: "0 0 42%",
    });
  });
  it("honors a custom keyboard step", () => {
    render(<SplitPane {...PIXELS} step={5} first="first" second="second" />);
    fireEvent.keyDown(screen.getByRole("separator"), { key: "ArrowRight" });
    expect(screen.getByRole("separator")).toHaveAttribute("aria-valuenow", "305");
  });
  it("measures the squeezed preference on focus without writing, rereading or remounting drafts", () => {
    localStorage.setItem("split", "900");
    const read = vi.spyOn(Storage.prototype, "getItem"),
      write = vi.spyOn(Storage.prototype, "setItem"),
      remove = vi.spyOn(Storage.prototype, "removeItem");
    const { container } = render(
      <SplitPane
        {...PIXELS}
        max={1000}
        storageKey="split"
        first={<input aria-label="first draft" defaultValue="first" />}
        second={<input aria-label="second draft" defaultValue="second" />}
      />,
    );
    const root = container.firstElementChild!,
      first = root.firstElementChild!,
      second = root.lastElementChild!,
      handle = screen.getByRole("separator");
    const firstInput = screen.getByRole("textbox", { name: "first draft" }),
      secondInput = screen.getByRole("textbox", { name: "second draft" });
    fireEvent.change(firstInput, { target: { value: "first unsaved" } });
    fireEvent.change(secondInput, { target: { value: "second unsaved" } });
    measured(root, 720);
    fireEvent.focus(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "346");
    expect(handle).toHaveAttribute("aria-valuemax", "346");
    expect(first).toHaveStyle({
      flex: "0 0 clamp(220px, 900px, calc(100% - 14px - 360px))",
    });
    measured(root, 1400);
    fireEvent.blur(handle);
    fireEvent.focus(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "900");
    expect(handle).toHaveAttribute("aria-valuemax", "1000");
    expect(root.firstElementChild).toBe(first);
    expect(root.lastElementChild).toBe(second);
    expect(screen.getByRole("textbox", { name: "first draft" })).toBe(firstInput);
    expect(firstInput).toHaveValue("first unsaved");
    expect(screen.getByRole("textbox", { name: "second draft" })).toBe(secondInput);
    expect(secondInput).toHaveValue("second unsaved");
    expect(read).toHaveBeenCalledTimes(1);
    expect(write).not.toHaveBeenCalled();
    expect(remove).not.toHaveBeenCalled();
    expect(localStorage.getItem("split")).toBe("900");
  });
  it("moves left once from a squeezed stored size before any prior measurement", () => {
    localStorage.setItem("split", "900");
    const write = vi.spyOn(Storage.prototype, "setItem");
    const { container } = render(
      <SplitPane
        {...PIXELS}
        max={1000}
        storageKey="split"
        first="first"
        second="second"
      />,
    );
    measured(container.firstElementChild!, 720);
    const handle = screen.getByRole("separator");
    fireEvent.keyDown(handle, { key: "ArrowLeft" });
    expect(handle).toHaveAttribute("aria-valuenow", "330");
    expect(handle).toHaveAttribute("aria-valuemax", "346");
    expect(write).toHaveBeenCalledExactlyOnceWith("split", "330");
  });
  it("keeps passive CSS squeeze memory until the first key steps from fresh bounds", () => {
    localStorage.setItem("split", "900");
    const write = vi.spyOn(Storage.prototype, "setItem");
    const { container } = render(
      <SplitPane
        {...PIXELS}
        max={1000}
        storageKey="split"
        first={<Counter name="first" />}
        second={<Counter name="second" />}
      />,
    );
    const root = container.firstElementChild!,
      a = root.firstElementChild,
      b = root.lastElementChild,
      handle = screen.getByRole("separator");
    measured(root, 1400);
    fireEvent.focus(handle);
    fireEvent.click(screen.getByRole("button", { name: "first 0" }));
    fireEvent.click(screen.getByRole("button", { name: "second 0" }));
    measured(root, 720);
    expect(root.firstElementChild).toHaveStyle({
      flex: "0 0 clamp(220px, 900px, calc(100% - 14px - 360px))",
    });
    expect(write).not.toHaveBeenCalled();
    fireEvent.keyDown(handle, { key: "ArrowLeft" });
    expect(handle).toHaveAttribute("aria-valuenow", "330");
    expect(handle).toHaveAttribute("aria-valuemax", "346");
    expect(write).toHaveBeenCalledExactlyOnceWith("split", "330");
    measured(root, 1400);
    fireEvent.focus(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "330");
    expect(handle).toHaveAttribute("aria-valuemax", "1000");
    expect(write).toHaveBeenCalledTimes(1);
    expect(root.firstElementChild).toBe(a);
    expect(root.lastElementChild).toBe(b);
    expect(screen.getByRole("button", { name: "first 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "second 1" })).toBeInTheDocument();
  });
  it("refreshes the reset ceiling after the row expands and only removes storage", () => {
    localStorage.setItem("split", "900");
    const write = vi.spyOn(Storage.prototype, "setItem"),
      remove = vi.spyOn(Storage.prototype, "removeItem");
    const { container } = render(
      <SplitPane
        {...PIXELS}
        max={1000}
        storageKey="split"
        first="first"
        second="second"
      />,
    );
    const root = container.firstElementChild!,
      handle = screen.getByRole("separator");
    measured(root, 720);
    fireEvent.focus(handle);
    expect(handle).toHaveAttribute("aria-valuemax", "346");
    measured(root, 1400);
    fireEvent.doubleClick(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "300");
    expect(handle).toHaveAttribute("aria-valuemax", "1000");
    expect(write).not.toHaveBeenCalled();
    expect(remove).toHaveBeenCalledExactlyOnceWith("split");
  });
  it("focus leaves percentage sizing and the no-layout maximum unchanged without a write", () => {
    const write = vi.spyOn(Storage.prototype, "setItem");
    render(
      <SplitPane {...PERCENT} storageKey="percent" first="source" second="result" />,
    );
    const handle = screen.getByRole("separator");
    fireEvent.focus(handle);
    expect(handle).toHaveAttribute("aria-valuenow", "42");
    expect(handle).toHaveAttribute("aria-valuemax", "65");
    expect(write).not.toHaveBeenCalled();
    fireEvent.keyDown(handle, { key: "ArrowLeft" });
    expect(handle).toHaveAttribute("aria-valuenow", "40");
    expect(write).toHaveBeenCalledExactlyOnceWith("percent", "40");
  });
});
