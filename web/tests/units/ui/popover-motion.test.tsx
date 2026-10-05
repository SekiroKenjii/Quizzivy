import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { compile } from "tailwindcss";
import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeckScale } from "@/components/ui/deck-scale";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

const animations = readFileSync(
  resolve(
    import.meta.dirname,
    "../../../node_modules/tw-animate-css/dist/tw-animate.css",
  ),
  "utf8",
);
function OpenPopover() {
  return (
    <Popover defaultOpen>
      <PopoverTrigger asChild>
        <button type="button">Mở</button>
      </PopoverTrigger>
      <PopoverContent aria-label="Lịch">Nội dung</PopoverContent>
    </Popover>
  );
}
it("emits reduced animation none with importance that beats the deck open-state entrance", async () => {
  render(
    <DeckScale>
      <OpenPopover />
    </DeckScale>,
  );
  const content = screen.getByRole("dialog", { name: "Lịch" });
  expect(content).toHaveAttribute("data-state", "open");
  const compiler = await compile(`${animations}\n@tailwind utilities;`);
  const css = compiler.build(Array.from(content.classList));
  expect(css).toMatch(/animation: enter [^;]+;/);
  expect(css).toMatch(
    /@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.motion-reduce\\:animate-none\\!\s*\{\s*animation: none !important;/,
  );
  expect(content).toHaveClass(
    "duration-120",
    "ease-[cubic-bezier(0,0,0.58,1)]",
    "data-[state=open]:slide-in-from-top-[3px]",
    "data-[state=closed]:animate-none",
  );
});
it("retains the inherited off-deck entrance and exit without a deck reduced override", async () => {
  render(<OpenPopover />);
  const content = screen.getByRole("dialog", { name: "Lịch" });
  expect(content).not.toHaveAttribute("data-scale");
  expect(content).toHaveClass(
    "data-[state=open]:animate-in",
    "data-[state=closed]:animate-out",
    "data-[state=open]:zoom-in-95",
    "z-50",
  );
  const compiler = await compile(`${animations}\n@tailwind utilities;`);
  const css = compiler.build(Array.from(content.classList));
  expect(css).toMatch(/animation: enter [^;]+;/);
  expect(css).toMatch(/animation: exit [^;]+;/);
  expect(css).not.toContain("prefers-reduced-motion");
});
