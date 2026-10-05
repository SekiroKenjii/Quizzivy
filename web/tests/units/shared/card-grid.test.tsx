import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardGrid } from "@/components/shared/CardGrid";

const ITEMS = [
  { id: "a", label: "Một" },
  { id: "b", label: "Hai" },
  { id: "c", label: "Ba" },
];
const key = (item: (typeof ITEMS)[number]) => item.id;

function Counter({ item }: Readonly<{ item: (typeof ITEMS)[number] }>) {
  const [count, setCount] = useState(0);
  return (
    <button onClick={() => setCount(count + 1)}>
      {item.label}: {count}
    </button>
  );
}

function grid(items = ITEMS, gap?: 10 | 12) {
  return (
    <CardGrid
      label="Đề thi"
      items={items}
      itemKey={key}
      min={290}
      {...(gap === undefined ? {} : { gap })}
    >
      {(item) => <Counter item={item} />}
    </CardGrid>
  );
}

describe("CardGrid", () => {
  it("names an explicit list with ordered single-cell list items", () => {
    render(grid());
    const list = screen.getByRole("list", { name: "Đề thi" });
    expect(list.tagName).toBe("UL");
    expect(list).toHaveAttribute("role", "list");
    expect(screen.getAllByRole("listitem").map((item) => item.textContent)).toEqual([
      "Một: 0",
      "Hai: 0",
      "Ba: 0",
    ]);
    for (const item of screen.getAllByRole("listitem"))
      expect(item).toHaveClass("grid", "min-w-0");
  });

  it.each([290, 300, 240, 160])(
    "uses auto-fill tracks capped to the available width for min %s",
    (min) => {
      render(
        <CardGrid label="Thẻ" items={ITEMS} itemKey={key} min={min}>
          {(item) => item.label}
        </CardGrid>,
      );
      expect(screen.getByRole("list")).toHaveStyle({
        gridTemplateColumns: `repeat(auto-fill, minmax(min(${min}px, 100%), 1fr))`,
      });
    },
  );

  it("uses a 12px gap by default", () => {
    render(grid());
    expect(screen.getByRole("list")).toHaveStyle({ gap: "12px" });
  });

  it("accepts a 10px gap and caller classes", () => {
    render(
      <CardGrid
        label="Thẻ"
        items={ITEMS}
        itemKey={key}
        min={160}
        gap={10}
        className="items-start"
      >
        {(item) => item.label}
      </CardGrid>,
    );
    expect(screen.getByRole("list")).toHaveStyle({ gap: "10px" });
    expect(screen.getByRole("list")).toHaveClass("grid", "items-start");
  });

  it("renders nothing for an empty collection", () => {
    const { container } = render(grid([]));
    expect(container).toBeEmptyDOMElement();
  });

  it("keeps each surviving card's state when an earlier item is removed", async () => {
    const user = userEvent.setup();
    const { rerender } = render(grid());
    await user.click(screen.getByRole("button", { name: "Hai: 0" }));
    rerender(grid(ITEMS.slice(1)));
    expect(screen.getByRole("button", { name: "Hai: 1" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Ba: 0" })).toBeVisible();
  });
});
