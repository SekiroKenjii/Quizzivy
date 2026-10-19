import { describe, expect, it } from "vitest";
import type { GroupBundle } from "@/features/question-groups/api";
import { aiPassagesIn, aiSections } from "@/features/tests/aiSections";
import type { OutlineSection } from "@/features/tests/outline";

const bundle = {
  group: { members: [{ questionId: "m1" }, { questionId: "m2" }] },
  questions: [
    { id: "m1", input: { type: "true_false", level: "c1" } },
    { id: "m2", input: { type: "matching", level: "c1" } },
  ],
} as unknown as GroupBundle;

const sections: OutlineSection[] = [
  {
    id: "s1",
    title: "Ngữ pháp",
    instructions: "  ",
    questionIds: ["q1", "q2"],
    units: [
      { kind: "question", id: "q1" },
      { kind: "group", id: "g1" },
      { kind: "question", id: "q2" },
    ],
  },
  { id: null, title: "Chưa lưu", instructions: null, questionIds: [] },
  { id: "s2", title: "", instructions: "Đọc kỹ.", questionIds: [] },
];

const questions = new Map([
  ["q1", { type: "single_choice", level: "b2" }],
  ["q2", { type: "single_choice", level: "c1" }],
]);

describe("aiSections", () => {
  it("describes each saved section with its group members counted, the types Generate writes and the commonest level", () => {
    expect(
      aiSections(sections, new Map([["g1", bundle]]), (id) => questions.get(id)),
    ).toEqual([
      {
        id: "s1",
        title: "Ngữ pháp",
        instructions: false,
        questions: 4,
        types: ["single_choice", "true_false"],
        level: "c1",
      },
      { id: "s2", title: "", instructions: true, questions: 0 },
    ]);
  });

  it("gives a passage the section that holds its group", () => {
    const passage = { id: "p1", title: "", text: "x", group: { id: "g1", title: "" } };
    const loose = { id: "p2", title: "", text: "y" };
    expect(aiPassagesIn([passage, loose], sections)).toEqual([
      { ...passage, sectionId: "s1" },
      loose,
    ]);
  });
});
