import { it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { DeckDialog } from "@/components/shared/DeckDialog";
import "@/lib/i18n";
const CONTENT =
  "bg-background data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border p-6 shadow-lg duration-200 outline-none sm:max-w-lg z-(--z-dialog)";
const OVERLAY =
  "data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 bg-overlay fixed inset-0 z-(--z-dialog)";
it("keeps the content and overlay classes exactly, at the dialog layer", () => {
  render(
    <Dialog open>
      <DialogContent>
        <DialogTitle>Tên</DialogTitle>
      </DialogContent>
    </Dialog>,
  );
  expect(screen.getByRole("dialog").className).toBe(CONTENT);
  const overlay = document.querySelector('[data-slot="dialog-overlay"]')!;
  expect(overlay.className).toBe(OVERLAY);
  expect(screen.getByRole("dialog")).not.toHaveClass("z-50");
  expect(overlay).not.toHaveClass("z-50");
});
it("keeps the student frame's title geometry and motion", () => {
  render(
    <DeckDialog open onOpenChange={() => {}} title="Tham gia" description="Mô tả">
      <button>Đồng ý</button>
    </DeckDialog>,
  );
  expect(screen.getByRole("dialog").className).toContain(
    "data-[state=open]:animate-in",
  );
  expect(document.querySelector('[data-slot="dialog-title"]')!.className).toBe(
    "text-lg leading-normal font-semibold",
  );
});
it("allows an overlay-only motion override without changing content defaults", () => {
  render(
    <Dialog open>
      <DialogContent overlayClassName="data-[state=open]:animate-none">
        <DialogTitle>Tên</DialogTitle>
      </DialogContent>
    </Dialog>,
  );
  expect(document.querySelector('[data-slot="dialog-overlay"]')).toHaveClass(
    "data-[state=open]:animate-none",
  );
  expect(screen.getByRole("dialog").className).toBe(CONTENT);
});
