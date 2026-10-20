import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { compile } from "tailwindcss";
import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
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
it("emits reduced animation none with importance that beats the open-state entrance", async () => {
  render(<OpenPopover />);
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
it("draws neither the replaced zoom entrance nor an exit animation", async () => {
  render(<OpenPopover />);
  const content = screen.getByRole("dialog", { name: "Lịch" });
  expect(content).toHaveClass("z-(--z-popover)");
  for (const replaced of [
    "data-[state=closed]:animate-out",
    "data-[state=open]:zoom-in-95",
    "z-50",
  ]) {
    expect(content).not.toHaveClass(replaced);
  }
  const compiler = await compile(`${animations}\n@tailwind utilities;`);
  const css = compiler.build(Array.from(content.classList));
  expect(css).not.toMatch(/animation: exit [^;]+;/);
});
