import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import {
  clampTo,
  digitsOf,
  keyTarget,
  typedValue,
} from "@/components/shared/form/stepperValue";
import "@/lib/i18n";

const NAME = "Thời gian làm bài";
const spin = () => screen.getByRole<HTMLInputElement>("spinbutton", { name: NAME });
const less = () => screen.getByRole("button", { name: "Giảm" });
const more = () => screen.getByRole("button", { name: "Tăng" });

function Minutes({
  start = 45,
  onChange = () => {},
  disabled = false,
  step = 5,
}: Readonly<{
  start?: number;
  onChange?: (next: number) => void;
  disabled?: boolean;
  step?: number;
}>) {
  const [value, setValue] = useState(start);
  return (
    <>
      <NumberStepper
        label={NAME}
        value={value}
        min={1}
        max={600}
        step={step}
        disabled={disabled}
        format={(minutes) => `${minutes} phút`}
        onChange={(next) => {
          onChange(next);
          setValue(next);
        }}
      />
      <button type="button">Tiếp tục</button>
    </>
  );
}

describe("the stepper's value", () => {
  it("is a spinbutton with its name, its bounds and its formatted text", () => {
    render(<Minutes />);
    expect(spin()).toHaveAttribute("aria-valuenow", "45");
    expect(spin()).toHaveAttribute("aria-valuemin", "1");
    expect(spin()).toHaveAttribute("aria-valuemax", "600");
    expect(spin()).toHaveAttribute("aria-valuetext", "45 phút");
    expect(spin()).toHaveAttribute("inputmode", "numeric");
    expect(spin()).toHaveAttribute("type", "text");
    expect(spin()).toHaveValue("45 phút");
  });

  it("writes the bare number when it is given no format", () => {
    render(
      <NumberStepper label={NAME} value={2} min={1} max={99} onChange={() => {}} />,
    );
    expect(spin()).toHaveValue("2");
    expect(spin()).toHaveAttribute("aria-valuetext", "2");
  });

  it("is drawn at the nearer bound when the value is outside them", () => {
    const high = render(<Minutes start={700} />);
    expect(spin()).toHaveValue("600 phút");
    expect(spin()).toHaveAttribute("aria-valuenow", "600");
    expect(more()).toBeDisabled();
    high.unmount();

    render(<Minutes start={-3} />);
    expect(spin()).toHaveValue("1 phút");
    expect(spin()).toHaveAttribute("aria-valuenow", "1");
    expect(less()).toBeDisabled();
  });

  it("is the one tab stop of the control", async () => {
    const user = userEvent.setup();
    render(<Minutes />);
    await user.tab();
    expect(spin()).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Tiếp tục" })).toHaveFocus();
    expect(less()).toHaveAttribute("tabindex", "-1");
    expect(more()).toHaveAttribute("tabindex", "-1");
  });

  it("keeps the deck's 54px least width, centred and tabular, at 16px below 1024", () => {
    render(<Minutes />);
    const cell = spin().parentElement!;
    expect(cell).toHaveClass(
      "inline-grid",
      "min-w-13.5",
      "font-medium",
      "tabular-nums",
      "text-[length:var(--text-input)]",
      "lg:text-ui",
    );
    expect(spin()).toHaveClass("text-center", "bg-transparent", "border-0", "w-full");
    expect(cell.firstElementChild).toHaveAttribute("aria-hidden", "true");
    expect(cell.firstElementChild).toHaveTextContent("45 phút");
    expect(cell.parentElement).toHaveClass(
      "inline-flex",
      "h-9",
      "rounded-md",
      "border",
      "border-input",
      "bg-bg",
      "has-[input:focus-visible]:outline-2",
    );
  });
});

describe("the stepper's buttons", () => {
  it("step by the step, from the value", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(more());
    expect(onChange).toHaveBeenLastCalledWith(50);
    expect(spin()).toHaveValue("50 phút");
    await user.click(less());
    await user.click(less());
    expect(onChange).toHaveBeenLastCalledWith(40);
    expect(onChange).toHaveBeenCalledTimes(3);
  });

  it("step by one when no step is given", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <NumberStepper label={NAME} value={2} min={1} max={99} onChange={onChange} />,
    );
    await user.click(more());
    expect(onChange).toHaveBeenLastCalledWith(3);
    await user.click(less());
    expect(onChange).toHaveBeenLastCalledWith(1);
  });

  it("stop at the bounds, where the one that cannot move is switched off", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const top = render(<Minutes start={598} onChange={onChange} />);
    await user.click(more());
    expect(onChange).toHaveBeenLastCalledWith(600);
    expect(more()).toBeDisabled();
    expect(more()).toHaveClass("disabled:opacity-45");
    expect(less()).toBeEnabled();
    await user.click(more());
    expect(onChange).toHaveBeenCalledTimes(1);
    top.unmount();

    onChange.mockClear();
    render(<Minutes start={3} onChange={onChange} />);
    await user.click(less());
    expect(onChange).toHaveBeenLastCalledWith(1);
    expect(less()).toBeDisabled();
    expect(more()).toBeEnabled();
    await user.click(less());
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("are the deck's 34px squares with a 14px icon", () => {
    render(<Minutes />);
    expect(less()).toHaveClass("size-8.5", "hover:bg-muted", "rounded-l-seg");
    expect(more()).toHaveClass("size-8.5", "hover:bg-muted", "rounded-r-seg");
    expect(less().querySelector("svg")).toHaveClass("size-3.5");
    expect(more().querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    expect(less()).toHaveAttribute("type", "button");
  });
});

describe("the stepper's keys", () => {
  it("step with the arrows and go to the ends with Home and End", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("{ArrowUp}");
    expect(onChange).toHaveBeenLastCalledWith(50);
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(onChange).toHaveBeenLastCalledWith(40);
    await user.keyboard("{Home}");
    expect(onChange).toHaveBeenLastCalledWith(1);
    await user.keyboard("{End}");
    expect(onChange).toHaveBeenLastCalledWith(600);
    expect(onChange).toHaveBeenCalledTimes(5);
    expect(spin()).toHaveAttribute("aria-valuenow", "600");
  });

  it("never call past a bound", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const top = render(<Minutes start={600} onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("{ArrowUp}{End}");
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}");
    expect(onChange).toHaveBeenLastCalledWith(595);
    top.unmount();

    onChange.mockClear();
    render(<Minutes start={1} onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("{ArrowDown}{Home}");
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{ArrowUp}");
    expect(onChange).toHaveBeenLastCalledWith(6);
  });
});

describe("a switched-off stepper", () => {
  it("cannot be moved by any means", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes disabled onChange={onChange} />);
    expect(spin()).toBeDisabled();
    expect(less()).toBeDisabled();
    expect(more()).toBeDisabled();
    await user.click(more());
    await user.click(less());
    await user.click(spin());
    await user.keyboard("{ArrowUp}{End}47{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    expect(spin()).toHaveValue("45 phút");
  });
});

describe("typing the stepper's value", () => {
  it("shows the bare number, selected, while it has focus", async () => {
    const user = userEvent.setup();
    render(<Minutes />);
    expect(spin()).toHaveValue("45 phút");
    await user.click(spin());
    expect(spin()).toHaveValue("45");
    expect(spin().selectionStart).toBe(0);
    expect(spin().selectionEnd).toBe(2);
    await user.tab();
    expect(spin()).toHaveValue("45 phút");
  });

  it("calls nothing while the teacher types, then commits on Enter without snapping to the step", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("47");
    expect(spin()).toHaveValue("47");
    expect(onChange).not.toHaveBeenCalled();
    expect(spin()).toHaveAttribute("aria-valuenow", "45");

    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(47);
    expect(spin()).toHaveFocus();
    expect(spin()).toHaveValue("47");
    expect(spin()).toHaveAttribute("aria-valuetext", "47 phút");
  });

  it("commits on leaving the field in the same way", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("47");
    await user.tab();
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(47);
    expect(spin()).toHaveValue("47 phút");
  });

  it("clamps what was typed to the bounds", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("700{Enter}");
    expect(onChange).toHaveBeenLastCalledWith(600);
    expect(spin()).toHaveValue("600");

    await user.clear(spin());
    await user.keyboard("0{Enter}");
    expect(onChange).toHaveBeenLastCalledWith(1);
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it("keeps digits only", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("4a-7.b phút");
    expect(spin()).toHaveValue("47");
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith(47);
  });

  it("restores the value from an empty field", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.clear(spin());
    expect(spin()).toHaveValue("");
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    expect(spin()).toHaveValue("45");

    await user.clear(spin());
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
    expect(spin()).toHaveValue("45 phút");
  });

  it("restores the value on Escape and keeps the focus", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("99{Escape}");
    expect(spin()).toHaveValue("45");
    expect(spin()).toHaveFocus();
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
    expect(spin()).toHaveValue("45 phút");
  });

  it("calls nothing for a draft that equals the value", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("045{Enter}");
    expect(spin()).toHaveValue("45");
    await user.keyboard("{Control>}a{/Control}45");
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("steps from the committed value and drops the draft, for an arrow and for a button", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Minutes onChange={onChange} />);
    await user.click(spin());
    await user.keyboard("47{ArrowUp}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(50);
    expect(spin()).toHaveValue("50");

    await user.keyboard("{Control>}a{/Control}12");
    expect(spin()).toHaveValue("12");
    await user.click(less());
    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange).toHaveBeenLastCalledWith(45);
    expect(spin()).toHaveFocus();
    expect(spin()).toHaveValue("45");
  });

  it("does not submit a form on Enter", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
      event.preventDefault(),
    );
    render(
      <form onSubmit={onSubmit}>
        <Minutes />
        <button type="submit">Giao bài</button>
      </form>,
    );
    await user.click(spin());
    await user.keyboard("47{Enter}");
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe("the stepper's arithmetic", () => {
  it("clamps to both bounds", () => {
    expect(clampTo(5, 1, 600)).toBe(5);
    expect(clampTo(0, 1, 600)).toBe(1);
    expect(clampTo(601, 1, 600)).toBe(600);
    expect(clampTo(1, 1, 600)).toBe(1);
    expect(clampTo(600, 1, 600)).toBe(600);
  });

  it("keeps the digits of what was typed", () => {
    expect(digitsOf("4a 7,5 phút")).toBe("475");
    expect(digitsOf("-12")).toBe("12");
    expect(digitsOf("")).toBe("");
  });

  it("reads a whole number, clamped and never snapped, or nothing", () => {
    expect(typedValue("47", 1, 600)).toBe(47);
    expect(typedValue("007", 1, 600)).toBe(7);
    expect(typedValue("700", 1, 600)).toBe(600);
    expect(typedValue("0", 1, 600)).toBe(1);
    expect(typedValue("0", 0, 30)).toBe(0);
    expect(typedValue("99999999999999999999999", 1, 600)).toBe(600);
    expect(typedValue("", 1, 600)).toBeNull();
    expect(typedValue("4.5", 1, 600)).toBeNull();
    expect(typedValue("abc", 1, 600)).toBeNull();
  });

  it("names where each key goes, and nothing for any other key", () => {
    expect(keyTarget("ArrowUp", 45, 5, 1, 600)).toBe(50);
    expect(keyTarget("ArrowDown", 45, 5, 1, 600)).toBe(40);
    expect(keyTarget("ArrowUp", 598, 5, 1, 600)).toBe(603);
    expect(keyTarget("Home", 45, 5, 1, 600)).toBe(1);
    expect(keyTarget("End", 45, 5, 1, 600)).toBe(600);
    expect(keyTarget("a", 45, 5, 1, 600)).toBeNull();
    expect(keyTarget("Enter", 45, 5, 1, 600)).toBeNull();
  });
});
