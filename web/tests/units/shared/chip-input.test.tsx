import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChipInput } from "@/components/shared/form/ChipInput";
import {
  chipText,
  sameFolded,
  sameIgnoringCase,
  withChip,
  type ChipSame,
} from "@/components/shared/form/chips";
import "@/lib/i18n";

const NAME = "Đáp án chấp nhận";
const DECOMPOSED = "nghé";
const COMPOSED = "nghé";
const input = () => screen.getByRole<HTMLInputElement>("textbox", { name: NAME });
const chips = () =>
  [...document.querySelectorAll('[data-slot="chip"]')].map((chip) => chip.textContent);
const chip = (text: string) =>
  [...document.querySelectorAll('[data-slot="chip"]')].find(
    (node) => node.textContent === text,
  )!;
const box = () => document.querySelector('[data-slot="chip-box"]')!;

function Answers({
  start = ["travel"],
  onChange = () => {},
  ...props
}: Readonly<{
  start?: readonly string[];
  onChange?: (next: string[]) => void;
  tone?: "neutral" | "success";
  size?: "sm" | "md";
  frame?: boolean;
  commaAdds?: boolean;
  invalid?: boolean;
  describedBy?: string;
  same?: ChipSame;
}>) {
  const [values, setValues] = useState(start);
  return (
    <>
      <ChipInput
        label={NAME}
        values={values}
        placeholder="Nhập đáp án rồi nhấn Enter"
        removeLabel={(value) => `Bỏ đáp án ${value}`}
        onChange={(next) => {
          onChange(next);
          setValues(next);
        }}
        {...props}
      />
      <button type="button">Tiếp tục</button>
    </>
  );
}

async function enter(text: string) {
  const user = userEvent.setup();
  await user.click(input());
  await user.paste(text);
  await user.keyboard("{Enter}");
}

describe("the chip input", () => {
  it("draws each value as a chip beside a named input", () => {
    render(<Answers start={["travel", "trip"]} />);
    expect(chips()).toEqual(["travel", "trip"]);
    expect(input()).toHaveAttribute("placeholder", "Nhập đáp án rồi nhấn Enter");
    expect(input()).toHaveValue("");
    expect(input()).toHaveAttribute("autocomplete", "off");
  });

  it("adds what is typed on Enter and empties the input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.type(input(), "journey{Enter}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["travel", "journey"]);
    expect(chips()).toEqual(["travel", "journey"]);
    expect(input()).toHaveValue("");
    expect(input()).toHaveFocus();
  });

  it("does not submit a form on Enter", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
      event.preventDefault(),
    );
    render(
      <form onSubmit={onSubmit}>
        <Answers />
      </form>,
    );
    await user.type(input(), "journey{Enter}{Enter}");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("adds what is typed when a comma follows it", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.type(input(), "journey,");
    expect(onChange).toHaveBeenCalledWith(["travel", "journey"]);
    expect(input()).toHaveValue("");
    await user.type(input(), "tr");
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("adds every part before a comma of what is pasted and keeps the rest to type on", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.click(input());
    await user.paste("journey, Travel, trip,voy");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["travel", "journey", "trip"]);
    expect(input()).toHaveValue("voy");
  });

  it("keeps a comma in the value when a comma does not add", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers commaAdds={false} onChange={onChange} />);
    await user.type(input(), "Hanoi, Vietnam");
    expect(onChange).not.toHaveBeenCalled();
    expect(input()).toHaveValue("Hanoi, Vietnam");
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith(["travel", "Hanoi, Vietnam"]);
  });

  it("adds what is typed when the input loses focus", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.type(input(), "journey");
    expect(onChange).not.toHaveBeenCalled();
    await user.tab();
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["travel", "journey"]);
    expect(input()).toHaveValue("");
  });

  it("adds nothing for a blank entry", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.type(input(), "{Enter}");
    await user.type(input(), "   {Enter}");
    await user.type(input(), " , ,");
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
    expect(chips()).toEqual(["travel"]);
    expect(input()).toHaveValue("");
  });

  it("trims an entry before it adds it", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await user.type(input(), "  green space  {Enter}");
    expect(onChange).toHaveBeenCalledWith(["travel", "green space"]);
  });
});

describe("an entry that is already there", () => {
  it("is not added twice when only its case differs, by default", async () => {
    const onChange = vi.fn();
    render(<Answers onChange={onChange} />);
    await enter("Travel");
    expect(onChange).not.toHaveBeenCalled();
    expect(chips()).toEqual(["travel"]);
    expect(input()).toHaveValue("");
  });

  it("is added when its accents differ, by default", async () => {
    const onChange = vi.fn();
    render(<Answers start={[COMPOSED]} onChange={onChange} />);
    await enter("nghe");
    expect(onChange).toHaveBeenCalledWith([COMPOSED, "nghe"]);
    expect(chips()).toEqual([COMPOSED, "nghe"]);
  });

  it("is not added when its accents differ and the caller compares folded", async () => {
    const onChange = vi.fn();
    render(<Answers start={[COMPOSED]} same={sameFolded} onChange={onChange} />);
    await enter("Nghe");
    expect(onChange).not.toHaveBeenCalled();
    expect(chips()).toEqual([COMPOSED]);
  });

  it("is added when only its case differs and the caller compares exactly", async () => {
    const onChange = vi.fn();
    render(<Answers start={["london"]} same={(a, b) => a === b} onChange={onChange} />);
    await enter("London");
    expect(onChange).toHaveBeenCalledWith(["london", "London"]);
    onChange.mockClear();
    await enter("London");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("is not added when it is the same word typed with combining marks", async () => {
    const onChange = vi.fn();
    render(<Answers start={[COMPOSED]} onChange={onChange} />);
    await enter(DECOMPOSED);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("is stored composed when it was typed with combining marks", async () => {
    const onChange = vi.fn();
    render(<Answers start={[]} same={(a, b) => a === b} onChange={onChange} />);
    await enter(DECOMPOSED);
    expect(onChange).toHaveBeenCalledWith([COMPOSED]);
  });
});

describe("removing a chip", () => {
  it("removes the last value on Backspace in the empty input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers start={["travel", "trip"]} onChange={onChange} />);
    await user.click(input());
    await user.keyboard("{Backspace}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["travel"]);
    await user.keyboard("{Backspace}");
    expect(onChange).toHaveBeenLastCalledWith([]);
    await user.keyboard("{Backspace}");
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it("only deletes text on Backspace while the input holds some", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers start={["travel", "trip"]} onChange={onChange} />);
    await user.type(input(), "ab{Backspace}");
    expect(input()).toHaveValue("a");
    expect(onChange).not.toHaveBeenCalled();
    expect(chips()).toEqual(["travel", "trip"]);
  });

  it("removes nothing while Backspace is held down", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers start={["travel", "trip"]} onChange={onChange} />);
    await user.click(input());
    fireEvent.keyDown(input(), { key: "Backspace", repeat: true });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("removes its own chip from the button the caller names, and returns to the input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers start={["travel", "trip", "journey"]} onChange={onChange} />);
    const remove = screen.getByRole("button", { name: "Bỏ đáp án trip" });
    expect(remove).toHaveAttribute("type", "button");
    await user.click(remove);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["travel", "journey"]);
    expect(chips()).toEqual(["travel", "journey"]);
    expect(input()).toHaveFocus();
  });

  it("removes one of two equal values, not both", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Answers start={["trip", "trip"]} onChange={onChange} />);
    await user.click(screen.getAllByRole("button", { name: "Bỏ đáp án trip" })[1]!);
    expect(onChange).toHaveBeenCalledWith(["trip"]);
  });
});

describe("the chip input's drawing", () => {
  it("is the framed 22px neutral field unless it is told otherwise", () => {
    render(<Answers />);
    expect(box()).toHaveClass(
      "flex",
      "flex-wrap",
      "items-center",
      "gap-1.25",
      "min-h-9",
      "rounded-md",
      "border",
      "border-input",
      "bg-bg",
      "px-1.5",
      "py-1.25",
      "focus-within:border-ring",
      "has-[input:focus-visible]:outline-2",
      "has-[input:focus-visible]:outline-focus",
    );
    expect(chip("travel")).toHaveClass(
      "h-5.5",
      "pl-2",
      "pr-0.75",
      "text-xs",
      "rounded-full",
      "border",
      "font-medium",
      "bg-muted",
      "text-fg",
    );
    expect(screen.getByRole("button", { name: "Bỏ đáp án travel" })).toHaveClass(
      "size-4",
      "rounded-full",
      "text-muted-fg",
      "hover:bg-hover",
      "hover:text-fg",
    );
    expect(input()).toHaveClass(
      "h-5.5",
      "min-w-30",
      "px-1",
      "flex-[1_1_140px]",
      "outline-none!",
      "text-[length:var(--text-input)]",
      "lg:text-sm",
    );
  });

  it("puts the success tone and the 24px size on the chip", () => {
    render(<Answers tone="success" size="md" />);
    expect(chip("travel")).toHaveClass(
      "h-6",
      "pl-2.25",
      "text-meta",
      "bg-success-soft",
      "text-success-ink",
    );
    expect(chip("travel")).not.toHaveClass("bg-muted");
    expect(chip("travel")).not.toHaveClass("h-5.5");
    const remove = screen.getByRole("button", { name: "Bỏ đáp án travel" });
    expect(remove).toHaveClass("size-4.5");
    expect(remove).not.toHaveClass("text-muted-fg");
    expect(remove.querySelector("svg")).toHaveClass("size-2.75");
    expect(input()).toHaveClass("h-6.5", "min-w-35");
  });

  it("draws no box of its own without a frame, and leaves the input its focus ring", () => {
    render(<Answers frame={false} size="md" tone="success" />);
    expect(box()).toHaveClass("flex", "flex-wrap", "gap-1.25", "min-h-7", "flex-1");
    expect(box().className).not.toMatch(/border|bg-|rounded|outline|p[xy]-/);
    expect(input()).not.toHaveClass("outline-none!");
  });

  it("marks an invalid field on the input and on the frame", () => {
    const valid = render(<Answers />);
    expect(input()).not.toHaveAttribute("aria-invalid");
    expect(box()).not.toHaveClass("border-danger");
    valid.unmount();

    render(<Answers invalid />);
    expect(input()).toHaveAttribute("aria-invalid", "true");
    expect(box()).toHaveClass("border-danger");
    expect(box()).not.toHaveClass("border-input");
  });

  it("is described by the message its host names, and by nothing otherwise", () => {
    const plain = render(<Answers />);
    expect(input()).not.toHaveAttribute("aria-describedby");
    plain.unmount();

    render(
      <>
        <Answers invalid describedBy="answers-error" />
        <p id="answers-error">Thêm ít nhất một đáp án.</p>
      </>,
    );
    expect(input()).toHaveAccessibleDescription("Thêm ít nhất một đáp án.");
  });
});

describe("the chip comparisons", () => {
  it("compares lower-cased by default, so accents count", () => {
    expect(sameIgnoringCase("Travel", "travel")).toBe(true);
    expect(sameIgnoringCase("NGHÉ", "nghé")).toBe(true);
    expect(sameIgnoringCase("nghe", "nghé")).toBe(false);
    expect(sameIgnoringCase("đi", "di")).toBe(false);
  });

  it("compares folded for tags, so accents and case do not count", () => {
    expect(sameFolded("nghe", "Nghé")).toBe(true);
    expect(sameFolded("Đi", "di")).toBe(true);
    expect(sameFolded(DECOMPOSED, COMPOSED)).toBe(true);
    expect(sameFolded("nghe", "nghi")).toBe(false);
  });

  it("stores an entry trimmed and in NFC", () => {
    expect(chipText("  green space ")).toBe("green space");
    expect(chipText(DECOMPOSED)).toBe(COMPOSED);
    expect(chipText("   ")).toBe("");
  });

  it("adds an entry at the end, or nothing for a blank or a repeated one", () => {
    const values = ["travel"] as const;
    expect(withChip(values, " trip ", sameIgnoringCase)).toEqual(["travel", "trip"]);
    expect(withChip(values, "TRAVEL", sameIgnoringCase)).toBeNull();
    expect(withChip(values, "TRAVEL", (a, b) => a === b)).toEqual(["travel", "TRAVEL"]);
    expect(withChip(values, "  ", sameIgnoringCase)).toBeNull();
    expect(values).toEqual(["travel"]);
  });
});
