import { useState } from "react";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { render, screen, within, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { format } from "date-fns";
import { vi as viLocale } from "date-fns/locale";
import { DeckScale } from "@/components/ui/deck-scale";
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
    <DeckScale>
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
    </DeckScale>
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
  expect(dialog).toHaveAttribute("data-scale", "deck");
  expect(dialog).toHaveClass(
    "data-[scale=deck]:z-(--z-popover)",
    "duration-120",
    "ease-[cubic-bezier(0,0,0.58,1)]",
    "data-[state=open]:slide-in-from-top-[3px]",
    "motion-reduce:animate-none",
    "data-[state=closed]:animate-none",
  );
  expect(dialog.className).not.toMatch(/zoom-|slide-in-from-bottom/);
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
    <DeckScale>
      <FormDialog
        open={open}
        onOpenChange={setOpen}
        title="Cửa sổ"
        initial={{ date: "2026-09-24" }}
        fields={[{ kind: "date", name: "date", label: "Ngày" }]}
        submitLabel="Lưu"
        onSubmit={() => {}}
      />
    </DeckScale>
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
  "keeps the legacy %s trigger's new ARIA props absent by default",
  (mode) => {
    render(
      <DateTimeField
        label="Mốc"
        value={mode === "date" ? "2026-09-24" : "08:00"}
        mode={mode}
        onChange={() => {}}
      />,
    );
    const group = screen.getByRole("group", { name: "Mốc" });
    const button = within(group).getByRole("button");
    expect(button).not.toHaveAttribute("aria-invalid");
    expect(button).not.toHaveAttribute("aria-describedby");
  },
);

it.each(["date", "time"] as const)(
  "pins absent-disabled legacy %s defaults",
  (mode) => {
    render(
      <DateTimeField
        id="default-field"
        label="Mốc"
        value={mode === "date" ? "2026-09-24" : "08:00"}
        mode={mode}
        onChange={() => {}}
      />,
    );
    const html = screen
      .getByRole("group", { name: "Mốc" })
      .outerHTML.replace(/aria-controls="[^"]+"/g, 'aria-controls="radix-id"');
    const expected = {
      date: '<div role="group" aria-label="Mốc" class="flex"><button data-slot="popover-trigger" data-variant="outline" data-size="default" class="inline-flex items-center gap-2 rounded-md text-sm whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&amp;_svg]:pointer-events-none [&amp;_svg]:shrink-0 [&amp;_svg:not([class*=\'size-\'])]:size-4 border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50 in-data-[scale=deck]:bg-card in-data-[scale=deck]:hover:bg-muted dark:in-data-[scale=deck]:border-border dark:in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:hover:bg-muted h-9 px-4 py-2 in-data-[scale=deck]:px-3.5 in-data-[scale=deck]:text-ui in-data-[scale=deck]:gap-1.5 in-data-[scale=deck]:[&amp;_svg:not([class*=\'size-\'])]:size-[15px] flex-1 justify-between font-normal" id="default-field" type="button" aria-haspopup="dialog" aria-expanded="false" data-state="closed">24/09/2026<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-calendar" aria-hidden="true"><path d="M8 2v4"></path><path d="M16 2v4"></path><rect width="18" height="18" x="3" y="4" rx="2"></rect><path d="M3 10h18"></path></svg></button></div>',
      time: '<div role="group" aria-label="Mốc" class="flex"><button data-slot="popover-trigger" data-variant="outline" data-size="default" class="inline-flex items-center gap-2 rounded-md text-sm whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&amp;_svg]:pointer-events-none [&amp;_svg]:shrink-0 [&amp;_svg:not([class*=\'size-\'])]:size-4 border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50 in-data-[scale=deck]:bg-card in-data-[scale=deck]:hover:bg-muted dark:in-data-[scale=deck]:border-border dark:in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:hover:bg-muted h-9 px-4 py-2 in-data-[scale=deck]:px-3.5 in-data-[scale=deck]:text-ui in-data-[scale=deck]:gap-1.5 in-data-[scale=deck]:[&amp;_svg:not([class*=\'size-\'])]:size-[15px] justify-between font-normal tabular-nums flex-1" id="default-field" aria-label="Mốc — giờ" type="button" aria-haspopup="dialog" aria-expanded="false" data-state="closed">08:00<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-clock" aria-hidden="true"><path d="M12 6v6l4 2"></path><circle cx="12" cy="12" r="10"></circle></svg></button></div>',
    };
    expect(html).toBe(expected[mode]);
    expect(screen.getByRole("button")).not.toBeDisabled();
  },
);

it.each(["date", "time"] as const)(
  "dismisses an already-open legacy %s picker when disabled and edits again after re-enable",
  async (mode) => {
    const user = userEvent.setup();
    const change = vi.fn();
    const value = mode === "date" ? "2026-09-24" : "08:07";
    const props = { id: "legacy-lock", label: "Mốc", value, mode, onChange: change };
    const view = render(<DateTimeField {...props} />);
    await user.click(
      within(screen.getByRole("group", { name: "Mốc" })).getByRole("button"),
    );
    expect(document.querySelector('[data-slot="popover-content"]')).toBeInTheDocument();
    view.rerender(<DateTimeField {...props} disabled />);
    expect(
      document.querySelector('[data-slot="popover-content"]'),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByRole("group", { name: "Mốc" })).getByRole("button"),
    ).toBeDisabled();
    expect(change).not.toHaveBeenCalled();
    view.rerender(<DateTimeField {...props} />);
    await user.click(
      within(screen.getByRole("group", { name: "Mốc" })).getByRole("button"),
    );
    if (mode === "date")
      await user.click(screen.getByRole("button", { name: /ngày 25 tháng 09/ }));
    else
      await user.click(
        within(screen.getByRole("listbox", { name: "Phút" })).getByRole("option", {
          name: "10",
        }),
      );
    expect(change).toHaveBeenCalledExactlyOnceWith(
      mode === "date" ? "2026-09-25" : "08:10",
    );
  },
);
it("discards a standalone datetime draft on lock and accepts a fresh draft after re-enable", async () => {
  const user = userEvent.setup();
  const change = vi.fn();
  const props = { label: "Mốc", value: "2026-09-24T08:07", onChange: change };
  const view = render(
    <DeckScale>
      <DateTimeField {...props} />
    </DeckScale>,
  );
  await user.click(screen.getByRole("button", { name: /Mốc/ }));
  await user.click(screen.getByRole("button", { name: /ngày 25 tháng 09/ }));
  await user.click(screen.getByRole("button", { name: "Tăng phút" }));
  view.rerender(
    <DeckScale>
      <DateTimeField {...props} disabled />
    </DeckScale>,
  );
  expect(screen.queryByRole("dialog", { name: "Chọn ngày" })).toBeNull();
  expect(change).not.toHaveBeenCalled();
  view.rerender(
    <DeckScale>
      <DateTimeField {...props} />
    </DeckScale>,
  );
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
