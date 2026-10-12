import { it, expect } from "vitest";
import {
  bumpHour,
  bumpMinute,
  chooseDay,
  dateTimeLabel,
  parseDateTime,
  pickerStart,
  serializeDateTime,
} from "@/components/shared/dateTimeValue";
it.each([
  ["date", "2024-02-29"],
  ["datetime", "2026-12-31T23:07"],
  ["time", "08:07"],
] as const)("round trips wall-clock %s without UTC", (mode, value) => {
  const date = parseDateTime(value, mode, new Date(2026, 8, 24));
  expect(date).not.toBeNull();
  expect(serializeDateTime(date!, mode)).toBe(value);
});
it.each(["2025-02-29", "2026-13-01", "2026-04-31", "2026-00-01", "bad", ""])(
  "refuses invalid date %s",
  (value) => expect(parseDateTime(value, "date")).toBeNull(),
);
it.each(["24:00", "08:60", "8:00", ""])("refuses invalid time %s", (value) =>
  expect(parseDateTime(value, "time")).toBeNull(),
);
it("keeps an off-step minute until an explicit nearest-step bump and never carries hours", () => {
  const date = parseDateTime("2026-09-24T08:07", "datetime")!;
  expect(serializeDateTime(date, "datetime")).toBe("2026-09-24T08:07");
  expect(serializeDateTime(bumpMinute(date, 1, 5), "datetime")).toBe(
    "2026-09-24T08:10",
  );
  expect(serializeDateTime(bumpMinute(date, -1, 5), "datetime")).toBe(
    "2026-09-24T08:00",
  );
  const last = parseDateTime("08:55", "time")!;
  expect(serializeDateTime(bumpMinute(last, 1, 5), "time")).toBe("08:00");
  expect(serializeDateTime(bumpMinute(date, 1, 15), "time")).toBe("08:15");
});
it("wraps hours and selects month-boundary days without moving the other components", () => {
  const start = parseDateTime("2026-12-31T00:10", "datetime")!;
  expect(serializeDateTime(bumpHour(start, -1), "datetime")).toBe("2026-12-31T23:10");
  expect(serializeDateTime(chooseDay(start, new Date(2027, 0, 1)), "datetime")).toBe(
    "2027-01-01T00:10",
  );
});
it("empty pickers start at the current full hour, and labels change language without changing stored values", () => {
  const now = new Date(2026, 8, 24, 10, 37);
  expect(serializeDateTime(pickerStart("", "datetime", now), "datetime")).toBe(
    "2026-09-24T10:00",
  );
  const date = parseDateTime("2026-09-24T08:07", "datetime")!;
  expect(dateTimeLabel(date, "datetime", "en")).toContain("24 Sept");
  expect(dateTimeLabel(date, "datetime", "vi")).not.toBe(
    dateTimeLabel(date, "datetime", "en"),
  );
  expect(serializeDateTime(date, "datetime")).toBe("2026-09-24T08:07");
});
