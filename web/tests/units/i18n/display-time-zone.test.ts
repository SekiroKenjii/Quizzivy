import { afterEach, describe, expect, it } from "vitest";
import {
  APP_TIME_ZONE,
  appDaysUntil,
  appHour,
  formatDateTime,
  fromDateTimeInput,
  getDisplayTimeZone,
  getRequestedTimeZone,
  setDisplayTimeZone,
  subscribeDisplayTimeZone,
  toDateTimeInput,
} from "@/lib/i18n/datetime";

afterEach(() => setDisplayTimeZone(APP_TIME_ZONE));

describe("display zones", () => {
  it("reacts immediately across midnight and DST without changing dirty input interpretation", () => {
    const changes: string[] = [];
    const stop = subscribeDisplayTimeZone(() => changes.push(getDisplayTimeZone()));
    const input = toDateTimeInput("2026-03-08T06:30:00Z");
    expect(setDisplayTimeZone("America/New_York")).toBe(true);
    expect(formatDateTime("2026-03-08T06:30:00Z", "en")).toBe("01:30, 08/03/2026");
    expect(formatDateTime("2026-03-08T07:30:00Z", "en")).toBe("03:30, 08/03/2026");
    expect(appHour("2026-03-08T07:30:00Z")).toBe(3);
    expect(appDaysUntil("2026-03-09T04:30:00Z", "2026-03-08T06:30:00Z")).toBe(1);
    expect(toDateTimeInput("2026-03-08T06:30:00Z")).toBe(input);
    expect(fromDateTimeInput(input).toISOString()).toBe("2026-03-08T06:30:00.000Z");
    expect(changes).toEqual(["America/New_York"]);
    stop();
    setDisplayTimeZone("UTC");
    expect(changes).toHaveLength(1);
  });

  it("keeps the unsupported server value separate from explicit HCM presentation", () => {
    expect(setDisplayTimeZone("not/a-supported-zone")).toBe(false);
    expect(getRequestedTimeZone()).toBe("not/a-supported-zone");
    expect(getDisplayTimeZone()).toBe(APP_TIME_ZONE);
    expect(formatDateTime("2026-10-01T20:00:00Z")).toBe("03:00, 02/10/2026");
    setDisplayTimeZone("UTC");
    expect(formatDateTime("2026-10-01T20:00:00Z")).toBe("20:00, 01/10/2026");
  });
});
