import { describe, expect, it } from "vitest";
import { formatOverLimit, formatStorage } from "@/features/media/format";

const MIB = 1024 * 1024;
const GIB = 1024 * MIB;
const NBSP = String.fromCharCode(0xa0);

describe("the storage line's sizes", () => {
  it("reads the deck's figures as the deck writes them", () => {
    expect(formatStorage(5 * GIB)).toBe(`5${NBSP}GB`);
    expect(formatStorage(1.25 * GIB)).toBe(`1.25${NBSP}GB`);
    expect(formatStorage(Math.round(1.05 * GIB))).toBe(`1.05${NBSP}GB`);
    expect(formatStorage(Math.round(0.2 * GIB))).toBe(`0.2${NBSP}GB`);
  });

  it("keeps a small library in megabytes, kilobytes and bytes", () => {
    expect(formatStorage(GIB / 10 - 1)).toBe(`102.4${NBSP}MB`);
    expect(formatStorage(3.8 * MIB)).toBe(`3.8${NBSP}MB`);
    expect(formatStorage(240 * 1024)).toBe(`240${NBSP}KB`);
    expect(formatStorage(0)).toBe(`0${NBSP}B`);
  });
});

describe("the size a too-large file is refused for", () => {
  const LIMIT = 50 * MIB;

  it("keeps the usual figure when it already reads as over the limit", () => {
    expect(formatOverLimit(51 * MIB, LIMIT, "vi")).toBe("51.0 MB");
  });

  it("gives the bytes when the usual figure would equal the limit", () => {
    expect(formatOverLimit(52_428_900, LIMIT, "vi")).toBe("52.428.900 B");
    expect(formatOverLimit(52_428_900, LIMIT, "en")).toBe("52,428,900 B");
    expect(formatOverLimit(LIMIT + 1, LIMIT, "en")).toBe("52,428,801 B");
  });
});
