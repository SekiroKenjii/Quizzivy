import { describe, expect, it } from "vitest";
import {
  dashboardActivities,
  dashboardBars,
  dashboardDestinations,
  dashboardGreeting,
  dashboardLongDate,
  dashboardRange,
  dashboardRangeLocation,
} from "@/features/dashboard/view";
import { dashboardKeys } from "@/features/dashboard/keys";
import { dashboard23, DASHBOARD23_IDS } from "@tests/support/dashboard23";

describe("dashboard public derivations", () => {
  it.each([
    ["", "14d"],
    ["?range=invalid", "14d"],
    ["?range=7d", "7d"],
    ["?range=30d", "30d"],
    ["?range=14d", "14d"],
  ])("reads %s without rewriting the URL", (search, expected) =>
    expect(dashboardRange(search)).toBe(expected),
  );
  it("changes range alone and preserves duplicated unrelated values and hash", () => {
    expect(
      dashboardRangeLocation(
        { pathname: "/teacher", search: "?other=A&range=7d&other=B", hash: "#work" },
        "30d",
      ),
    ).toEqual({
      pathname: "/teacher",
      search: "?other=A&range=30d&other=B",
      hash: "#work",
    });
    expect(dashboardKeys.home("7d")).not.toEqual(dashboardKeys.home("30d"));
    expect(dashboardKeys.home("7d")[0]).toBe("admin-dashboard");
    expect(dashboardKeys.summary).toEqual(["admin-dashboard", "summary"]);
  });
  it.each([
    ["04:59", "evening"],
    ["05:00", "morning"],
    ["11:59", "morning"],
    ["12:00", "afternoon"],
    ["17:59", "afternoon"],
    ["18:00", "evening"],
    ["23:59", "evening"],
  ])("applies HCM greeting boundary %s", (time, greeting) =>
    expect(dashboardGreeting(new Date(`2026-09-24T${time}:00+07:00`))).toBe(greeting),
  );
  it("formats long dates after the HCM midnight even while UTC is yesterday", () => {
    const now = new Date("2025-09-23T17:00:00Z");
    expect(dashboardLongDate(now, "en")).toBe("Wednesday, 24 September");
    expect(dashboardLongDate(now, "vi")).toMatch(/Thứ Tư.*24 tháng 9/);
  });
  it("uses the exact server identities and no legacy reading", () => {
    const data = dashboard23();
    expect(dashboardDestinations(data, true, true)).toEqual({
      grade: "/teacher/grading",
      flagged: `/teacher/assignments/${DASHBOARD23_IDS.assignment}?tab=students&attempt=${DASHBOARD23_IDS.attempt}`,
      closing: `/teacher/assignments/${DASHBOARD23_IDS.assignment}?tab=students`,
      taking: `/teacher/assignments/${DASHBOARD23_IDS.taking}?tab=students`,
    });
    expect(dashboardDestinations(data, false, false).flagged).toBeNull();
    expect(dashboardDestinations(data, false, false).grade).toBeNull();
  });
  it("creates no navigation from positive counts with null or temporarily absent destinations", () => {
    const data = dashboard23();
    data.newestFlaggedAttempt = null;
    data.takingNow.assignmentId = null;
    delete data.closingSoon;
    expect(dashboardDestinations(data, true, true)).toMatchObject({
      flagged: null,
      closing: null,
      taking: null,
    });
    const partial = { ...data };
    Reflect.deleteProperty(partial, "newestFlaggedAttempt");
    Reflect.deleteProperty(partial.takingNow, "assignmentId");
    expect(dashboardDestinations(partial, true, true)).toMatchObject({
      flagged: null,
      taking: null,
    });
  });
  it("does not link known zero work even when destination IDs remain cached", () => {
    const data = dashboard23();
    data.awaitingGrading = 0;
    data.flaggedAttempts = 0;
    data.closingSoon = 0;
    data.takingNow.students = 0;
    expect(Object.values(dashboardDestinations(data, true, true))).toEqual([
      null,
      null,
      null,
      null,
    ]);
  });
  it("retains ordered zero days and distinct deterministic activity keys", () => {
    const data = dashboard23("7d");
    const bars = dashboardBars(
      data.submissions.days,
      "vi",
      (count, date) => `${count} · ${date}`,
    );
    expect(bars.map((bar) => [bar.key, bar.value])).toEqual(
      data.submissions.days.map((day) => [day.date, day.count]),
    );
    const rows = [data.recentActivity[0]!, data.recentActivity[0]!];
    const first = dashboardActivities(rows);
    const next = dashboardActivities([{ ...data.recentActivity[1]! }, ...rows]);
    expect(new Set(first.map((row) => row.key)).size).toBe(2);
    expect(next.slice(1).map((row) => row.key)).toEqual(first.map((row) => row.key));
  });
});
