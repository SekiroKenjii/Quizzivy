import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { modules } from "@/app/modules";

describe("the availability map", () => {
  it("has notifications on since R4 (T-R4.45b) and nothing else yet", () => {
    expect(modules).toEqual({
      notifications: true,
      messages: false,
      schedule: false,
      grades: false,
      learn: false,
    });
  });

  it("imports nothing, so reading it pulls no screen into a chunk", () => {
    const source = readFileSync(
      resolve(import.meta.dirname, "../../../src/app/modules.ts"),
      "utf8",
    );
    expect(source.split("\n").filter((line) => line.startsWith("import"))).toEqual([]);
  });
});
