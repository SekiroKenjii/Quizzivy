import { describe, expect, it } from "vitest";
import {
  byUse,
  lastDayOf,
  monthName,
  paceFigure,
  poolTone,
  projectionAt,
  teacherTone,
  withOption,
} from "@/features/ai-credits/figures";
import { bao, duc, giau, khai, linh, trang } from "@tests/support/aiCredits";

describe("the pool bar's tone", () => {
  it.each([
    [74, "accent"],
    [75, "warning"],
    [89, "warning"],
    [90, "danger"],
    [100, "danger"],
  ])("at %i% of the pool it is %s", (used, tone) => {
    expect(poolTone(used, 100)).toBe(tone);
  });

  it("compares the shares exactly, so 74.9% is still the accent and 89.9% still warning", () => {
    expect(poolTone(1498, 2000)).toBe("accent");
    expect(poolTone(1797, 2000)).toBe("warning");
    expect(poolTone(1800, 2000)).toBe("danger");
  });

  it("is the accent for an empty month and danger for use against a pool of nothing", () => {
    expect(poolTone(0, 0)).toBe("accent");
    expect(poolTone(1, 0)).toBe("danger");
  });
});

describe("the projection's tick", () => {
  it("sits at the projected share of the pool", () => {
    expect(projectionAt(156050, 200000)).toBeCloseTo(78.025);
  });

  it("stays inside the track when the projection passes the pool", () => {
    expect(projectionAt(500000, 200000)).toBe(99.5);
  });

  it("is absent with no pool or no projected use", () => {
    expect(projectionAt(100, 0)).toBeNull();
    expect(projectionAt(0, 200000)).toBeNull();
  });

  it("is written to the nearest hundred once it is that large", () => {
    expect(paceFigure(156050)).toBe(156100);
    expect(paceFigure(30)).toBe(30);
    expect(paceFigure(99)).toBe(99);
    expect(paceFigure(100)).toBe(100);
  });
});

describe("a teacher's bar", () => {
  it("takes its colour from the standing the server reads", () => {
    expect(teacherTone("on_track")).toBe("accent");
    expect(teacherTone("near_limit")).toBe("warning");
    expect(teacherTone("at_limit")).toBe("danger");
    expect(teacherTone("paused")).toBe("danger");
  });
});

describe("teachers by use", () => {
  it("puts the largest share of the allowance first and does not touch its input", () => {
    const rows = [giau, duc, bao, trang, khai, linh];
    expect(byUse(rows, "vi").map((row) => row.name)).toEqual([
      "Trương Văn Khải",
      "Phan Minh Đức",
      "Đinh Quốc Bảo",
      "Giàu Bùi",
      "Ngô Thu Trang",
      "Hà Mỹ Linh",
    ]);
    expect(rows[0]).toBe(giau);
  });

  it("orders equal shares by name and survives an allowance of nothing", () => {
    const empty = { ...giau, userId: "a", name: "An", allowance: 0, used: 0 };
    const over = { ...giau, userId: "b", name: "Bình", allowance: 0, used: 5 };
    const same = { ...giau, userId: "c", name: "Chi", allowance: 0, used: 0 };
    expect(byUse([same, empty, over], "vi").map((row) => row.name)).toEqual([
      "Bình",
      "An",
      "Chi",
    ]);
  });
});

describe("the month", () => {
  it("names itself and its last day in the reader's language, from the calendar date alone", () => {
    expect(monthName("2026-09-01", "en")).toBe("September");
    expect(monthName("2026-09-01", "vi")).toBe("Tháng 9");
    expect(lastDayOf("2026-09-01", "en")).toBe("30 September");
    expect(lastDayOf("2026-09-01", "vi")).toBe("30 tháng 9");
    expect(lastDayOf("2026-02-01", "en")).toBe("28 February");
    expect(lastDayOf("2028-02-01", "en")).toBe("29 February");
    expect(lastDayOf("2026-12-01", "en")).toBe("31 December");
  });
});

describe("a rule set to a figure the buttons do not offer", () => {
  it("joins the options in numeric order, and a figure they offer adds nothing", () => {
    expect(withOption([20000, 50000, 100000], 30000)).toEqual([
      20000, 30000, 50000, 100000,
    ]);
    expect(withOption([80, 90], 95)).toEqual([80, 90, 95]);
    const offered = [80, 90] as const;
    expect(withOption(offered, 90)).toBe(offered);
  });
});
