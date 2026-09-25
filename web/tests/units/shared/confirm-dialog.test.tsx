import { useEffect, useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

  it("returns focus to the button that opened it", async () => {
    const user = userEvent.setup();
    render(<Opener />);
    await user.click(screen.getByRole("button", { name: "Xoá phần" }));
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Xoá phần" })).toHaveFocus(),
    );
  });

  it("leaves focus where the confirmed action put it", async () => {
    const user = userEvent.setup();
    render(<Opener moveFocus />);
    await user.click(screen.getByRole("button", { name: "Xoá phần" }));
    await user.click(screen.getByRole("button", { name: "Xoá" }));
    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: "Ghi chú" })).toHaveFocus(),
    );
  });

  it("prefers an explicit return target over the opener", async () => {
    const user = userEvent.setup();
    render(<Opener returnToNote />);
    await user.click(screen.getByRole("button", { name: "Xoá phần" }));
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: "Ghi chú" })).toHaveFocus(),
    );
  });
});

function Opener({
  moveFocus = false,
  returnToNote = false,
}: Readonly<{ moveFocus?: boolean; returnToNote?: boolean }>) {
  const [open, setOpen] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const note = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (confirmed && moveFocus) note.current?.focus();
  }, [confirmed, moveFocus]);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Xoá phần
      </button>
      <input ref={note} aria-label="Ghi chú" />
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title="Xoá phần này?"
        confirmLabel="Xoá"
        {...(returnToNote ? { returnFocus: note } : {})}
        onConfirm={() => {
          setConfirmed(true);
          setOpen(false);
        }}
      />
    </>
  );
}
