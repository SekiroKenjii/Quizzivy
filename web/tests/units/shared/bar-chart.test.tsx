import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { BarChart } from "@/components/shared/charts/BarChart";
import { barHeights, labelShown } from "@/components/shared/charts/bars";

const CHART = [6, 9, 4, 12, 15, 3, 2, 8, 11, 14, 18, 10, 13, 17];
const DAYS = CHART.map((_, index) => String(11 + index));
const DATA = CHART.map((value, index) => ({
  key: DAYS[index]!,
  label: DAYS[index]!,
  value,
  title: `${value} bài nộp · ${DAYS[index]} thg 9`,
}));
const HEADERS = ["Ngày", "Số bài nộp"] as const;
const CAPTION = "Bài nộp trong 14 ngày qua";

function chart(props: Partial<Parameters<typeof BarChart>[0]> = {}) {
  const { container } = render(
    <BarChart caption={CAPTION} columns={HEADERS} data={DATA} {...props} />,
  );
  const columns = [...container.querySelectorAll<HTMLElement>("[title]")];
  const drawing = columns[0]?.closest('[aria-hidden="true"]') ?? null;
  return {
    container,
    columns,
    drawing,
    bars: columns.map((column) => column.firstElementChild as HTMLElement),
    slots: drawing === null ? [] : [...drawing.lastElementChild!.children],
  };
}

describe("BarChart's text alternative", () => {
  it("is a table with the caption, both headers and one row per column", () => {
    chart();
    const table = screen.getByRole("table", { name: CAPTION });
    expect(table.querySelector("caption")).toHaveTextContent(CAPTION);
    expect(
      within(table)
        .getAllByRole("columnheader")
        .map((header) => header.textContent),
    ).toEqual(["Ngày", "Số bài nộp"]);
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(14);
    expect(
      rows.map((row) => [
        within(row).getByRole("rowheader").textContent,
        within(row).getByRole("cell").textContent,
      ]),
    ).toEqual(CHART.map((value, index) => [DAYS[index], String(value)]));
  });

  it("is hidden from the eye and not from assistive technology", () => {
    chart();
    const table = screen.getByRole("table", { name: CAPTION });
    expect(table.parentElement).toHaveClass("sr-only");
    expect(table.closest("[aria-hidden]")).toBeNull();
  });

  it("positions the chart, so the hidden table scrolls with it and never lengthens the page", () => {
    const { container } = chart();
    const table = screen.getByRole("table", { name: CAPTION });
    expect(table.parentElement!.parentElement).toBe(container.firstElementChild);
    expect(container.firstElementChild).toHaveClass("relative");
  });
});

describe("BarChart's drawing", () => {
  it("is hidden from assistive technology, bars and labels alike", () => {
    const { columns, drawing, slots } = chart();
    expect(drawing).not.toBeNull();
    expect(columns).toHaveLength(14);
    for (const column of columns) expect(drawing).toContainElement(column);
    expect(slots).toHaveLength(14);
    for (const slot of slots) expect(drawing).toContainElement(slot as HTMLElement);
    expect(drawing).not.toContainElement(screen.getByRole("table"));
  });

  it("keeps a title on each column for a pointer", () => {
    const { columns } = chart();
    expect(columns.map((column) => column.title)).toEqual(
      DATA.map((datum) => datum.title),
    );
    expect(columns[10]!.title).toBe("18 bài nộp · 21 thg 9");
  });

  it("draws only the last bar in the accent colour", () => {
    const { bars } = chart();
    expect(bars.filter((bar) => bar.classList.contains("bg-brand"))).toEqual([
      bars[13],
    ]);
    expect(bars.filter((bar) => bar.classList.contains("bg-hover"))).toEqual(
      bars.slice(0, 13),
    );
  });

  it("takes a class from the caller on its root", () => {
    const { container } = chart({ className: "mt-4.5" });
    expect(container.firstElementChild).toHaveClass("mt-4.5", "relative");
  });

  it("gives each column the row's full height for its bar to be a share of", () => {
    const { columns } = chart();
    expect(columns).toHaveLength(14);
    for (const column of columns) expect(column).toHaveClass("h-full");
  });

  it("scales the bars to the largest value", () => {
    const { bars } = chart();
    expect(bars[10]!.style.height).toBe("100%");
    expect(bars[1]!.style.height).toBe("50%");
    expect(bars[0]!.style.height).toBe("33%");
    expect(bars[13]!.style.height).toBe("94%");
  });

  it("draws every bar at 0 when nothing was counted, with no NaN", () => {
    const { bars } = chart({ data: DATA.map((datum) => ({ ...datum, value: 0 })) });
    expect(bars.map((bar) => bar.style.height)).toEqual(
      Array.from({ length: 14 }, () => "0%"),
    );
    expect(bars.map((bar) => bar.getAttribute("style")).join(" ")).not.toContain("NaN");
  });

  it("labels every column by default", () => {
    const { slots } = chart();
    expect(slots.map((slot) => slot.textContent)).toEqual(DAYS);
  });

  it("labels the odd indexes at labelEvery 2 and keeps fourteen slots", () => {
    const { slots } = chart({ labelEvery: 2 });
    expect(slots).toHaveLength(14);
    expect(slots.map((slot) => slot.textContent)).toEqual([
      "",
      "12",
      "",
      "14",
      "",
      "16",
      "",
      "18",
      "",
      "20",
      "",
      "22",
      "",
      "24",
    ]);
    for (const slot of slots) expect(slot).toHaveClass("flex-1", "min-w-0");
  });

  it("does not animate the bars under reduced motion", () => {
    const { bars } = chart();
    for (const bar of bars)
      expect(bar).toHaveClass(
        "transition-[height]",
        "duration-200",
        "motion-reduce:transition-none",
      );
  });

  it("renders an empty chart without a bar", () => {
    const { columns } = chart({ data: [] });
    expect(columns).toEqual([]);
    expect(screen.getByRole("table", { name: CAPTION })).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(1);
  });
});

describe("barHeights", () => {
  it("is each value as a whole percentage of the largest", () => {
    expect(barHeights([6, 9, 18])).toEqual([33, 50, 100]);
    expect(barHeights(CHART)).toEqual([
      33, 50, 22, 67, 83, 17, 11, 44, 61, 78, 100, 56, 72, 94,
    ]);
    expect(barHeights([7])).toEqual([100]);
  });

  it("is all zeros when the largest value is 0, and empty for no values", () => {
    expect(barHeights([0, 0, 0])).toEqual([0, 0, 0]);
    expect(barHeights([])).toEqual([]);
  });

  it("draws nothing for a value below zero or one that is not a number", () => {
    expect(barHeights([-3, 6])).toEqual([0, 100]);
    expect(barHeights([Number.NaN, 4, 2])).toEqual([0, 100, 50]);
    expect(barHeights([-1, -2])).toEqual([0, 0]);
  });
});

describe("labelShown", () => {
  it("shows every label at 1", () => {
    expect([0, 1, 2, 3].map((index) => labelShown(index, 1))).toEqual([
      true,
      true,
      true,
      true,
    ]);
  });

  it("shows the odd indexes at 2", () => {
    expect([0, 1, 2, 3, 4, 5].map((index) => labelShown(index, 2))).toEqual([
      false,
      true,
      false,
      true,
      false,
      true,
    ]);
  });

  it("shows the last of each run of three at 3", () => {
    expect([0, 1, 2, 3, 4, 5].map((index) => labelShown(index, 3))).toEqual([
      false,
      false,
      true,
      false,
      false,
      true,
    ]);
  });

  it("counts a step below 1 as 1", () => {
    expect(labelShown(0, 0)).toBe(true);
    expect(labelShown(3, -2)).toBe(true);
    expect(labelShown(1, Number.NaN)).toBe(true);
  });
});
