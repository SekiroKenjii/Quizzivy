import { describe, expect, it } from "vitest";
import {
  appDaysUntil,
  appHour,
  audioLength,
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

  it("counts down the same way on the paper and on the card", () => {
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
