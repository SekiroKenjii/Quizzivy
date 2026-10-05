import { readFileSync } from "node:fs";
import { compile } from "tailwindcss";
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { ListSkeleton } from "@/components/shared/ListState";
import "@/lib/i18n";

it.each(["hover", "focus-within"])(
  "compiles a deck-only host %s pause that outranks the unlayered shorthand",
  async (state) => {
    render(
      <>
        <div data-scale="deck" data-testid="deck">
          <ListSkeleton rows={3} />
        </div>
        <div data-testid="legacy">
          <ListSkeleton rows={2} />
        </div>
      </>,
    );
    const host = screen.getAllByRole("status")[0]!;
    const candidates = Array.from(host.classList).filter(
      (value) => value.includes(`${state}:`) && value.includes("animation-play-state"),
    );
    expect(candidates).toHaveLength(1);
    const compiler = await compile("@tailwind utilities;");
    const css = compiler.build(candidates);
    expect(css).toContain(':where([data-scale="deck"])');
    expect(css).toContain(`[data-slot=skeleton]`);
    expect(css).toContain(`:${state} [data-slot=skeleton]`);
    const line = css
      .split("\n")
      .find((value) => value.trim().endsWith(`[data-slot=skeleton] {`))!;
    const selector = line
      .trim()
      .slice(0, -2)
      .replace(new RegExp(`\\.[^ ]+(?=:${state} )`), "[data-probe-host]")
      .replace(`:${state} `, "[data-probe-active] ");
    host.setAttribute("data-probe-host", "");
    const deck = screen.getByTestId("deck");
    deck.setAttribute("data-probe-active", "");
    expect(document.querySelectorAll(selector)).toHaveLength(0);
    host.setAttribute("data-probe-active", "");
    expect(Array.from(document.querySelectorAll(selector))).toEqual(
      Array.from(host.querySelectorAll('[data-slot="skeleton"]')),
    );
    const legacy = screen.getAllByRole("status")[1]!;
    legacy.setAttribute("data-probe-host", "");
    legacy.setAttribute("data-probe-active", "");
    expect(legacy.querySelector('[data-slot="skeleton"]')?.matches(selector)).toBe(
      false,
    );
    expect(css).toContain("animation-play-state: paused !important;");
    expect(css).not.toMatch(/animation:\s/);
    expect(host.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(3);
    expect(host.querySelector("[tabindex],button,input")).toBeNull();
    expect(host).not.toHaveAttribute("tabindex");
    const source = readFileSync("src/index.css", "utf8");
    expect(source).toContain("animation: qz-shimmer 1.4s linear infinite");
    expect(source).toMatch(
      /prefers-reduced-motion: reduce[\s\S]*?\[data-scale="deck"\] \[data-slot="skeleton"\] \{\s*animation: none;/,
    );
  },
);
