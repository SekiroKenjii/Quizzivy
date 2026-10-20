import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { trimmed, useDebouncedInput } from "@/hooks/useDebouncedInput";

afterEach(() => {
  vi.useRealTimers();
});

function render(applied: string) {
  vi.useFakeTimers();
  const apply = vi.fn<(value: string) => void>();
  const view = renderHook(
    ({ value }: { value: string }) =>
      useDebouncedInput(value, (next) => apply(next), trimmed),
    { initialProps: { value: applied } },
  );
  return { apply, view };
}

describe("useDebouncedInput", () => {
  it("applies the text once it has stopped changing, as normalize makes it", () => {
    const { apply, view } = render("");

    act(() => view.result.current[1](" ngu"));
    act(() => vi.advanceTimersByTime(299));
    expect(apply).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(apply).toHaveBeenCalledExactlyOnceWith("ngu");
    expect(view.result.current[0]).toBe(" ngu");
  });

  it("takes an outside change of the applied value without writing the old text back, though apply is new on every render", () => {
    const { apply, view } = render("nguyen");
    act(() => vi.advanceTimersByTime(300));

    view.rerender({ value: "tran" });
    expect(view.result.current[0]).toBe("tran");
    act(() => vi.advanceTimersByTime(300));
    view.rerender({ value: "tran" });

    expect(apply).not.toHaveBeenCalled();
    expect(view.result.current[0]).toBe("tran");
  });

  it("empties the field when the applied value is cleared from outside before the text settles", () => {
    const { apply, view } = render("");

    act(() => view.result.current[1]("ngu"));
    view.rerender({ value: "" });
    view.rerender({ value: "tran" });
    view.rerender({ value: "" });
    act(() => vi.advanceTimersByTime(300));

    expect(view.result.current[0]).toBe("");
    expect(apply).not.toHaveBeenCalled();
  });

  it("lets an outside change that lands as the text settles win over the text", () => {
    const { apply, view } = render("");

    act(() => view.result.current[1]("ngu"));
    act(() => vi.advanceTimersByTime(299));
    act(() => {
      view.rerender({ value: "tran" });
      vi.advanceTimersByTime(1);
    });
    act(() => vi.advanceTimersByTime(300));

    expect(apply).not.toHaveBeenCalled();
    expect(view.result.current[0]).toBe("tran");
  });

  it("applies text typed again after an outside change, even the same text", () => {
    const { apply, view } = render("");

    act(() => view.result.current[1]("ngu"));
    act(() => {
      view.rerender({ value: "tran" });
      vi.advanceTimersByTime(300);
    });
    act(() => view.result.current[1]("ngu"));
    act(() => vi.advanceTimersByTime(300));

    expect(apply).toHaveBeenCalledExactlyOnceWith("ngu");
  });
});
