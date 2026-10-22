import { describe, expect, it } from "vitest";
import {
  BANDS,
  bandOfCriteria,
  bandOfKey,
  formatBand,
  isBand,
  pointsOfBand,
} from "@/lib/bands";

const rubric = [{ id: "a" }, { id: "b" }, { id: "c" }, { id: "d" }];

describe("bands", () => {
  it("runs 3 to 9 in half steps (DG-86)", () => {
    expect(BANDS).toHaveLength(13);
    expect(BANDS[0]).toBe(3);
    expect(BANDS.at(-1)).toBe(9);
    expect(BANDS.every(isBand)).toBe(true);
    expect([2.5, 9.5, 6.25, Number.NaN].some(isBand)).toBe(false);
  });

  it("derives a rubric's band as the server does: the mean rounded half up to 0.5", () => {
    expect(bandOfCriteria(rubric, { a: 6, b: 6.5, c: 6.5, d: 6 })).toBe(6.5); // 6.25
    expect(bandOfCriteria(rubric, { a: 7, b: 6.5, c: 7, d: 6.5 })).toBe(7); // 6.75
    expect(bandOfCriteria(rubric, { a: 6, b: 6, c: 6, d: 6.5 })).toBe(6); // 6.125
    expect(bandOfCriteria(rubric.slice(0, 3), { a: 6, b: 6, c: 6.5 })).toBe(6); // 6.1666
  });

  it("has no band until every criterion has one", () => {
    expect(bandOfCriteria(rubric, { a: 6, b: 7, c: 8 })).toBeNull();
    expect(bandOfCriteria([], {})).toBeNull();
  });

  it("converts a band to the points the server stores", () => {
    expect(pointsOfBand(6.5, 9)).toBe(6.5);
    expect(pointsOfBand(6.5, 10)).toBeCloseTo(7.22, 5);
    expect(pointsOfBand(9, 2)).toBe(2);
  });

  it("reads band keys by code: the digit is the band, Shift the half above", () => {
    expect(bandOfKey({ code: "Digit6", shiftKey: false })).toBe(6);
    expect(bandOfKey({ code: "Digit6", shiftKey: true })).toBe(6.5);
    expect(bandOfKey({ code: "Numpad3", shiftKey: false })).toBe(3);
    expect(bandOfKey({ code: "Digit9", shiftKey: false })).toBe(9);
    expect(bandOfKey({ code: "Digit9", shiftKey: true })).toBeNull();
    expect(bandOfKey({ code: "Digit2", shiftKey: false })).toBeNull();
    expect(bandOfKey({ code: "KeyJ", shiftKey: false })).toBeNull();
  });

  it("writes a band with one decimal in the reader's locale", () => {
    expect(formatBand(6, "en")).toBe("6.0");
    expect(formatBand(6.5, "vi")).toBe("6,5");
  });
});
