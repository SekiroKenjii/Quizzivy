import { afterEach, describe, expect, it, vi } from "vitest";
import { revealWithin } from "@/lib/revealWithin";

function at(left: number, right: number) {
  const element = document.createElement("div");
  vi.spyOn(element, "getBoundingClientRect").mockReturnValue({
    left,
    right,
    width: right - left,
    top: 0,
    bottom: 30,
    height: 30,
    x: left,
    y: 0,
    toJSON: () => ({}),
  });
  return element;
}

function frame(scrollLeft: number) {
  const element = at(100, 300);
  element.scrollLeft = scrollLeft;
  element.scrollTop = 7;
  return element;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("revealWithin", () => {
  it.each([
    ["cut by the right edge", 252, 340, 0, 40, 0],
    ["wholly beyond the right edge", 342, 430, 0, 130, 0],
    ["cut by the left edge", 40, 130, 80, 20, 0],
    ["cut by the right edge, with room to spare", 252, 340, 0, 43, 3],
    ["cut by the left edge, with room to spare", 40, 130, 80, 17, 3],
  ])("scrolls sideways to an element %s", (_, left, right, from, to, padding) => {
    const outer = frame(from);
    revealWithin(outer, at(left, right), padding);
    expect(outer.scrollLeft).toBe(to);
    expect(outer.scrollTop).toBe(7);
  });

  it.each([
    ["inside", 132, 250],
    ["flush with both edges", 100, 300],
  ])("leaves an element %s where it is", (_, left, right) => {
    const outer = frame(80);
    revealWithin(outer, at(left, right), 3);
    expect(outer.scrollLeft).toBe(80);
  });

  it("shows the start of an element wider than the frame", () => {
    const outer = frame(80);
    revealWithin(outer, at(60, 420));
    expect(outer.scrollLeft).toBe(40);
  });
});
