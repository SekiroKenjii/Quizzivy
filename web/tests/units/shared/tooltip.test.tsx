import { it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { Tooltip } from "@/components/shared/Tooltip";
import { DeckScale } from "@/components/ui/deck-scale";
import { DialogShell, DialogShellHeader } from "@/components/shared/form/DialogShell";
const LEGACY =
  '<div data-side="top" data-align="center" data-state="instant-open" role="tooltip" id="radix-id" class="bg-foreground text-background z-50 rounded-md px-2 py-1 text-xs shadow-sm" style="--radix-tooltip-content-transform-origin: var(--radix-popper-transform-origin); --radix-tooltip-content-available-width: var(--radix-popper-available-width); --radix-tooltip-content-available-height: var(--radix-popper-available-height); --radix-tooltip-trigger-width: var(--radix-popper-anchor-width); --radix-tooltip-trigger-height: var(--radix-popper-anchor-height);">Gợi ý</div>';
it("preserves open legacy tooltip outerHTML with only its generated id normalized", async () => {
  render(
    <Tooltip label="Gợi ý">
      <button>Nút</button>
    </Tooltip>,
  );
  act(() => screen.getByRole("button").focus());
  const content = await screen.findByRole("tooltip");
  expect(content.outerHTML.replace(/id="[^"]+"/, 'id="radix-id"')).toBe(LEGACY);
});
it("raises only deck tooltip content above an open teacher dialog", async () => {
  render(
    <DeckScale>
      <DialogShell open onOpenChange={() => {}}>
        <DialogShellHeader title="Tên" />
        <Tooltip label="Gợi ý">
          <button>Nút</button>
        </Tooltip>
      </DialogShell>
    </DeckScale>,
  );
  act(() => screen.getByRole("button", { name: "Nút" }).focus());
  const content = await screen.findByRole("tooltip");
  expect(content).toHaveAttribute("data-scale", "deck");
  expect(content).toHaveClass("data-[scale=deck]:z-(--z-popover)");
  expect(content).not.toHaveClass("z-(--z-popover)");
});
