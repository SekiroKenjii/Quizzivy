import { icons } from "lucide-react";
import { describe, expect, it } from "vitest";

import { DECK_ICONS } from "@/lib/icons";

const pascal = (kebab: string) =>
  kebab
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");

describe("deck icon names", () => {
  it("map to lucide's canonical icons, never a deprecated alias", () => {
    for (const [name, component] of Object.entries(DECK_ICONS)) {
      expect(icons[pascal(name) as keyof typeof icons], name).toBe(component);
    }
  });
});
