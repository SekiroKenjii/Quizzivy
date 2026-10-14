import { describe, expect, it } from "vitest";
import { BuilderWrites } from "@/features/tests/BuilderWrites";
import {
  dropRequest,
  findMember,
  findUnit,
  gapBoundMembers,
  keepClientIds,
  moveUnit,
  reconcileSections,
  stepUnit,
  unitKey,
  unitsOf,
} from "@/features/tests/outlineUnits";
import type { OutlineSection } from "@/features/tests/outline";
import type { Test } from "@/features/tests/api";
import type { GroupBundle } from "@/features/question-groups/api";

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

function group(id: string, members: string[], bound: string[] = []): GroupBundle {
  return {
    group: {
      id,
      title: id,
      members: members.map((questionId) => ({ questionId, optionOrder: "shuffle" })),
      stimuli: [
        {
          id: `${id}-material`,
          title: "Passage",
          content: { format: "semantic_v1", blocks: [] },
          gaps: bound.map((questionId, index) => ({
            kind: "question",
            gapId: `g${index + 1}`,
            questionId,
          })),
        },
      ],
      recordings: [],
    },
    questions: [],
  };
}

function grouped(): OutlineSection[] {
  return [
    {
      id: "s1",
      clientId: "s1",
      title: "Reading",
      instructions: null,
      questionIds: ["q1", "q2"],
      units: [
        { kind: "question", id: "q1" },
        { kind: "group", id: "g1" },
        { kind: "question", id: "q2" },
      ],
    },
    {
      id: "s2",
      clientId: "s2",
      title: "Listening",
      instructions: null,
      questionIds: [],
      units: [{ kind: "group", id: "g2" }],
    },
  ];
}

const groups = new Map([
  ["g1", group("g1", ["m1", "m2", "m3"], ["m3"])],
  ["g2", group("g2", ["n1"])],
]);

function drop(
  key: string,
  place: Parameters<typeof dropRequest>[3],
  shift = true,
): ReturnType<typeof dropRequest> {
  return dropRequest(grouped(), groups, key, place, shift);
}

describe("finding a member and resolving a drop", () => {
  it("finds a member by its outline key, and nothing for a unit or a group", () => {
    expect(findMember(groups, "question:m2")).toEqual({ groupId: "g1", index: 1 });
    expect(findMember(groups, "question:n1")).toEqual({ groupId: "g2", index: 0 });
    expect(findMember(groups, "question:q1")).toBeNull();
    expect(findMember(groups, "group:g1")).toBeNull();
    expect(findMember(groups, "m1")).toBeNull();
  });

  it("keeps a move between unit places on the outline's own write", () => {
    expect(drop("question:q1", { kind: "unit", sectionIndex: 0, index: 3 })).toEqual({
      kind: "units",
      from: { sectionIndex: 0, index: 0 },
      to: { sectionIndex: 0, index: 2 },
    });
    expect(
      drop("question:q1", { kind: "unit", sectionIndex: 0, index: 2 }, false),
    ).toEqual({
      kind: "units",
      from: { sectionIndex: 0, index: 0 },
      to: { sectionIndex: 0, index: 2 },
    });
    expect(drop("group:g1", { kind: "unit", sectionIndex: 1, index: 0 })).toEqual({
      kind: "units",
      from: { sectionIndex: 0, index: 1 },
      to: { sectionIndex: 1, index: 0 },
    });
  });

  it("asks for a copy into a group at the place dropped on, touching only that group", () => {
    expect(drop("question:q2", { kind: "member", groupId: "g1", index: 0 })).toEqual({
      kind: "question",
      move: {
        questionId: "q2",
        to: { sectionIndex: 0, groupId: "g1", index: 0 },
        groupIds: ["g1"],
      },
    });
    expect(drop("question:q1", { kind: "member", groupId: "g2", index: 1 })).toEqual({
      kind: "question",
      move: {
        questionId: "q1",
        to: { sectionIndex: 1, groupId: "g2", index: 1 },
        groupIds: ["g2"],
      },
    });
  });

  it("moves a member out before or after a unit, counting the units as they are", () => {
    expect(drop("question:m1", { kind: "unit", sectionIndex: 0, index: 0 })).toEqual({
      kind: "question",
      move: {
        questionId: "m1",
        to: { sectionIndex: 0, groupId: null, index: 0 },
        groupIds: ["g1"],
      },
    });
    expect(drop("question:m1", { kind: "unit", sectionIndex: 0, index: 3 })).toEqual({
      kind: "question",
      move: {
        questionId: "m1",
        to: { sectionIndex: 0, groupId: null, index: 3 },
        groupIds: ["g1"],
      },
    });
  });

  it("moves a member to another group, touching both", () => {
    expect(drop("question:m2", { kind: "member", groupId: "g2", index: 1 })).toEqual({
      kind: "question",
      move: {
        questionId: "m2",
        to: { sectionIndex: 1, groupId: "g2", index: 1 },
        groupIds: ["g1", "g2"],
      },
    });
  });

  it("reorders a member within its group once it has left its place, and ignores a drop in place", () => {
    const within = (index: number, shift: boolean) =>
      drop("question:m1", { kind: "member", groupId: "g1", index }, shift);
    const toTwo = {
      kind: "question",
      move: {
        questionId: "m1",
        to: { sectionIndex: 0, groupId: "g1", index: 2 },
        groupIds: ["g1"],
      },
    };
    expect(within(3, true)).toEqual(toTwo);
    expect(within(2, false)).toEqual(toTwo);
    expect(within(1, true)).toBeNull();
    expect(within(0, false)).toBeNull();
  });

  it("refuses a group into a group, and anything it cannot place", () => {
    expect(drop("group:g2", { kind: "member", groupId: "g1", index: 0 })).toBeNull();
    expect(
      drop("question:nowhere", { kind: "unit", sectionIndex: 0, index: 0 }),
    ).toBeNull();
    expect(
      drop("question:q1", { kind: "member", groupId: "gone", index: 0 }),
    ).toBeNull();
  });

  it("names the members a material gap is bound to", () => {
    expect([...gapBoundMembers(groups.get("g1")!)]).toEqual(["m3"]);
    expect(gapBoundMembers(groups.get("g2")!).size).toBe(0);
  });

  it("keeps each drawn section's client identity when the outline comes back from the server", () => {
    const current = sections().map((section) =>
      section.clientId === "new" ? { ...section, id: "assigned" } : section,
    );
    const next: OutlineSection[] = [
      {
        id: "assigned",
        clientId: "assigned",
        title: "Listening",
        instructions: null,
        questionIds: [],
      },
      { id: "a", clientId: "a", title: "Reading", instructions: null, questionIds: [] },
      {
        id: "other",
        clientId: "other",
        title: "New",
        instructions: null,
        questionIds: [],
      },
    ];
    expect(keepClientIds(next, current).map((section) => section.clientId)).toEqual([
      "new",
      "a",
      "other",
    ]);
  });
});
