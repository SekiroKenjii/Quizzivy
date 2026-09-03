import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";

/**
 * A `datetime-local` field carries no timezone: the browser reads its value in
 * the DEVICE's zone. Everything in Quizzivy is Asia/Ho_Chi_Minh (§13.2), so on
 * a laptop set to anything else a naive `new Date(value)` shifts every window
 * the teacher sets -- an 08:00 open becomes 15:00 for the whole class.
 */
describe("datetime-local, pinned to the app's timezone", () => {
  const original = process.env["TZ"];
  beforeAll(() => {
    process.env["TZ"] = "UTC";
  });
  afterAll(() => {
    process.env["TZ"] = original;
  });

  it("is testing something: the device zone disagrees", () => {
    expect(new Date("2026-08-29T08:00").toISOString()).toBe("2026-08-29T08:00:00.000Z");
  });

  it("reads a field value as Vietnam time, not the device's", () => {
    // 08:00 in Asia/Ho_Chi_Minh (UTC+7) is 01:00 UTC, on any machine.
    expect(fromDateTimeInput("2026-08-29T08:00").toISOString()).toBe(
      "2026-08-29T01:00:00.000Z",
    );
  });

  it("writes an instant back into the field as Vietnam time", () => {
    expect(toDateTimeInput("2026-08-29T01:00:00.000Z")).toBe("2026-08-29T08:00");
  });

  it("round-trips a value the teacher typed", () => {
    const typed = "2026-12-31T23:30";
    expect(toDateTimeInput(fromDateTimeInput(typed))).toBe(typed);
  });
});
