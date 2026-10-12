import { useState } from "react";
import { Direction } from "radix-ui";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RadioCards, type RadioCardOption } from "@/components/shared/form/RadioCard";

const options: readonly RadioCardOption[] = [
  { value: "a", label: "Đã chọn" },
  { value: "b", label: "Tắt", disabled: true },
  { value: "c", label: "Có thể chọn" },
  { value: "d", label: "Cuối" },
];

function Choices({
  onChange,
  controlled = false,
  choices = options,
}: Readonly<{
  onChange: (value: string) => void;
  controlled?: boolean;
  choices?: readonly RadioCardOption[];
}>) {
  const [value, setValue] = useState("a");
  return (
    <>
      <button type="button">Trước</button>
      <RadioCards
        label="Chọn"
        value={controlled ? "a" : value}
        options={choices}
        onChange={(next) => {
          onChange(next);
          if (!controlled) setValue(next);
        }}
      />
      <button type="button">Sau</button>
    </>
  );
}

const radio = (name: string) => screen.getByRole("radio", { name });
const settle = () => act(() => vi.runOnlyPendingTimers());
const microtasks = () =>
  act(async () => {
    await Promise.resolve();
  });
const press = async (key: string, modifiers: Record<string, boolean> = {}) => {
  const target = document.activeElement!;
  fireEvent.keyDown(target, { key, ...modifiers });
  fireEvent.keyUp(document.activeElement!, { key, ...modifiers });
  await microtasks();
  settle();
};

beforeEach(() => vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] }));
afterEach(() => vi.useRealTimers());

describe("radio card keyboard selection", async () => {
  it.each([
    ["ArrowRight", "Có thể chọn", "c"],
    ["ArrowDown", "Có thể chọn", "c"],
    ["ArrowLeft", "Cuối", "d"],
    ["ArrowUp", "Cuối", "d"],
  ])("selects with %s when keyup precedes deferred focus", async (key, name, value) => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    await press(key!);
    expect(radio(name!)).toHaveFocus();
    expect(radio(name!)).toBeChecked();
    expect(radio("Tắt")).not.toBeChecked();
    expect(onChange).toHaveBeenCalledExactlyOnceWith(value);
  });

  it("handles rapid held repeats, skips disabled cards and loops with one callback per selection", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    for (let repeat = 0; repeat < 3; repeat++) {
      fireEvent.keyDown(document.activeElement!, {
        key: "ArrowRight",
        repeat: repeat > 0,
      });
      await microtasks();
    }
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    settle();
    expect(onChange.mock.calls).toEqual([["c"], ["d"], ["a"]]);
    expect(radio("Đã chọn")).toBeChecked();
    await press("ArrowLeft");
    expect(radio("Cuối")).toHaveFocus();
    expect(radio("Cuối")).toBeChecked();
    expect(onChange.mock.calls).toEqual([["c"], ["d"], ["a"], ["d"]]);
  });

  it("preserves controlled checked state until the caller changes the value", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} controlled />);
    act(() => radio("Đã chọn").focus());
    await press("ArrowRight");
    expect(radio("Có thể chọn")).toHaveFocus();
    expect(radio("Đã chọn")).toBeChecked();
    expect(radio("Có thể chọn")).not.toBeChecked();
    expect(onChange).toHaveBeenCalledExactlyOnceWith("c");
    expect(screen.getAllByRole("radio").filter((item) => item.tabIndex === 0)).toEqual([
      radio("Có thể chọn"),
    ]);
    await press("ArrowRight");
    expect(onChange.mock.calls).toEqual([["c"], ["d"]]);
  });

  it.each(["altKey", "ctrlKey", "metaKey", "shiftKey"])(
    "does not move or choose with %s held",
    async (modifier) => {
      const onChange = vi.fn();
      render(<Choices onChange={onChange} />);
      act(() => radio("Đã chọn").focus());
      await press("ArrowRight", { [modifier]: true });
      expect(radio("Đã chọn")).toHaveFocus();
      expect(radio("Đã chọn")).toBeChecked();
      expect(onChange).not.toHaveBeenCalled();
    },
  );

  it("does not choose on Tab or programmatic focus, and Space selects once", async () => {
    vi.useRealTimers();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    await user.tab();
    await user.tab();
    expect(radio("Đã chọn")).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Sau" })).toHaveFocus();
    act(() => radio("Có thể chọn").focus());
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard(" ");
    expect(radio("Có thể chọn")).toBeChecked();
    expect(onChange).toHaveBeenCalledExactlyOnceWith("c");
  });

  it("does not call back when an arrow has no other enabled choice", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} choices={options.slice(0, 2)} />);
    act(() => radio("Đã chọn").focus());
    await press("ArrowRight");
    expect(radio("Đã chọn")).toHaveFocus();
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("reverses horizontal arrows under the same direction provider as Radix", async () => {
    const onChange = vi.fn();
    render(
      <Direction.Provider dir="rtl">
        <Choices onChange={onChange} />
      </Direction.Provider>,
    );
    act(() => radio("Đã chọn").focus());
    await press("ArrowRight");
    expect(radio("Cuối")).toHaveFocus();
    expect(radio("Cuối")).toBeChecked();
    await press("ArrowLeft");
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange.mock.calls).toEqual([["d"], ["a"]]);
  });

  it("preserves Home and End focus without choosing", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Có thể chọn").focus());
    await press("End");
    expect(radio("Cuối")).toHaveFocus();
    await press("Home");
    expect(radio("Đã chọn")).toHaveFocus();
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not choose after focus moves elsewhere before the fallback", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    act(() => screen.getByRole("button", { name: "Sau" }).focus());
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Đã chọn")).toBeChecked();
  });

  it("does not cancel pending selection for an equivalent fresh options array", async () => {
    const onChange = vi.fn();
    const view = render(<Choices onChange={onChange} choices={[...options]} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    view.rerender(<Choices onChange={onChange} choices={[...options]} />);
    await microtasks();
    settle();
    expect(radio("Có thể chọn")).toBeChecked();
    expect(onChange).toHaveBeenCalledExactlyOnceWith("c");
  });

  it("cancels pending selection when the target is disabled before the fallback", async () => {
    const onChange = vi.fn();
    const view = render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    view.rerender(
      <Choices
        onChange={onChange}
        choices={options.map((option) =>
          option.value === "c" ? { ...option, disabled: true } : option,
        )}
      />,
    );
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Đã chọn")).toBeChecked();
  });

  it("cancels pending selection on Tab before its fallback", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    fireEvent.keyDown(document.activeElement!, { key: "Tab" });
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Đã chọn")).toBeChecked();
  });

  it("cancels pending selection on a pointer gesture before its fallback", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    fireEvent.pointerDown(document.activeElement!);
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Đã chọn")).toBeChecked();
  });

  it("cancels pending selection when another option's enabled state changes", async () => {
    const onChange = vi.fn();
    const view = render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    view.rerender(
      <Choices
        onChange={onChange}
        choices={options.map((option) =>
          option.value === "d" ? { ...option, disabled: true } : option,
        )}
      />,
    );
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Đã chọn")).toBeChecked();
  });

  it("cancels pending selection when controlled value changes externally", async () => {
    const onChange = vi.fn();
    const view = render(
      <RadioCards label="Chọn" value="a" options={options} onChange={onChange} />,
    );
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    view.rerender(
      <RadioCards label="Chọn" value="d" options={options} onChange={onChange} />,
    );
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
    expect(radio("Cuối")).toBeChecked();
  });

  it("does not notify after unmount before the fallback", async () => {
    const onChange = vi.fn();
    const view = render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    view.unmount();
    await microtasks();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not choose through unrelated programmatic focus while its arrow is held", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} />);
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    act(() => radio("Cuối").focus());
    fireEvent.keyUp(radio("Cuối"), { key: "ArrowRight" });
    await microtasks();
    expect(radio("Cuối")).toHaveFocus();
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("calls once if Radix already selects on focus while the caller refuses the value", async () => {
    const onChange = vi.fn();
    render(<Choices onChange={onChange} controlled />);
    fireEvent.keyDown(screen.getByRole("button", { name: "Trước" }), {
      key: "ArrowRight",
    });
    act(() => radio("Đã chọn").focus());
    fireEvent.keyDown(radio("Đã chọn"), { key: "ArrowRight" });
    fireEvent.keyUp(document.activeElement!, { key: "ArrowRight" });
    await microtasks();
    expect(radio("Có thể chọn")).toHaveFocus();
    expect(radio("Đã chọn")).toBeChecked();
    expect(onChange).toHaveBeenCalledExactlyOnceWith("c");
  });
});
