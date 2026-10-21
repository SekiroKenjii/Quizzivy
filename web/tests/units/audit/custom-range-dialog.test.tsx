import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CustomRangeDialog } from "@/features/audit/components/CustomRangeDialog";
import "@/lib/i18n";

function renderDialog(initial: { from: string; to: string }) {
  const onApply = vi.fn();
  const onOpenChange = vi.fn();
  render(
    <CustomRangeDialog
      open
      initial={initial}
      onOpenChange={onOpenChange}
      onApply={onApply}
    />,
  );
  return { onApply, onOpenChange, dialog: screen.getByRole("dialog") };
}

describe("the custom range dialog", () => {
  it("asks for two days and opens on the ones it is given", () => {
    const { dialog } = renderDialog({ from: "2026-09-18", to: "2026-09-24" });

    expect(dialog).toHaveAccessibleName("Khoảng ngày tuỳ chọn");
    expect(within(dialog).getByRole("button", { name: /Từ ngày/ })).toBeVisible();
    expect(within(dialog).getByRole("button", { name: /Đến ngày/ })).toBeVisible();
  });

  it("applies the two days and closes", async () => {
    const { dialog, onApply, onOpenChange } = renderDialog({
      from: "2026-09-18",
      to: "2026-09-24",
    });

    await userEvent
      .setup()
      .click(within(dialog).getByRole("button", { name: "Áp dụng" }));

    expect(onApply).toHaveBeenCalledExactlyOnceWith({
      from: "2026-09-18",
      to: "2026-09-24",
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("applies a single day, the same one twice", async () => {
    const { dialog, onApply } = renderDialog({ from: "2026-09-24", to: "2026-09-24" });

    await userEvent
      .setup()
      .click(within(dialog).getByRole("button", { name: "Áp dụng" }));

    expect(onApply).toHaveBeenCalledOnce();
  });

  it("refuses an end before the start, names the end, and applies nothing", async () => {
    const { dialog, onApply, onOpenChange } = renderDialog({
      from: "2026-09-24",
      to: "2026-09-18",
    });

    await userEvent
      .setup()
      .click(within(dialog).getByRole("button", { name: "Áp dụng" }));

    expect(
      await within(dialog).findByText("Phải trùng hoặc sau ngày bắt đầu."),
    ).toBeVisible();
    expect(onApply).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("refuses a day left empty", async () => {
    const { dialog, onApply } = renderDialog({ from: "2026-09-18", to: "" });

    await userEvent
      .setup()
      .click(within(dialog).getByRole("button", { name: "Áp dụng" }));

    expect(onApply).not.toHaveBeenCalled();
  });
});
