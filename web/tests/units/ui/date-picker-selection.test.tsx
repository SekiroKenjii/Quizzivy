import { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { DateTimeField, type DateTimeMode } from "@/components/shared/DateTimeField";
import { FormDialog, type FormDialogProps } from "@/components/shared/form/FormDialog";
import i18n from "@/lib/i18n";

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 8, 24, 10, 7));
});
afterEach(async () => {
  vi.useRealTimers();
  await i18n.changeLanguage("vi");
});
function Selection({
  initial = "",
  mode = "date",
  disabled = false,
  onChange,
}: Readonly<{
  initial?: string;
  mode?: DateTimeMode;
  disabled?: boolean;
  onChange: (value: string) => void;
}>) {
  const [value, setValue] = useState(initial);
  return (
    <DeckScale>
      <DateTimeField
        id="selected-day"
        label="Mốc"
        value={value}
        mode={mode}
        disabled={disabled}
        onChange={(next) => {
          onChange(next);
          setValue(next);
        }}
      />
      <output aria-label="Giá trị">{value}</output>
      <button type="button">Ngoài</button>
    </DeckScale>
  );
}
const trigger = () => screen.getByRole("button", { name: /Mốc/ });
const today = () =>
  within(screen.getByRole("dialog", { name: "Chọn ngày" })).getByRole("button", {
    name: /ngày 24 tháng 09/,
  });

it.each(["", "2026-09-24"])(
  "commits the highlighted current day exactly once from controlled value %s",
  async (initial) => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Selection initial={initial} onChange={onChange} />);
    await user.click(trigger());
    expect(today()).toHaveAttribute("data-selected-single", "true");
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("status", { name: "Giá trị" }).textContent).toBe(initial);
    await user.click(today());
    expect(onChange).toHaveBeenCalledExactlyOnceWith("2026-09-24");
    expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
      "2026-09-24",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(trigger()).toHaveFocus();
  },
);
it("keyboard activation of the selected day commits and restores trigger focus", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  render(<Selection onChange={onChange} />);
  act(() => trigger().focus());
  await user.keyboard("{ArrowDown}");
  await screen.findByRole("dialog");
  act(() => today().focus());
  await user.keyboard("{Enter}");
  expect(onChange).toHaveBeenCalledExactlyOnceWith("2026-09-24");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(trigger()).toHaveFocus();
});
it("selected datetime day remains a draft with unchanged time until Done and can be cancelled", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  render(<Selection mode="datetime" initial="2026-09-24T08:07" onChange={onChange} />);
  await user.click(trigger());
  await user.click(today());
  expect(today()).toHaveAttribute("data-selected-single", "true");
  expect(screen.getByRole("dialog")).toHaveTextContent("07");
  expect(onChange).not.toHaveBeenCalled();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(onChange).not.toHaveBeenCalled();
  await user.click(trigger());
  await user.click(today());
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  expect(onChange).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(onChange).toHaveBeenCalledExactlyOnceWith("2026-09-24T08:10");
});
it("a same-mounted disabled picker rejects its detached selected day and permits a fresh commit after unlock", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  const view = render(<Selection onChange={onChange} />);
  await user.click(trigger());
  const detached = today();
  view.rerender(<Selection disabled onChange={onChange} />);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(trigger()).toBeDisabled();
  fireEvent.click(detached);
  expect(onChange).not.toHaveBeenCalled();
  view.rerender(<Selection onChange={onChange} />);
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(trigger());
  await user.click(today());
  expect(onChange).toHaveBeenCalledExactlyOnceWith("2026-09-24");
});
it.each(["pending", "disabled"] as const)(
  "selected today clears required error, submits current values through the external footer and preserves the %s lock",
  async (lock) => {
    const user = userEvent.setup();
    const submitted = vi.fn();
    const props: FormDialogProps<{ title: string; date: string }> = {
      open: true,
      onOpenChange: vi.fn(),
      title: "Mốc bắt buộc",
      initial: { title: "Giữ tên", date: "" },
      fields: [
        { kind: "text" as const, name: "title", label: "Tên", required: true },
        { kind: "date" as const, name: "date", label: "Mốc", required: true },
      ],
      submitLabel: "Lưu",
      onSubmit: submitted,
    };
    const view = render(
      <DeckScale>
        <FormDialog {...props} />
      </DeckScale>,
    );
    const submit = screen.getByRole("button", { name: "Lưu" }) as HTMLButtonElement;
    await user.click(submit);
    expect(submitted).not.toHaveBeenCalled();
    expect(trigger()).toHaveFocus();
    expect(trigger()).toHaveAttribute("aria-invalid", "true");
    expect(submit).toHaveAttribute("aria-disabled", "true");
    await user.click(trigger());
    expect(submitted).not.toHaveBeenCalled();
    await user.click(today());
    expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
    expect(trigger()).not.toHaveAttribute("aria-invalid", "true");
    expect(submit).not.toHaveAttribute("aria-disabled", "true");
    const title = screen.getByRole("textbox", { name: "Tên" }) as HTMLInputElement;
    expect(title.form).not.toBeNull();
    expect(submit.form).toBe(title.form);
    expect(submit.closest("form")).toBeNull();
    expect(Array.from(title.form!.elements)).toContain(submit);
    await user.clear(title);
    await user.type(title, "Tên mới{Enter}");
    expect(submitted).toHaveBeenCalledExactlyOnceWith({
      title: "Tên mới",
      date: "2026-09-24",
    });
    expect(screen.getByRole("dialog", { name: "Mốc bắt buộc" })).toBeInTheDocument();
    submitted.mockClear();
    await user.click(trigger());
    const heldDay = today();
    view.rerender(
      <DeckScale>
        <FormDialog {...props} {...{ [lock]: true }} />
      </DeckScale>,
    );
    expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
    fireEvent.click(heldDay);
    fireEvent.submit(title.form!);
    expect(submitted).not.toHaveBeenCalled();
    expect(title).toHaveValue("Tên mới");
    view.rerender(
      <DeckScale>
        <FormDialog {...props} />
      </DeckScale>,
    );
    expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
    await user.click(trigger());
    await user.click(today());
    await user.click(submit);
    expect(submitted).toHaveBeenCalledExactlyOnceWith({
      title: "Tên mới",
      date: "2026-09-24",
    });
  },
);
