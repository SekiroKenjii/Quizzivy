import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Checkbox } from "@/components/ui/checkbox";
import { DeckScale } from "@/components/ui/deck-scale";

const INSIDE = "in-data-[scale=deck]:";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);
const plain = (element: Element) =>
  classes(element).filter((c) => !c.includes("scale=deck"));
const inside = (element: Element) =>
  classes(element)
    .filter((c) => c.startsWith(INSIDE))
    .map((c) => c.slice(INSIDE.length));

const TODAY = [
  "border-input",
  "bg-background",
  "checked:bg-primary",
  "checked:border-primary",
  "relative",
  "inline-grid",
  "size-4",
  "flex-none",
  "cursor-pointer",
  "appearance-none",
  "place-content-center",
  "rounded-sm",
  "border",
  "transition-colors",
  "disabled:cursor-not-allowed",
  "disabled:opacity-50",
  "checked:after:bg-primary-foreground",
  "checked:after:size-2.5",
  "checked:after:content-['']",
  "checked:after:[clip-path:polygon(14%_47%,0_61%,39%_100%,100%_20%,85%_8%,38%_70%)]",
];
const INDETERMINATE = [
  "indeterminate:bg-primary",
  "indeterminate:border-primary",
  "indeterminate:after:bg-primary-foreground",
  "indeterminate:after:h-0.5",
  "indeterminate:after:w-2",
  "indeterminate:after:content-['']",
];
const DECK = [
  "border-ring",
  "bg-card",
  "rounded-[0.25rem]",
  "checked:after:size-[0.6875rem]",
];

const box = () => screen.getByRole<HTMLInputElement>("checkbox", { name: "Chọn hàng" });

describe("the checkbox off a deck surface", () => {
  it("is today's, plus the bar for the mixed state and nothing else", () => {
    render(<Checkbox aria-label="Chọn hàng" />);
    expect(plain(box())).toEqual([...TODAY, ...INDETERMINATE]);
    expect(box().closest("[data-scale]")).toBeNull();
  });

  it.each([
    ["unchecked", false, false],
    ["checked", true, false],
    ["mixed", false, true],
  ] as const)("carries the same recipe when %s", (_, checked, mixed) => {
    render(<Checkbox aria-label="Chọn hàng" checked={checked} onChange={() => {}} />);
    box().indeterminate = mixed;
    expect(box().checked).toBe(checked);
    if (mixed) expect(box()).toBePartiallyChecked();
    else expect(box()).not.toBePartiallyChecked();
    expect(plain(box())).toEqual([...TODAY, ...INDETERMINATE]);
  });
});

describe("the checkbox on a deck surface", () => {
  it.each([
    ["unchecked", false, false],
    ["checked", true, false],
    ["mixed", false, true],
  ] as const)(
    "is the deck's square when %s: radius 4, the ring colour on the card",
    (_, checked, mixed) => {
      render(
        <DeckScale>
          <Checkbox aria-label="Chọn hàng" checked={checked} onChange={() => {}} />
        </DeckScale>,
      );
      box().indeterminate = mixed;
      expect(plain(box())).toEqual([...TODAY, ...INDETERMINATE]);
      expect(inside(box())).toEqual(DECK);
      expect(classes(box()).filter((c) => c.startsWith("data-[scale=deck]:"))).toEqual(
        [],
      );
    },
  );

  it("fills from the primary colour when checked or mixed, over the deck's empty square", () => {
    render(
      <DeckScale>
        <Checkbox aria-label="Chọn hàng" />
      </DeckScale>,
    );
    expect(plain(box())).toEqual(
      expect.arrayContaining([
        "checked:bg-primary",
        "checked:border-primary",
        "indeterminate:bg-primary",
        "indeterminate:border-primary",
      ]),
    );
    expect(
      inside(box()).filter((c) => c.includes("bg-") || c.includes("border-")),
    ).toEqual(["border-ring", "bg-card"]);
  });
});

describe("the checkbox as a control", () => {
  it("stays a native input that toggles from the keyboard", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Checkbox aria-label="Chọn hàng" onChange={onChange} />);
    expect(box().tagName).toBe("INPUT");
    expect(box().type).toBe("checkbox");
    await user.tab();
    expect(box()).toHaveFocus();
    await user.keyboard(" ");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(box()).toBeChecked();
  });

  it("keeps the global focus ring: no class removes its outline", () => {
    render(
      <DeckScale>
        <Checkbox aria-label="Chọn hàng" />
      </DeckScale>,
    );
    expect(classes(box()).filter((c) => c.includes("outline"))).toEqual([]);
  });

  it("cannot be toggled when disabled", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Checkbox aria-label="Chọn hàng" disabled onChange={onChange} />);
    expect(box()).toBeDisabled();
    expect(plain(box())).toEqual(
      expect.arrayContaining(["disabled:cursor-not-allowed", "disabled:opacity-50"]),
    );
    await user.click(box());
    expect(onChange).not.toHaveBeenCalled();
    expect(box()).not.toBeChecked();
  });

  it("does not double the mixed recipe when a caller still passes it", () => {
    render(<Checkbox aria-label="Chọn hàng" className={INDETERMINATE.join(" ")} />);
    expect(plain(box())).toEqual([...TODAY, ...INDETERMINATE]);
  });
});
