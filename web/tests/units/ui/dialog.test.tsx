import { it, expect } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { DeckScale } from "@/components/ui/deck-scale";
import { DeckDialog } from "@/components/shared/DeckDialog";
import "@/lib/i18n";
const CONTENT =
  "bg-background data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border p-6 shadow-lg duration-200 outline-none sm:max-w-lg";
const OVERLAY =
  "data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 bg-overlay fixed inset-0 z-50";
const classes = (element: Element) =>
  element.className.split(" ").sort((a, b) => a.localeCompare(b));
it("keeps the legacy content and overlay classes exactly", () => {
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
  expect(overlay).not.toHaveAttribute("data-scale");
});
it("adds only its own deck z class and propagates deck scale to the overlay", () => {
  render(
    <DeckScale>
      <Dialog open>
        <DialogContent>
          <DialogTitle>Tên</DialogTitle>
        </DialogContent>
      </Dialog>
    </DeckScale>,
  );
  const content = screen.getByRole("dialog");
  const overlay = document.querySelector('[data-slot="dialog-overlay"]')!;
  expect(classes(content)).toEqual(
    (CONTENT + " data-[scale=deck]:z-(--z-dialog)")
      .split(" ")
      .sort((a, b) => a.localeCompare(b)),
  );
  expect(classes(overlay)).toEqual(
    (OVERLAY + " data-[scale=deck]:z-(--z-dialog)")
      .split(" ")
      .sort((a, b) => a.localeCompare(b)),
  );
  expect(overlay).toHaveAttribute("data-scale", "deck");
});
it("keeps Student frame and title geometry and motion with deck scale", () => {
  const child = (
    <DeckDialog open onOpenChange={() => {}} title="Tham gia" description="Mô tả">
      <button>Đồng ý</button>
    </DeckDialog>
  );
  render(child);
  const original = screen.getByRole("dialog").className;
  const title = "text-lg leading-normal font-semibold";
  expect(document.querySelector('[data-slot="dialog-title"]')!.className).toBe(title);
  cleanup();
  render(<DeckScale>{child}</DeckScale>);
  const actual = screen.getByRole("dialog").className;
  expect(
    actual
      .split(" ")
      .filter((c) => !c.includes("scale=deck"))
      .sort((a, b) => a.localeCompare(b)),
  ).toEqual(original.split(" ").sort((a, b) => a.localeCompare(b)));
  expect(actual).toContain("data-[state=open]:animate-in");
  expect(document.querySelector('[data-slot="dialog-title"]')!.className).toBe(title);
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
