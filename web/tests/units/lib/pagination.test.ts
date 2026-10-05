import { describe, expect, it } from "vitest";
import {
  pageCountOf,
  pageRange,
  pageWindow,
  parsePage,
  parsePageSize,
} from "@/lib/pagination";

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

describe("parsePageSize", () => {
  const SIZES = [10, 20, 30, 50];

  it("reads a size that is listed", () => {
    expect(parsePageSize("30", SIZES)).toBe(30);
    expect(parsePageSize("10", SIZES)).toBe(10);
    expect(parsePageSize("50", SIZES)).toBe(50);
  });

  it("falls back to the first size for one that is not listed", () => {
    expect(parsePageSize("7", SIZES)).toBe(10);
    expect(parsePageSize("100", SIZES)).toBe(10);
    expect(parsePageSize("0", SIZES)).toBe(10);
    expect(parsePageSize("abc", SIZES)).toBe(10);
    expect(parsePageSize("", SIZES)).toBe(10);
    expect(parsePageSize(null, SIZES)).toBe(10);
  });

  it("takes the default from the list it is given", () => {
    expect(parsePageSize(null, [24, 48])).toBe(24);
    expect(parsePageSize("10", [24, 48])).toBe(24);
    expect(parsePageSize("48", [24, 48])).toBe(48);
  });

  it("does not read an absent parameter as a listed zero", () => {
    expect(parsePageSize(null, [5, 0])).toBe(5);
  });
});

describe("pageRange", () => {
  it("places the first page of 312 at ten a page", () => {
    expect(pageRange(1, 10, 312)).toEqual({ page: 1, pages: 32, from: 1, to: 10 });
  });

  it("places a middle page", () => {
    expect(pageRange(3, 10, 312)).toEqual({ page: 3, pages: 32, from: 21, to: 30 });
    expect(pageRange(2, 20, 312)).toEqual({ page: 2, pages: 16, from: 21, to: 40 });
  });

  it("ends the last page at the total", () => {
    expect(pageRange(32, 10, 312)).toEqual({ page: 32, pages: 32, from: 311, to: 312 });
  });

  it("clamps a page past the end, and one before the start", () => {
    expect(pageRange(40, 10, 312)).toEqual({ page: 32, pages: 32, from: 311, to: 312 });
    expect(pageRange(0, 10, 312)).toEqual({ page: 1, pages: 32, from: 1, to: 10 });
    expect(pageRange(-3, 10, 312)).toEqual({ page: 1, pages: 32, from: 1, to: 10 });
  });

  it("has one page and no positions for an empty list", () => {
    expect(pageRange(1, 10, 0)).toEqual({ page: 1, pages: 1, from: 0, to: 0 });
    expect(pageRange(4, 10, 0)).toEqual({ page: 1, pages: 1, from: 0, to: 0 });
    expect(pageRange(1, 10, -1)).toEqual({ page: 1, pages: 1, from: 0, to: 0 });
  });

  it("has one page for a total equal to the size, and two for one more", () => {
    expect(pageRange(1, 10, 10)).toEqual({ page: 1, pages: 1, from: 1, to: 10 });
    expect(pageRange(2, 10, 11)).toEqual({ page: 2, pages: 2, from: 11, to: 11 });
    expect(pageRange(1, 10, 3)).toEqual({ page: 1, pages: 1, from: 1, to: 3 });
  });

  it("puts every row on one page for a size of zero or less", () => {
    expect(pageRange(1, 0, 45)).toEqual({ page: 1, pages: 1, from: 1, to: 45 });
    expect(pageRange(3, -5, 45)).toEqual({ page: 1, pages: 1, from: 1, to: 45 });
    expect(pageRange(1, 0, 0)).toEqual({ page: 1, pages: 1, from: 0, to: 0 });
  });
});
