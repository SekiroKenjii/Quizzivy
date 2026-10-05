import { compile } from "tailwindcss";
import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeckScale } from "@/components/ui/deck-scale";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  DialogShell,
  DialogShellHeader,
  DialogShellBody,
  DialogShellFooter,
} from "@/components/shared/form/DialogShell";
import "@/lib/i18n";

it.each([false, true])(
  "removes inherited max-height resize transition from teacher frame with deck=%s",
  async (deck) => {
    const shell = (
      <DialogShell open onOpenChange={() => {}}>
        <DialogShellHeader title="Khung" />
        <DialogShellBody>
          <input aria-label="Tên" />
        </DialogShellBody>
        <DialogShellFooter>
          <button type="button">Lưu</button>
        </DialogShellFooter>
      </DialogShell>
    );
    render(deck ? <DeckScale>{shell}</DeckScale> : shell);
    const content = screen.getByRole("dialog", { name: "Khung" });
    const compiler = await compile("@tailwind utilities;");
    const css = compiler.build(Array.from(content.classList));
    expect(css).toContain("transition-property: none;");
    expect(css).toMatch(/transition-duration: 0(?:ms|s);/);
    expect(css).not.toMatch(/transition-duration: (?:200ms|0\.2s);/);
    expect(content).toHaveClass(
      "max-h-[86dvh]",
      "data-[state=open]:animate-none",
      "data-[state=closed]:animate-none",
    );
    expect(screen.getByRole("textbox", { name: "Tên" }).parentElement).toHaveClass(
      "overflow-y-auto",
      "min-h-0",
    );
    expect(screen.getByRole("button", { name: "Lưu" }).parentElement).toHaveClass(
      "flex-none",
    );
  },
);
it("leaves the vendored default dialog duration and animations unchanged", async () => {
  render(
    <Dialog open>
      <DialogContent>
        <DialogTitle>Thông thường</DialogTitle>
      </DialogContent>
    </Dialog>,
  );
  const content = screen.getByRole("dialog", { name: "Thông thường" });
  const compiler = await compile("@tailwind utilities;");
  const css = compiler.build(Array.from(content.classList));
  expect(css).toMatch(/transition-duration: (?:200ms|0\.2s);/);
  expect(css).not.toContain("transition-property: none;");
  expect(content).toHaveClass(
    "data-[state=open]:animate-in",
    "data-[state=closed]:animate-out",
  );
});
