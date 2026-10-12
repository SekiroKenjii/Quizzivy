import { describe, expect, it } from "vitest";
import {
  DENSE_CUT,
  MENU_TRACK,
  PAD_X,
  SELECT_TRACK,
  rowBox,
  shownAt,
  templateOf,
  thresholdsOf,
} from "@/components/shared/data/tableLayout";

const ASSIGNMENTS = [
  { id: "title", track: "minmax(200px,2.2fr)" },
  { id: "classes", track: "minmax(130px,1.3fr)", showFrom: 860 },
  { id: "when", track: "120px", showFrom: 700 },
  { id: "submitted", track: "150px", showFrom: 560 },
  { id: "status", track: "110px", showFrom: 760 },
];

const THRESHOLDS = [560, 700, 760, 860];

describe("thresholdsOf", () => {
  it("lists the columns' widths in ascending order whatever order they are declared in", () => {
    expect(thresholdsOf(ASSIGNMENTS)).toEqual(THRESHOLDS);
  });

  it("lists a width two columns share once", () => {
    expect(
      thresholdsOf([
        { id: "a", track: "1fr", showFrom: 700 },
        { id: "b", track: "1fr", showFrom: 560 },
        { id: "c", track: "1fr", showFrom: 700 },
      ]),
    ).toEqual([560, 700]);
  });

  it("sorts by number, not by text", () => {
    expect(
      thresholdsOf([
        { id: "a", track: "1fr", showFrom: 1000 },
        { id: "b", track: "1fr", showFrom: 620 },
      ]),
    ).toEqual([620, 1000]);
  });

  it("is empty for columns that always show", () => {
    expect(thresholdsOf([{ id: "a", track: "1fr" }])).toEqual([]);
  });
});

describe("shownAt", () => {
  it.each([
    [0, ["title"]],
    [1, ["title", "submitted"]],
    [2, ["title", "when", "submitted"]],
    [3, ["title", "when", "submitted", "status"]],
    [4, ["title", "classes", "when", "submitted", "status"]],
  ])("shows the columns whose width band %i meets", (band, ids) => {
    expect([...shownAt(ASSIGNMENTS, THRESHOLDS, band)]).toEqual(ids);
  });

  it("shows both columns that share a width in the same band", () => {
    const columns = [
      { id: "a", track: "1fr", showFrom: 700 },
      { id: "b", track: "1fr", showFrom: 700 },
    ];
    expect([...shownAt(columns, [700], 0)]).toEqual([]);
    expect([...shownAt(columns, [700], 1)]).toEqual(["a", "b"]);
  });
});

describe("templateOf", () => {
  const edges = { select: true, menuTrack: MENU_TRACK };

  it.each([
    [0, "16px minmax(200px,2.2fr) 44px"],
    [1, "16px minmax(200px,2.2fr) 150px 44px"],
    [2, "16px minmax(200px,2.2fr) 120px 150px 44px"],
    [3, "16px minmax(200px,2.2fr) 120px 150px 110px 44px"],
    [4, "16px minmax(200px,2.2fr) minmax(130px,1.3fr) 120px 150px 110px 44px"],
  ])("joins the deck's Assignments tracks for band %i", (band, template) => {
    expect(templateOf(ASSIGNMENTS, shownAt(ASSIGNMENTS, THRESHOLDS, band), edges)).toBe(
      template,
    );
  });

  it("has no checkbox track without selection and no menu track without a menu", () => {
    const shown = shownAt(ASSIGNMENTS, THRESHOLDS, 0);
    expect(templateOf(ASSIGNMENTS, shown, { select: false })).toBe(
      "minmax(200px,2.2fr)",
    );
    expect(templateOf(ASSIGNMENTS, shown, { select: true })).toBe(
      "16px minmax(200px,2.2fr)",
    );
    expect(templateOf(ASSIGNMENTS, shown, { select: false, menuTrack: "32px" })).toBe(
      "minmax(200px,2.2fr) 32px",
    );
  });

  it("keeps the declared order whatever order the set was built in", () => {
    expect(
      templateOf(ASSIGNMENTS, new Set(["status", "title"]), { select: false }),
    ).toBe("minmax(200px,2.2fr) 110px");
  });

  it("ignores an id that names no column", () => {
    expect(templateOf(ASSIGNMENTS, new Set(["title", "gone"]), { select: false })).toBe(
      "minmax(200px,2.2fr)",
    );
  });
});

describe("rowBox", () => {
  it("keeps a fixed height and takes 16px off it when dense", () => {
    expect(rowBox({ height: 56 }, false)).toEqual({ height: 56 });
    expect(rowBox({ height: 56 }, true)).toEqual({ height: 40 });
    expect(rowBox({ height: 54 }, true)).toEqual({ height: 38 });
  });

  it("keeps a minimum height and takes 16px off it when dense", () => {
    expect(rowBox({ minHeight: 60 }, false)).toEqual({ minHeight: 60 });
    expect(rowBox({ minHeight: 60 }, true)).toEqual({ minHeight: 44 });
  });

  it("pads a padded row above and below and halves the padding when dense", () => {
    expect(rowBox({ padY: 10 }, false)).toEqual({ paddingTop: 10, paddingBottom: 10 });
    expect(rowBox({ padY: 10 }, true)).toEqual({ paddingTop: 5, paddingBottom: 5 });
  });
});

describe("the deck's defaults", () => {
  it("are a 16px checkbox track, a 44px menu track, 16px padding and a 16px dense cut", () => {
    expect([SELECT_TRACK, MENU_TRACK, PAD_X, DENSE_CUT]).toEqual([
      "16px",
      "44px",
      16,
      16,
    ]);
  });
});
