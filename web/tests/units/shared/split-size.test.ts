import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  clampSplitSize,
  nextSplitSize,
  percentSplitSize,
  pixelSplitSize,
  readSplitSize,
  writeSplitSize,
} from "@/components/shared/splitSize";

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe("split sizing", () => {
  it.each([
    [219, 220],
    [220, 220],
    [500, 500],
    [501, 500],
  ])("clamps %s to %s", (size, expected) => {
    expect(clampSplitSize(size, 220, 500)).toBe(expected);
  });
  it("honors the minimum when the measured ceiling is lower", () => {
    expect(clampSplitSize(300, 220, 100)).toBe(220);
  });
  it("applies positive and negative pixel deltas from the starting position", () => {
    expect(pixelSplitSize(300, 400, 460, 220, 600)).toBe(360);
    expect(pixelSplitSize(300, 400, 340, 220, 600)).toBe(240);
    expect(pixelSplitSize(300, 400, 900, 220, 600)).toBe(600);
    expect(pixelSplitSize(300, 400, 0, 220, 600)).toBe(220);
  });
  it.each([
    [400, 30],
    [520, 42],
    [750, 65],
    [0, 30],
    [1200, 65],
  ])("converts pointer %s to %s percent", (x, expected) => {
    expect(percentSplitSize(x, 100, 1000, 30, 65)).toBe(expected);
  });
  it("falls back to the minimum without measurable width", () => {
    expect(percentSplitSize(100, 0, 0, 30, 65)).toBe(30);
  });
  it("maps the four keys and passes other keys through", () => {
    expect(nextSplitSize("ArrowLeft", 300, 16, 220, 500)).toBe(284);
    expect(nextSplitSize("ArrowRight", 300, 16, 220, 500)).toBe(316);
    expect(nextSplitSize("Home", 300, 16, 220, 500)).toBe(220);
    expect(nextSplitSize("End", 300, 16, 220, 500)).toBe(500);
    expect(nextSplitSize("a", 300, 16, 220, 500)).toBeNull();
    expect(nextSplitSize("End", 300, 16, 220, 100)).toBe(220);
  });
  it.each(["abc", "", " ", "NaN", "Infinity"])(
    "ignores invalid stored %j",
    (stored) => {
      localStorage.setItem("split", stored);
      expect(readSplitSize("split", 300, 220, 500)).toBe(300);
    },
  );
  it("restores valid values, clamps overflow and defaults for a missing key", () => {
    expect(readSplitSize("split", 300, 220, 500)).toBe(300);
    localStorage.setItem("split", "9000");
    expect(readSplitSize("split", 300, 220, 500)).toBe(500);
    localStorage.setItem("split", "250");
    expect(readSplitSize("split", 300, 220, 500)).toBe(250);
    localStorage.setItem("split", "-1");
    expect(readSplitSize("split", 300, 220, 500)).toBe(220);
  });
  it("survives refused reads, writes and removal", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("refused");
    });
    expect(readSplitSize("split", 300, 220, 500)).toBe(300);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("refused");
    });
    expect(() => writeSplitSize("split", 350)).not.toThrow();
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
      throw new Error("refused");
    });
    expect(() => writeSplitSize("split", null)).not.toThrow();
  });
  it("does not touch storage without a key", () => {
    const get = vi.spyOn(Storage.prototype, "getItem"),
      set = vi.spyOn(Storage.prototype, "setItem"),
      remove = vi.spyOn(Storage.prototype, "removeItem");
    expect(readSplitSize(undefined, 300, 220, 500)).toBe(300);
    writeSplitSize(undefined, 350);
    writeSplitSize(undefined, null);
    expect(get).not.toHaveBeenCalled();
    expect(set).not.toHaveBeenCalled();
    expect(remove).not.toHaveBeenCalled();
  });
});
