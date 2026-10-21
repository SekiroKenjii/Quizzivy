import { describe, expect, it } from "vitest";
import { matchItemKey, matchOptionKey } from "@/features/question-bank/matchKeys";

/**
 * Pins the key sequence the server's matchkey package derives (D-063), up to
 * the bounds of 12 choices and 20 items. Fails when the editor would label an
 * unsaved row differently from the key the server will send once it saves.
 * Not a copy of server/internal/shared/matchkey's test: that one pins the
 * server's half of the same rule.
 */
describe("matching keys", () => {
  it("names choices i to xii and items A to T by ordinal", () => {
    expect(Array.from({ length: 12 }, (_, ordinal) => matchOptionKey(ordinal))).toEqual(
      ["i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x", "xi", "xii"],
    );
    expect(
      Array.from({ length: 20 }, (_, ordinal) => matchItemKey(ordinal)).join(""),
    ).toBe("ABCDEFGHIJKLMNOPQRST");
    expect(matchItemKey(26)).toBe("AA");
    expect(matchOptionKey(-1)).toBe("");
    expect(matchItemKey(-1)).toBe("");
  });
});
