import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import "@/lib/i18n";

describe("the confirm dialog", () => {
  it("asks, then acts only on the confirming button", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <ConfirmDialog
        open
        onOpenChange={onOpenChange}
        title="Xoá câu hỏi này?"
        description="Câu hỏi rời khỏi ngân hàng."
        confirmLabel="Xoá"
        destructive
        onConfirm={onConfirm}
      />,
    );
    expect(screen.getByRole("dialog")).toHaveTextContent("Câu hỏi rời khỏi ngân hàng.");

    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);

    await user.click(screen.getByRole("button", { name: "Xoá" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("keeps the confirming button off while disabled or pending", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="Đóng ngay?"
        confirmLabel="Đóng ngay"
        disabled
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Đóng ngay" })).toBeDisabled();
  });
});
