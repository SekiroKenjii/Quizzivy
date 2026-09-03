import { describe, expect, it } from "vitest";
import { pageCountOf, pageWindow, parsePage } from "@/lib/pagination";

/** Never more than seven slots, first and last always there, no gap of one. */
describe("pageWindow", () => {
  it("is empty for a single page", () => {
    expect(pageWindow(1, 1)).toEqual([]);
    expect(pageWindow(1, 0)).toEqual([]);
  });

  it("lists every page when there are few", () => {
    expect(pageWindow(2, 4)).toEqual([1, 2, 3, 4]);
  });

  it("keeps the current page's neighbours and elides the rest", () => {
    expect(pageWindow(6, 12)).toEqual([1, "gap", 5, 6, 7, "gap", 12]);
  });

  it("fills a gap that would only hide one page", () => {
    expect(pageWindow(4, 12)).toEqual([1, 2, 3, 4, 5, "gap", 12]);
  });

  it("stays anchored at either end", () => {
    expect(pageWindow(1, 26)).toEqual([1, 2, "gap", 26]);
    expect(pageWindow(26, 26)).toEqual([1, "gap", 25, 26]);
  });
});

describe("parsePage", () => {
  it("reads a positive integer and falls back to 1 for anything else", () => {
    expect(parsePage("3")).toBe(3);
    expect(parsePage("0")).toBe(1);
    expect(parsePage("-2")).toBe(1);
    expect(parsePage("2.5")).toBe(1);
    expect(parsePage("abc")).toBe(1);
    expect(parsePage(null)).toBe(1);
  });
});

describe("pageCountOf", () => {
  it("rounds up and tolerates a zero size", () => {
    expect(pageCountOf(45, 20)).toBe(3);
    expect(pageCountOf(40, 20)).toBe(2);
    expect(pageCountOf(0, 20)).toBe(0);
    expect(pageCountOf(5, 0)).toBe(0);
  });
});
