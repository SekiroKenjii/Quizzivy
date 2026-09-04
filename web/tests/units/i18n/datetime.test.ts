import { describe, expect, it } from "vitest";
import {
  audioLength,
  countdown,
  formatMoment,
  formatTime,
  sameAppDay,
  shortDate,
  weekdayDate,
} from "@/lib/i18n/datetime";

const instant = "2026-09-07T01:00:00Z";

describe("the one set of formatters", () => {
  it("renders every moment in the app's zone", () => {
    expect(formatTime(instant)).toBe("08:00");
    expect(shortDate(instant)).toBe("07/09");
    expect(weekdayDate(instant)).toBe("Thứ Hai, 07/09");
    expect(formatMoment(instant)).toBe("08:00 · Thứ Hai, 07/09");
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
