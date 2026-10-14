import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

describe("the availability map", () => {
  it("imports nothing, so reading it pulls no screen into a chunk", () => {
    const source = readFileSync(
      resolve(import.meta.dirname, "../../../src/app/modules.ts"),
      "utf8",
    );
    expect(source.split("\n").filter((line) => line.startsWith("import"))).toEqual([]);
  });
});
