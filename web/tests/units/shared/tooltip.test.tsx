import { it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { Tooltip } from "@/components/shared/Tooltip";
import { DialogShell, DialogShellHeader } from "@/components/shared/form/DialogShell";
const OPEN =
  '<div data-side="top" data-align="center" data-state="instant-open" role="tooltip" id="radix-id" class="bg-foreground text-background rounded-md px-2 py-1 text-xs shadow-sm z-(--z-popover)" style="--radix-tooltip-content-transform-origin: var(--radix-popper-transform-origin); --radix-tooltip-content-available-width: var(--radix-popper-available-width); --radix-tooltip-content-available-height: var(--radix-popper-available-height); --radix-tooltip-trigger-width: var(--radix-popper-anchor-width); --radix-tooltip-trigger-height: var(--radix-popper-anchor-height);">Gợi ý</div>';
it("renders an open tooltip's outerHTML with only its generated id normalized", async () => {
  render(
    <Tooltip label="Gợi ý">
      <button>Nút</button>
    </Tooltip>,
  );
  act(() => screen.getByRole("button").focus());
  const content = await screen.findByRole("tooltip");
  expect(content.outerHTML.replace(/id="[^"]+"/, 'id="radix-id"')).toBe(OPEN);
});
it("raises its content above an open teacher dialog", async () => {
  render(
    <DialogShell open onOpenChange={() => {}}>
      <DialogShellHeader title="Tên" />
      <Tooltip label="Gợi ý">
        <button>Nút</button>
      </Tooltip>
    </DialogShell>,
  );
  act(() => screen.getByRole("button", { name: "Nút" }).focus());
  const content = await screen.findByRole("tooltip");
  expect(content).toHaveClass("z-(--z-popover)");
  expect(content).not.toHaveClass("z-50");
});
