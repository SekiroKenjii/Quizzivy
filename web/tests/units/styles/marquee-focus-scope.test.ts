import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect, it } from "vitest";

const css = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);

it("limits ancestor focus pause to interactive hosts without pausing the route", () => {
  const pauseSelectors = css.split("}").flatMap((block) => {
    const [selector, declarations] = block.split("{").slice(-2);
    return declarations?.includes("animation-play-state: paused")
      ? [selector!.trim()]
      : [];
  });
  expect(pauseSelectors.length).toBeGreaterThan(0);

  for (const [markup, paused, sibling] of [
    ['<main tabindex="-1"></main>', false, false],
    ['<div tabindex="0"></div>', false, false],
    ['<button type="button"></button>', true, false],
    ['<a href="/teacher/tests"></a>', true, false],
    ['<span role="button" tabindex="0"></span>', true, false],
    ['<span role="link" tabindex="0"></span>', true, false],
    ['<button type="button"></button>', false, true],
  ] satisfies [string, boolean, boolean][]) {
    const root = document.createElement("div");
    root.innerHTML = markup;
    const host = root.firstElementChild as HTMLElement;
    const marquee = document.createElement("span");
    marquee.className = "qz-marquee";
    marquee.innerHTML = '<span class="qz-marquee-track">Title</span>';
    (sibling ? root : host).append(marquee);
    document.body.append(root);
    try {
      host.focus();
      expect(document.activeElement).toBe(host);
      expect(host.matches(":focus-visible")).toBe(true);
      const track = marquee.firstElementChild!;
      expect(
        pauseSelectors.some((selector) => track.matches(selector)),
        markup,
      ).toBe(paused);
    } finally {
      root.remove();
    }
  }
});
