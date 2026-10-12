import { describe, expect, it } from "vitest";
import { sameRows, withPreference } from "@/features/notifications/preferences";
import { preferences } from "./support";

describe("notification preference rows", () => {
  it("changes one row without changing the order, other four rows or other channel", () => {
    const changed = withPreference(preferences, "attempt.flagged", { inApp: false });
    expect(changed).toHaveLength(5);
    expect(changed.map((row) => row.event)).toEqual(
      preferences.map((row) => row.event),
    );
    expect(changed[1]).toEqual({ ...preferences[1], inApp: false });
    expect(preferences[1]!.inApp).toBe(true);
    for (const index of [0, 2, 3, 4]) expect(changed[index]).toBe(preferences[index]);
  });
  it("compares event, both channels, order and length", () => {
    expect(
      sameRows(
        preferences,
        preferences.map((row) => ({ ...row })),
      ),
    ).toBe(true);
    expect(
      sameRows(
        preferences,
        withPreference(preferences, "result.ready", { email: true }),
      ),
    ).toBe(false);
    expect(
      sameRows(
        preferences,
        withPreference(preferences, "result.ready", { inApp: false }),
      ),
    ).toBe(false);
    expect(sameRows(preferences, [...preferences].reverse())).toBe(false);
    expect(sameRows(preferences, preferences.slice(1))).toBe(false);
  });
});
