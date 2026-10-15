import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const DIR = resolve(import.meta.dirname, "../../../src/features/classes/components");

describe("a paused join-code control keeps its whole focus ring (QC-U-16)", () => {
  it.each(["ClassCard.tsx", "JoinCodePanel.tsx"])(
    "%s fades no aria-disabled control by opacity on itself, which halves its outline",
    (file) => {
      const source = readFileSync(resolve(DIR, file), "utf8");
      const faded = source
        .split("\n")
        .filter(
          (line) =>
            /aria-disabled:opacity-/.test(line) &&
            !line.includes("DropdownMenuItem") &&
            !line.includes("heldBackItem") &&
            !/className: "aria-disabled:opacity-50"/.test(line),
        );
      expect(faded).toEqual([]);
    },
  );
});
