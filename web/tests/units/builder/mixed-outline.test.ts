import { describe, expect, it } from "vitest";
import { BuilderWrites } from "@/features/tests/BuilderWrites";
import {
  findUnit,
  moveUnit,
  reconcileSections,
  stepUnit,
  unitKey,
  unitsOf,
} from "@/features/tests/outlineUnits";
import type { OutlineSection } from "@/features/tests/outline";
import type { Test } from "@/features/tests/api";

function sections(): OutlineSection[] {
  return [
    {
      id: "a",
      clientId: "a",
      title: "Reading",
      instructions: null,
      questionIds: ["same"],
      units: [
        { kind: "question", id: "same" },
        { kind: "group", id: "same" },
      ],
    },
    {
      id: null,
      clientId: "new",
      title: "Listening",
      instructions: null,
      questionIds: [],
      units: [],
    },
  ];
}

describe("mixed outline identity and acknowledgements", () => {
  it("moves an empty group as one unit into an empty section without projecting its id as a standalone question", () => {
    const start = sections();
    const from = findUnit(start, "group:same")!;
    const target = stepUnit(start, from, 1)!;
    const moved = moveUnit(start, from, target);
    expect(unitsOf(moved[0]!).map(unitKey)).toEqual(["question:same"]);
    expect(unitsOf(moved[1]!).map(unitKey)).toEqual(["group:same"]);
    expect(moved.map((section) => section.questionIds)).toEqual([["same"], []]);
    expect(start[0]?.units).toHaveLength(2);
  });

  it("reconciles a newly saved section after it was renamed and reordered during the request", () => {
    const submitted = sections();
    const current = [{ ...submitted[1]!, title: "Newer title" }, submitted[0]!];
    const saved = { sections: [{ id: "a" }, { id: "assigned" }] } as Test;
    const next = reconcileSections(current, submitted, saved);
    expect(
      next.map((section) => [section.id, section.clientId, section.title]),
    ).toEqual([
      ["assigned", "new", "Newer title"],
      ["a", "a", "Reading"],
    ]);
  });

  it("does not overlap parent writes and allows a retry after a failed operation", async () => {
    const writes = new BuilderWrites();
    const calls: string[] = [];
    let finish = () => undefined as void;
    const first = writes.run(async () => {
      calls.push("first");
      await new Promise<void>((resolve) => {
        finish = resolve;
      });
      throw new Error("offline");
    });
    const rejected = expect(first).rejects.toThrow("offline");
    const second = writes.run(async () => {
      calls.push("retry");
      return 2;
    });
    await Promise.resolve();
    expect(calls).toEqual(["first"]);
    finish();
    await rejected;
    expect(await second).toBe(2);
    expect(calls).toEqual(["first", "retry"]);
  });
});
