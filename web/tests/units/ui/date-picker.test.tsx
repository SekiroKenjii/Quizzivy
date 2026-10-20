import { useState } from "react";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { render, screen, within, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { format } from "date-fns";
import { vi as viLocale } from "date-fns/locale";
import { DateTimeField, type DateTimeMode } from "@/components/shared/DateTimeField";
import { FormDialog } from "@/components/shared/form/FormDialog";
import i18n from "@/lib/i18n";
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 8, 24, 10, 7));
});
afterEach(async () => {
  vi.useRealTimers();
  await i18n.changeLanguage("vi");
});
function Picker({
  mode = "datetime",
  initial = "2026-09-24T08:07",
  step = 5,
}: Readonly<{ mode?: DateTimeMode; initial?: string; step?: number }>) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <DateTimeField
        id="date"
        label="Đóng lúc"
        mode={mode}
        value={value}
        onChange={setValue}
        minuteStep={step}
      />
      <output aria-label="Giá trị">{value}</output>
      <button type="button">Ngoài</button>
    </>
  );
}
const trigger = () => screen.getByRole("button", { name: /Đóng lúc/ });
const day = (name: string) =>
  within(screen.getByRole("dialog")).getByRole("button", {
    name: new RegExp(`ngày ${name} tháng 09`),
  });
it("uses one named trigger and ArrowDown opens a Monday-first six-week picker", async () => {
  render(<Picker />);
  expect(screen.getAllByRole("button", { name: /Đóng lúc/ })).toHaveLength(1);
  expect(trigger()).toHaveAccessibleName(/08:07/);
  expect(trigger()).not.toHaveAttribute("aria-invalid");
  expect(trigger()).not.toHaveAttribute("aria-describedby");
  fireEvent.keyDown(trigger(), { key: "ArrowDown" });
  const dialog = await screen.findByRole("dialog", { name: "Chọn ngày" });
  expect(within(dialog).getAllByRole("row")).toHaveLength(6);
  expect(
    within(dialog).getAllByRole("columnheader", { hidden: true })[0],
  ).toHaveTextContent(format(new Date(2026, 8, 21), "cccccc", { locale: viLocale }));
  expect(dialog).toHaveClass(
    "z-(--z-popover)",
    "duration-120",
    "ease-[cubic-bezier(0,0,0.58,1)]",
    "data-[state=open]:slide-in-from-top-[3px]",
    "motion-reduce:animate-none!",
    "data-[state=closed]:animate-none",
  );
  expect(dialog.className).not.toMatch(/zoom-|slide-in-from-bottom/);
  expect(dialog).not.toHaveClass("z-50");
});
it("date-only selects immediately and has no Today or Done footer", async () => {
  const user = userEvent.setup();
  render(<Picker mode="date" initial="2026-09-24" />);
  await user.click(trigger());
  expect(screen.queryByRole("button", { name: "Xong" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Hôm nay" })).toBeNull();
  await user.click(day("25"));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
    "2026-09-25",
  );
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(trigger()).toHaveFocus();
});
it("datetime day and time remain drafts until Done; Escape discards and reopening uses controlled value", async () => {
  const user = userEvent.setup();
  render(<Picker />);
  await user.click(trigger());
  await user.click(day("25"));
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
    "2026-09-24T08:07",
  );
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(trigger());
  expect(screen.getByRole("dialog")).toHaveTextContent("07");
  await user.click(day("26"));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
    "2026-09-26T08:07",
  );
});
it("outside click discards a draft, while a fresh parent render preserves it", async () => {
  const user = userEvent.setup();
  const view = render(<Picker />);
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  view.rerender(<Picker />);
  expect(screen.getByRole("dialog")).toHaveTextContent("10");
  await user.click(screen.getByRole("button", { name: "Ngoài" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(trigger());
  expect(screen.getByRole("dialog")).toHaveTextContent("07");
});
it.each([
  ["08:55", "Tăng phút", "08:00"],
  ["00:10", "Giảm giờ", "23:10"],
  ["08:07", "Tăng phút", "08:10"],
])("bumps %s via %s to %s", async (initial, name, expected) => {
  const user = userEvent.setup();
  render(<Picker mode="time" initial={initial} />);
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(initial);
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(expected);
});
it("Today changes only the draft day and Now snaps the current minute", async () => {
  const user = userEvent.setup();
  const view = render(<Picker initial="2026-09-10T08:07" />);
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name: "Hôm nay" }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
    "2026-09-24T08:07",
  );
  view.unmount();
  render(<Picker mode="time" initial="08:07" />);
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name: "Bây giờ" }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent("10:05");
});
it("an empty picker opens on now's full hour, retaining a non-default step", async () => {
  const user = userEvent.setup();
  render(<Picker mode="time" initial="" step={15} />);
  expect(trigger()).toHaveTextContent("Chọn giờ");
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent("10:15");
});
it("locale changes relabel an open draft without committing or resetting it", async () => {
  const user = userEvent.setup();
  render(<Picker />);
  await user.click(trigger());
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  await act(() => i18n.changeLanguage("en"));
  expect(screen.getByRole("dialog", { name: "Choose a date" })).toHaveTextContent("10");
  await user.click(screen.getByRole("button", { name: "Done" }));
  expect(screen.getByRole("status", { name: "Giá trị" })).toHaveTextContent(
    "2026-09-24T08:10",
  );
});
function NestedPicker() {
  const [open, setOpen] = useState(true);
  return (
    <FormDialog
      open={open}
      onOpenChange={setOpen}
      title="Cửa sổ"
      initial={{ date: "2026-09-24" }}
      fields={[{ kind: "date", name: "date", label: "Ngày" }]}
      submitLabel="Lưu"
      onSubmit={() => {}}
    />
  );
}
it("Escape dismisses only the calendar above its surrounding FormDialog", async () => {
  const user = userEvent.setup();
  render(<NestedPicker />);
  await user.click(screen.getByRole("button", { name: /Ngày/ }));
  expect(screen.getAllByRole("dialog")).toHaveLength(2);
  await user.keyboard("{Escape}");
  expect(screen.getByRole("dialog", { name: "Cửa sổ" })).toBeVisible();
  expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
});

beforeEach(() =>
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  ),
);
afterEach(() => vi.unstubAllGlobals());

it.each(["date", "time"] as const)(
  "keeps the %s trigger's ARIA props absent by default",
  (mode) => {
    render(
      <DateTimeField
        label="Mốc"
        value={mode === "date" ? "2026-09-24" : "08:00"}
        mode={mode}
        onChange={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: /Mốc/ });
    expect(button).not.toHaveAttribute("aria-invalid");
    expect(button).not.toHaveAttribute("aria-describedby");
    expect(button).not.toBeDisabled();
  },
);

it.each(["date", "time"] as const)(
  "dismisses an already-open %s picker when disabled and edits again after re-enable",
  async (mode) => {
    const user = userEvent.setup();
    const change = vi.fn();
    const value = mode === "date" ? "2026-09-24" : "08:07";
    const props = { id: "lock", label: "Mốc", value, mode, onChange: change };
    const view = render(<DateTimeField {...props} />);
    await user.click(screen.getByRole("button", { name: /Mốc/ }));
    expect(document.querySelector('[data-slot="popover-content"]')).toBeInTheDocument();
    view.rerender(<DateTimeField {...props} disabled />);
    expect(
      document.querySelector('[data-slot="popover-content"]'),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Mốc/ })).toBeDisabled();
    expect(change).not.toHaveBeenCalled();
    view.rerender(<DateTimeField {...props} />);
    await user.click(screen.getByRole("button", { name: /Mốc/ }));
    if (mode === "date") await user.click(day("25"));
    else {
      await user.click(screen.getByRole("button", { name: "Tăng phút" }));
      await user.click(screen.getByRole("button", { name: "Xong" }));
    }
    expect(change).toHaveBeenCalledExactlyOnceWith(
      mode === "date" ? "2026-09-25" : "08:10",
    );
  },
);
it("discards a standalone datetime draft on lock and accepts a fresh draft after re-enable", async () => {
  const user = userEvent.setup();
  const change = vi.fn();
  const props = { label: "Mốc", value: "2026-09-24T08:07", onChange: change };
  const view = render(<DateTimeField {...props} />);
  await user.click(screen.getByRole("button", { name: /Mốc/ }));
  await user.click(screen.getByRole("button", { name: /ngày 25 tháng 09/ }));
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  view.rerender(<DateTimeField {...props} disabled />);
  expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
  expect(change).not.toHaveBeenCalled();
  view.rerender(<DateTimeField {...props} />);
  await user.click(screen.getByRole("button", { name: /Mốc/ }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(change).toHaveBeenCalledExactlyOnceWith("2026-09-24T08:07");
  change.mockClear();
  await user.click(screen.getByRole("button", { name: /Mốc/ }));
  await user.click(screen.getByRole("button", { name: /ngày 25 tháng 09/ }));
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  await user.click(screen.getByRole("button", { name: "Xong" }));
  expect(change).toHaveBeenCalledExactlyOnceWith("2026-09-25T08:10");
});
