import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Checkbox } from "@/components/ui/checkbox";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);

const BASE = [
  "checked:bg-primary",
  "checked:border-primary",
  "relative",
  "inline-grid",
  "size-4",
  "flex-none",
  "cursor-pointer",
  "appearance-none",
  "place-content-center",
  "border",
  "transition-colors",
  "disabled:cursor-not-allowed",
  "disabled:opacity-50",
  "checked:after:bg-primary-foreground",
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
const REPLACED = [
  "border-input",
  "bg-background",
  "rounded-sm",
  "checked:after:size-2.5",
];

const box = () => screen.getByRole<HTMLInputElement>("checkbox", { name: "Chọn hàng" });

describe("the checkbox's square", () => {
  it.each([
    ["unchecked", false, false],
    ["checked", true, false],
    ["mixed", false, true],
  ] as const)(
    "is the deck's when %s: radius 4, the ring colour on the card, and the bar for the mixed state",
    (_, checked, mixed) => {
      render(<Checkbox aria-label="Chọn hàng" checked={checked} onChange={() => {}} />);
      box().indeterminate = mixed;
      expect(box().checked).toBe(checked);
      if (mixed) expect(box()).toBePartiallyChecked();
      else expect(box()).not.toBePartiallyChecked();
      expect(classes(box())).toEqual([...BASE, ...INDETERMINATE, ...DECK]);
      for (const replaced of REPLACED) expect(classes(box())).not.toContain(replaced);
    },
  );

  it("fills from the primary colour when checked or mixed, over the deck's empty square", () => {
    render(<Checkbox aria-label="Chọn hàng" />);
    expect(classes(box())).toEqual(
      expect.arrayContaining([
        "checked:bg-primary",
        "checked:border-primary",
        "indeterminate:bg-primary",
        "indeterminate:border-primary",
      ]),
    );
    expect(classes(box()).filter((c) => /^(bg-|border-)/.test(c))).toEqual([
      "border-ring",
      "bg-card",
    ]);
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
    render(<Checkbox aria-label="Chọn hàng" />);
    expect(classes(box()).filter((c) => c.includes("outline"))).toEqual([]);
  });

  it("cannot be toggled when disabled", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Checkbox aria-label="Chọn hàng" disabled onChange={onChange} />);
    expect(box()).toBeDisabled();
    expect(classes(box())).toEqual(
      expect.arrayContaining(["disabled:cursor-not-allowed", "disabled:opacity-50"]),
    );
    await user.click(box());
    expect(onChange).not.toHaveBeenCalled();
    expect(box()).not.toBeChecked();
  });

  it("does not double the mixed recipe when a caller still passes it", () => {
    render(<Checkbox aria-label="Chọn hàng" className={INDETERMINATE.join(" ")} />);
    expect(classes(box())).toEqual([...BASE, ...DECK, ...INDETERMINATE]);
  });
});
