import { describe, expect, it } from "vitest";
import vi from "./locales/vi.json";
import en from "./locales/en.json";
import { formatDateTime, formatDuration, APP_TIME_ZONE } from "./datetime";

/** Flatten to dotted key paths so a nested drift is caught, not just a top-level one. */
function keyPaths(obj: unknown, prefix = ""): string[] {
  if (typeof obj !== "object" || obj === null) return [prefix];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) =>
    keyPaths(v, prefix ? `${prefix}.${k}` : k),
  );
}

describe("locale parity", () => {
  const viKeys = keyPaths(vi).sort();
  const enKeys = keyPaths(en).sort();

  it("has the same keys in vi and en", () => {
    // AGENTS.md: "No English-only user-facing text ever reaches a commit."
    // The inverse matters too -- a vi-only key renders as a raw key path in en.
    expect(enKeys.filter((k) => !viKeys.includes(k)), "keys in en but missing from vi").toEqual([]);
    expect(viKeys.filter((k) => !enKeys.includes(k)), "keys in vi but missing from en").toEqual([]);
  });

  it("has no empty strings", () => {
    const empty = (obj: unknown, prefix = ""): string[] => {
      if (typeof obj === "string") return obj.trim() === "" ? [prefix] : [];
      if (typeof obj !== "object" || obj === null) return [];
      return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) =>
        empty(v, prefix ? `${prefix}.${k}` : k),
      );
    };
    expect(empty(vi)).toEqual([]);
    expect(empty(en)).toEqual([]);
  });
});

describe("datetime", () => {
  it("renders UTC in Asia/Ho_Chi_Minh, not the device timezone", () => {
    // 2026-01-15T03:30:00Z is 10:30 in UTC+7. This assertion holds wherever
    // the test runs, which is the whole point (§13.2).
    expect(APP_TIME_ZONE).toBe("Asia/Ho_Chi_Minh");
    expect(formatDateTime("2026-01-15T03:30:00Z")).toBe("10:30, 15/01/2026");
  });

  it("rolls the date over correctly across the UTC+7 boundary", () => {
    // 17:00Z is midnight the NEXT day in Ho Chi Minh City.
    expect(formatDateTime("2026-01-15T17:00:00Z")).toBe("00:00, 16/01/2026");
  });

  it("formats a countdown", () => {
    expect(formatDuration(0)).toBe("00:00");
    expect(formatDuration(59_000)).toBe("00:59");
    expect(formatDuration(90_000)).toBe("01:30");
    expect(formatDuration(3_600_000)).toBe("1:00:00");
    expect(formatDuration(-5_000), "a passed deadline shows zero, never negative").toBe("00:00");
  });
});
