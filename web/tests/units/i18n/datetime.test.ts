import { describe, expect, it } from "vitest";
import {
  appDaysUntil,
  appHour,
  audioLength,
  clockTime,
  countdown,
  dayDate,
  dayMonth,
  dayMonthYear,
  dayOfMonth,
  formatAgo,
  formatMoment,
  formatRelativeShort,
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
    expect(dayMonthYear("2026-10-24T16:59:00Z")).toBe("24/10/2026");
    expect(dayMonthYear("2026-10-24T16:59:00Z", "en")).toBe("24 Oct 2026");
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

describe("how long ago, for a line about an account", () => {
  const now = Date.parse("2026-10-14T12:00:00Z");
  const ago = (ms: number, locale: "vi" | "en" = "en") =>
    formatAgo(
      new Date(now - ms).toISOString(),
      locale,
      locale === "vi" ? "vừa xong" : "just now",
      now,
    );
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  it("says just now under a minute, and for a moment the browser's clock has not reached", () => {
    expect(ago(10_000)).toBe("just now");
    expect(ago(59_000, "vi")).toBe("vừa xong");
    expect(ago(-2 * minute)).toBe("just now");
  });

  it("counts minutes, hours and days, with yesterday for one", () => {
    expect(ago(minute)).toBe("1 minute ago");
    expect(ago(45 * minute, "vi")).toBe("45 phút trước");
    expect(ago(3 * hour)).toBe("3 hours ago");
    expect(ago(30 * hour)).toBe("yesterday");
    expect(ago(30 * hour, "vi")).toBe("hôm qua");
    expect(ago(3 * day)).toBe("3 days ago");
  });

  it("goes on in weeks, months and years instead of a bare date", () => {
    expect(ago(7 * day)).toBe("1 week ago");
    expect(ago(15 * day, "vi")).toBe("2 tuần trước");
    expect(ago(29 * day)).toBe("4 weeks ago");
    expect(ago(30 * day)).toBe("1 month ago");
    expect(ago(30 * day, "vi")).toBe("1 tháng trước");
    expect(ago(40 * day)).toBe("1 month ago");
    expect(ago(200 * day, "vi")).toBe("6 tháng trước");
    expect(ago(364 * day)).toBe("11 months ago");
    expect(ago(365 * day)).toBe("1 year ago");
    expect(ago(365 * day, "vi")).toBe("1 năm trước");
    expect(ago(800 * day)).toBe("2 years ago");
  });
});

describe("how long ago, for a table cell (D-017)", () => {
  const now = Date.parse("2026-10-14T12:00:00Z");
  const ago = (ms: number, locale: "vi" | "en" = "en") =>
    formatRelativeShort(
      new Date(now - ms).toISOString(),
      locale,
      locale === "vi" ? "vừa xong" : "now",
      now,
    );
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  it("says now under a minute, and for a moment the browser's clock has not reached", () => {
    expect(ago(10_000)).toBe("now");
    expect(ago(59_000, "vi")).toBe("vừa xong");
    expect(ago(-2 * minute)).toBe("now");
  });

  it("is the bare value in the narrow form, with no sentence around it", () => {
    expect(ago(5 * minute)).toBe("5m ago");
    expect(ago(5 * minute, "vi")).toBe("5 phút trước");
    expect(ago(3 * hour)).toBe("3h ago");
    expect(ago(23 * hour + 59 * minute, "vi")).toBe("23 giờ trước");
  });

  it("counts days to six with no yesterday, so a column keeps one shape", () => {
    expect(ago(30 * hour)).toBe("1d ago");
    expect(ago(30 * hour, "vi")).toBe("1 ngày trước");
    expect(ago(3 * day)).toBe("3d ago");
    expect(ago(6 * day + 23 * hour, "vi")).toBe("6 ngày trước");
  });

  it("counts weeks to four, then gives the date with its year", () => {
    expect(ago(7 * day)).toBe("1w ago");
    expect(ago(15 * day, "vi")).toBe("2 tuần trước");
    expect(ago(34 * day)).toBe("4w ago");
    expect(ago(35 * day)).toBe("9 Sep 2026");
    expect(ago(35 * day, "vi")).toBe("09/09/2026");
    expect(ago(800 * day)).toBe("5 Aug 2024");
  });
});
