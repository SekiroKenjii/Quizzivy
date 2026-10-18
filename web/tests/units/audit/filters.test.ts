import { describe, expect, it } from "vitest";
import { dotOf, humanise, isGone, sentenceOf } from "@/features/audit/actions";
import {
  AUDIT_SEARCH_MAX,
  DEFAULT_RANGE,
  hasNarrowing,
  isDay,
  isFiltered,
  lastSevenDays,
  readAuditFilters,
} from "@/features/audit/filters";
import { auditDay, auditMoment } from "@/lib/i18n/datetime";

const read = (search: string) => readAuditFilters(new URLSearchParams(search));

describe("reading the filters from an address", () => {
  it("opens on every category, no search and the last 7 days", () => {
    expect(read("")).toEqual({
      category: null,
      q: "",
      range: DEFAULT_RANGE,
      from: null,
      to: null,
    });
    expect(DEFAULT_RANGE).toBe("7d");
  });

  it("reads a category, a search and a named range", () => {
    expect(read("category=tests&q=nam%20nguyen&range=30d")).toEqual({
      category: "tests",
      q: "nam nguyen",
      range: "30d",
      from: null,
      to: null,
    });
  });

  it("cuts a search to the 200 characters the contract takes, by character and not by code unit", () => {
    expect(AUDIT_SEARCH_MAX).toBe(200);
    expect(read(`q=${"a".repeat(201)}`).q).toBe("a".repeat(200));
    const emoji = "😀".repeat(201);
    expect(read(`q=${encodeURIComponent(emoji)}`).q).toBe("😀".repeat(200));
    expect(read(`q=${"b".repeat(200)}`).q).toBe("b".repeat(200));
  });

  it.each(["today", "7d", "30d", "all"])("accepts the %s range", (range) => {
    expect(read(`range=${range}`).range).toBe(range);
  });

  it("drops a category and a range the screen does not offer", () => {
    expect(read("category=audit&range=yesterday")).toMatchObject({
      category: null,
      range: "7d",
    });
  });

  it("keeps a custom range with both days, the second not before the first", () => {
    expect(read("range=custom&from=2026-09-18&to=2026-09-24")).toMatchObject({
      range: "custom",
      from: "2026-09-18",
      to: "2026-09-24",
    });
    expect(read("range=custom&from=2026-09-24&to=2026-09-24").range).toBe("custom");
  });

  it.each([
    ["a missing day", "range=custom&from=2026-09-18"],
    ["days the wrong way round", "range=custom&from=2026-09-24&to=2026-09-18"],
    ["a day that does not exist", "range=custom&from=2026-02-30&to=2026-03-02"],
    ["days that are not days", "range=custom&from=x&to=y"],
  ])("falls back to the default for %s", (_, search) => {
    expect(read(search)).toMatchObject({ range: "7d", from: null, to: null });
  });

  it("ignores from and to on a range that is not custom", () => {
    expect(read("range=30d&from=2026-09-18&to=2026-09-24")).toMatchObject({
      range: "30d",
      from: null,
      to: null,
    });
  });
});

describe("what counts as a day", () => {
  it.each(["2026-09-24", "2024-02-29"])("accepts %s", (value) => {
    expect(isDay(value)).toBe(true);
  });

  it.each([null, "", "2026-9-24", "2026-13-01", "2025-02-29", "24/09/2026"])(
    "refuses %s",
    (value) => {
      expect(isDay(value)).toBe(false);
    },
  );
});

describe("what a reset clears and what an export narrows by", () => {
  const base = read("");

  it("is filtered by a search or a range other than the default, and not by a category", () => {
    expect(isFiltered(base)).toBe(false);
    expect(isFiltered(read("category=tests"))).toBe(false);
    expect(isFiltered(read("q=nam"))).toBe(true);
    expect(isFiltered(read("range=all"))).toBe(true);
    expect(isFiltered(read("q=%20%20"))).toBe(false);
  });

  it("narrows by a category or a search, and not by a range", () => {
    expect(hasNarrowing(base)).toBe(false);
    expect(hasNarrowing(read("range=30d"))).toBe(false);
    expect(hasNarrowing(read("category=classes"))).toBe(true);
    expect(hasNarrowing(read("q=nam"))).toBe(true);
    expect(hasNarrowing(read("q=%20"))).toBe(false);
  });
});

describe("the days a custom range opens on", () => {
  it("is the six days before today and today, in the viewer's zone", () => {
    expect(lastSevenDays(new Date("2026-09-24T05:00:00Z"), "Asia/Ho_Chi_Minh")).toEqual(
      {
        from: "2026-09-18",
        to: "2026-09-24",
      },
    );
  });

  it("takes today from the zone, not from UTC", () => {
    const instant = new Date("2026-09-24T18:00:00Z");
    expect(lastSevenDays(instant, "Asia/Ho_Chi_Minh").to).toBe("2026-09-25");
    expect(lastSevenDays(instant, "UTC").to).toBe("2026-09-24");
  });

  it("crosses a month and a year", () => {
    expect(lastSevenDays(new Date("2026-01-03T05:00:00Z"), "UTC")).toEqual({
      from: "2025-12-28",
      to: "2026-01-03",
    });
  });
});

describe("when a missing label means the row is gone", () => {
  const id = "019535d9-3df7-79fb-b466-fa907fa17f10";

  it.each(["class", "test", "user", "media_asset", "word_import", "question_group"])(
    "says so of a %s the server could have named",
    (type) => {
      expect(isGone({ type, id })).toBe(true);
    },
  );

  it.each([
    "role",
    "question",
    "data_export",
    "user_import",
    "ai_settings",
    "anything",
  ])("says nothing of a %s, which the server never names", (type) => {
    expect(isGone({ type, id })).toBe(false);
  });

  it("says nothing of an entry about no single row", () => {
    expect(isGone({ type: "class", id: null })).toBe(false);
  });
});

describe("wording an action", () => {
  const sentences = { "class.created": "Đã tạo lớp" };

  it("reads the sentence the table has for the key", () => {
    expect(sentenceOf(sentences, "class.created")).toBe("Đã tạo lớp");
  });

  it("humanises a key the table lacks, as the server does", () => {
    expect(sentenceOf(sentences, "widget.frobnicated_all")).toBe(
      "widget frobnicated all",
    );
    expect(humanise("a.b_c.d")).toBe("a b c d");
  });

  it("draws a dot for each category and a neutral one for none", () => {
    expect(dotOf("accounts")).toBe("bg-danger");
    expect(dotOf("classes")).toBe("bg-info");
    expect(dotOf("attempts")).toBe("bg-warning");
    expect(dotOf("tests")).toBe("bg-success");
    expect(dotOf("media")).toBe("bg-muted-fg");
    expect(dotOf("imports")).toBe("bg-warning");
    expect(dotOf(null)).toBe("bg-muted-fg");
  });
});

describe("the audit log's dates", () => {
  it("words a moment without its year, in the zone it is given", () => {
    expect(auditMoment("2026-09-24T13:31:00Z", "vi", "Asia/Ho_Chi_Minh")).toBe(
      "24 Th9, 20:31",
    );
    expect(auditMoment("2026-09-24T13:31:00Z", "en", "Asia/Ho_Chi_Minh")).toBe(
      "24 Sep, 20:31",
    );
  });

  it("moves to the next day when the zone does", () => {
    expect(auditMoment("2026-09-24T18:00:00Z", "en", "Asia/Ho_Chi_Minh")).toBe(
      "25 Sep, 01:00",
    );
  });

  it("words a day key without its year and without a zone shifting it", () => {
    expect(auditDay("2026-09-24", "vi")).toBe("24 Th9");
    expect(auditDay("2026-09-24", "en")).toBe("24 Sep");
    expect(auditDay("2026-01-01", "en")).toBe("1 Jan");
  });

  it("hands back a key that is not a day", () => {
    expect(auditDay("tomorrow", "vi")).toBe("tomorrow");
  });
});
