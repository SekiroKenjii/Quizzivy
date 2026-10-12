import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { compile } from "tailwindcss";
import { SearchInput } from "@/components/shared/SearchInput";

function matchesSpacing(input: HTMLElement, selector: string) {
  return (
    (selector.startsWith(".") || selector.startsWith(":where(")) &&
    !selector.includes("&") &&
    input.matches(selector)
  );
}

async function spacing(input: HTMLElement, properties: readonly string[]) {
  const compiler = await compile("@theme { --spacing: 4px; } @tailwind utilities;");
  const css = compiler.build([...input.classList]).replace(/\/\*[\s\S]*?\*\//g, "");
  const values = new Map<string, string>();
  for (const block of css.split("}")) {
    const opening = block.lastIndexOf("{");
    const selector = block.slice(0, opening).trim();
    if (!matchesSpacing(input, selector)) continue;
    for (const declaration of block.slice(opening + 1).split(";")) {
      const [property, value] = declaration.split(":").map((part) => part.trim());
      if (property && value && properties.includes(property))
        values.set(property.startsWith("padding") ? "padding" : property, value);
    }
  }
  return values;
}

afterEach(cleanup);

describe("SearchInput compiled spacing", () => {
  for (const deck of [false, true]) {
    for (const dense of [false, true]) {
      it(`preserves ${deck ? "deck" : "off-deck"} ${dense ? "dense" : "default"} clearance and height`, async () => {
        const changes: string[] = [];
        render(
          <div data-scale={deck ? "deck" : undefined}>
            <SearchInput
              dense={dense}
              value=""
              placeholder="Tìm học viên"
              onChange={(value) => changes.push(value)}
            />
          </div>,
        );
        const input = screen.getByRole("searchbox", { name: "Tìm học viên" });
        const css = await spacing(input, ["padding-left", "padding-inline", "height"]);
        expect(css.get("padding")).toBe(`calc(var(--spacing) * ${dense ? 8 : 9})`);
        const defaultHeight = deck ? 9.5 : 9;
        expect(css.get("height")).toBe(
          `calc(var(--spacing) * ${dense ? 8 : defaultHeight})`,
        );
        fireEvent.change(input, {
          target: { value: "Nguyễn Thị Hoàng Anh tìm học viên rất dài" },
        });
        expect(changes).toEqual(["Nguyễn Thị Hoàng Anh tìm học viên rất dài"]);
        expect(input).toHaveAttribute("type", "search");
        expect(input.parentElement!.querySelector("svg")).toHaveAttribute(
          "aria-hidden",
          "true",
        );
      });
    }
  }
});
