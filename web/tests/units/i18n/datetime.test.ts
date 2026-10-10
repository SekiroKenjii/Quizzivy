import { describe, expect, it } from "vitest";
import {
  appDaysUntil,
  appHour,
  audioLength,
  clockTime,
  countdown,
  dayDate,
  dayMonth,
  dayOfMonth,
  formatMoment,
  formatTime,
  sameAppDay,
  shortDate,
  weekdayDate,
  weekdayName,
  weekdayShort,
} from "@/lib/i18n/datetime";

const instant = "2026-09-07T01:00:00Z";

describe("the one set of formatters", () => {
  it("renders every moment in the app's zone", () => {
    expect(formatTime(instant)).toBe("08:00");
    expect(shortDate(instant)).toBe("07/09");
    expect(weekdayDate(instant)).toBe("Thứ hai, 07/09");
    expect(formatMoment(instant)).toBe("08:00 · Thứ hai, 07/09");
    expect(sameAppDay(instant, "2026-09-06T17:30:00Z")).toBe(true);
    expect(sameAppDay(instant, "2026-09-06T16:30:00Z")).toBe(false);
  });

  it("counts down the same way on the paper and in the teacher's monitor", () => {
    expect(countdown(90 * 60_000)).toBe("1:30:00");
    expect(countdown(44 * 60_000 + 58_000)).toBe("44:58");
    expect(countdown(-5_000)).toBe("00:00");
  });

  it("reads an audio length off the player", () => {
    expect(audioLength(110_000)).toBe("1:50");
    expect(audioLength(null)).toBe("—");
  });
});

const midnight = "2026-08-29T17:00:00Z";
const saturday = "2026-08-29T10:00:00Z";

describe("a day in the app's zone, where the UTC day differs", () => {
  it("reads the hour off the app's clock", () => {
    expect(appHour(midnight)).toBe(0);
    expect(appHour(saturday)).toBe(17);
    expect(appHour(new Date("2026-08-29T16:59:59Z"))).toBe(23);
  });

  it("counts calendar days, not 24-hour spans", () => {
    expect(appDaysUntil("2026-08-29T16:59:59Z", saturday)).toBe(0);
    expect(appDaysUntil(midnight, saturday)).toBe(1);
    expect(appDaysUntil("2026-08-29T23:30:00Z", saturday)).toBe(1);
    expect(appDaysUntil("2026-09-04T16:59:59Z", saturday)).toBe(6);
    expect(appDaysUntil("2026-09-04T17:00:00Z", saturday)).toBe(7);
    expect(appDaysUntil("2026-08-28T16:59:59Z", saturday)).toBe(-1);
    expect(appDaysUntil("2026-08-29T20:00:00Z", "2026-08-29T18:00:00Z")).toBe(0);
  });

  it("names the weekday short and long in both languages", () => {
    expect(weekdayShort(saturday)).toBe("T7");
    expect(weekdayShort(midnight)).toBe("CN");
    expect(weekdayShort(saturday, "en")).toBe("Sat");
    expect(weekdayShort(midnight, "en")).toBe("Sun");
    expect(weekdayName("2026-09-03T01:00:00Z")).toBe("thứ năm");
    expect(weekdayName(midnight)).toBe("chủ nhật");
    expect(weekdayName("2026-09-03T01:00:00Z", "en")).toBe("Thursday");
  });

  it("writes the day, the date and the two together", () => {
    expect(dayOfMonth(midnight)).toBe("30");
    expect(dayOfMonth("2026-09-01T01:00:00Z")).toBe("1");
    expect(dayMonth("2026-09-04T17:00:00Z")).toBe("05/09");
    expect(dayMonth("2026-09-04T17:00:00Z", "en")).toBe("5 Sep");
    expect(dayDate("2026-08-26T09:30:00Z")).toBe("Thứ 4, 26/08");
    expect(dayDate(midnight)).toBe("CN, 30/08");
    expect(dayDate("2026-08-26T09:30:00Z", "en")).toBe("Wed 26 Aug");
  });
});

describe("the time of a moment, with its day when the day is not today", () => {
  const now = "2026-10-12T01:00:00Z";
  const today = "2026-10-12T05:00:00Z";
  const later = "2026-10-13T19:34:00Z";

  it("is the bare time on the day of `now`, in both languages", () => {
    expect(clockTime(today, now)).toBe("12:00");
    expect(clockTime(today, now, "vi")).toBe("12:00");
    expect(clockTime(today, now, "en")).toBe("12:00");
  });

  it("names the day with a number and a slash in Vietnamese", () => {
    expect(clockTime(later, now)).toBe("02:34, 14/10");
    expect(clockTime(later, now, "vi")).toBe("02:34, 14/10");
  });

  it("names the month in English, where 14/10 and 10/14 are both read", () => {
    expect(clockTime(later, now, "en")).toBe("02:34, 14 Oct");
    expect(clockTime("2026-10-02T19:34:00Z", now, "en")).toBe("02:34, 3 Oct");
    expect(clockTime("2026-10-02T19:34:00Z", now, "vi")).toBe("02:34, 03/10");
  });

  it("reads the day in the app's zone, not in UTC", () => {
    expect(clockTime("2026-10-12T17:30:00Z", now, "en")).toBe("00:30, 13 Oct");
  });
});
