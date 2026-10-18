import { describe, expect, it } from "vitest";
import {
  PAGE_MAX,
  SEARCH_MAX,
  SEARCH_MIN,
  USER_PAGE_SIZES,
} from "@/features/users/api";
import {
  isTooShort,
  limitOf,
  pageOf,
  searchOf,
  stateOf,
  tabOf,
} from "@/features/users/userFilters";

describe("tabOf", () => {
  it.each(["all", "teachers", "students", "admins", "disabled"] as const)(
    "reads %s off the URL",
    (tab) => {
      expect(tabOf(tab)).toBe(tab);
    },
  );

  it("falls back on All for an absent or unknown tab", () => {
    expect(tabOf(null)).toBe("all");
    expect(tabOf("")).toBe("all");
    expect(tabOf("owners")).toBe("all");
    expect(tabOf("Teachers")).toBe("all");
  });
});

describe("stateOf", () => {
  it("reads the two states the Overview links to", () => {
    expect(stateOf("never_signed_in")).toBe("never_signed_in");
    expect(stateOf("must_change_password")).toBe("must_change_password");
  });

  it("is none for anything else", () => {
    expect(stateOf(null)).toBeNull();
    expect(stateOf("")).toBeNull();
    expect(stateOf("active")).toBeNull();
    expect(stateOf("disabled")).toBeNull();
  });
});

describe("limitOf", () => {
  it("keeps a size the pager offers", () => {
    for (const size of USER_PAGE_SIZES) {
      expect(limitOf(size)).toBe(size);
    }
  });

  it("answers the first size for one it does not", () => {
    expect(limitOf(0)).toBe(USER_PAGE_SIZES[0]);
    expect(limitOf(15)).toBe(USER_PAGE_SIZES[0]);
    expect(limitOf(1000)).toBe(USER_PAGE_SIZES[0]);
  });
});

describe("pageOf", () => {
  it("keeps a page the contract takes", () => {
    expect(pageOf(1)).toBe(1);
    expect(pageOf(26)).toBe(26);
    expect(pageOf(PAGE_MAX)).toBe(PAGE_MAX);
  });

  it("stops at the highest page the contract takes", () => {
    expect(pageOf(PAGE_MAX + 1)).toBe(PAGE_MAX);
    expect(pageOf(Number.MAX_SAFE_INTEGER)).toBe(PAGE_MAX);
  });
});

describe("searchOf", () => {
  it("is empty while the text is under the floor", () => {
    expect(SEARCH_MIN).toBe(3);
    expect(searchOf("")).toBe("");
    expect(searchOf("ng")).toBe("");
    expect(searchOf("  ng  ")).toBe("");
  });

  it("sends the text, trimmed, from the floor up", () => {
    expect(searchOf("ngo")).toBe("ngo");
    expect(searchOf("  nghe an  ")).toBe("nghe an");
  });

  it("does not count the space around the text", () => {
    expect(searchOf("   a b   ")).toBe("a b");
    expect(searchOf("  ab ")).toBe("");
  });

  it("counts a character once, composed or not, and sends it composed", () => {
    const composed = "Đức";
    const decomposed = "Đức".normalize("NFD");
    expect(decomposed.length).toBeGreaterThan(composed.length);
    expect(searchOf(composed)).toBe(composed);
    expect(searchOf(decomposed)).toBe(composed);
    expect(searchOf("Đứ".normalize("NFD"))).toBe("");
  });

  it("cuts the text to the longest search the contract takes", () => {
    expect(SEARCH_MAX).toBe(200);
    expect(searchOf("a".repeat(SEARCH_MAX))).toBe("a".repeat(SEARCH_MAX));
    expect(searchOf("a".repeat(SEARCH_MAX + 50))).toBe("a".repeat(SEARCH_MAX));
    expect([...searchOf("😀".repeat(SEARCH_MAX + 5))]).toHaveLength(SEARCH_MAX);
    expect(searchOf(`${"a".repeat(SEARCH_MAX - 1)} b`)).toBe(
      "a".repeat(SEARCH_MAX - 1),
    );
  });

  it("measures the longest search after composing, so decomposed text is not cut early", () => {
    const decomposed = "ế".repeat(SEARCH_MAX).normalize("NFD");
    expect(decomposed.length).toBeGreaterThan(SEARCH_MAX);
    expect(searchOf(decomposed)).toBe("ế".repeat(SEARCH_MAX));
  });

  it("counts a code point once, whatever its UTF-16 length", () => {
    expect(searchOf("😀😀")).toBe("");
    expect(searchOf("😀😀😀")).toBe("😀😀😀");
  });
});

describe("isTooShort", () => {
  it("is false for nothing typed, and for blanks", () => {
    expect(isTooShort("")).toBe(false);
    expect(isTooShort("   ")).toBe(false);
  });

  it("is true for one or two characters", () => {
    expect(isTooShort("n")).toBe(true);
    expect(isTooShort(" ng ")).toBe(true);
  });

  it("is false from the floor up", () => {
    expect(isTooShort("ngo")).toBe(false);
    expect(isTooShort("Đức".normalize("NFD"))).toBe(false);
  });

  it("agrees with searchOf: exactly one of them speaks for a non-blank text", () => {
    for (const typed of ["a", "ab", "abc", "abcd", "  a  ", "Đứ".normalize("NFD")]) {
      expect(isTooShort(typed)).toBe(searchOf(typed) === "");
    }
  });
});
