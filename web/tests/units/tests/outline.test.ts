import { describe, expect, it } from "vitest";
import {
  findQuestion,
  moveQuestion,
  stepQuestion,
  type OutlineSection,
} from "@/features/tests/outline";

function outline(): OutlineSection[] {
  return [
    { id: "s1", title: "Ngữ pháp", instructions: null, questionIds: ["a", "b", "c"] },
    { id: "s2", title: "Nghe", instructions: null, questionIds: ["d", "e"] },
    { id: "s3", title: "Viết", instructions: null, questionIds: [] },
  ];
}

const ids = (sections: OutlineSection[]) => sections.map((s) => s.questionIds);

describe("moving a question in the outline", () => {
  it("reorders within a section", () => {
    const moved = moveQuestion(
      outline(),
      { sectionIndex: 0, index: 0 },
      { sectionIndex: 0, index: 2 },
    );
    expect(ids(moved)).toEqual([["b", "c", "a"], ["d", "e"], []]);
  });

  it("moves across sections", () => {
    const moved = moveQuestion(
      outline(),
      { sectionIndex: 0, index: 1 },
      { sectionIndex: 1, index: 0 },
    );
    expect(ids(moved)).toEqual([["a", "c"], ["b", "d", "e"], []]);
  });

  it("moves into an empty section", () => {
    const moved = moveQuestion(
      outline(),
      { sectionIndex: 1, index: 1 },
      { sectionIndex: 2, index: 0 },
    );
    expect(ids(moved)).toEqual([["a", "b", "c"], ["d"], ["e"]]);
  });

  it("never loses or duplicates a question, wherever it lands", () => {
    const before = outline();
    for (let s = 0; s < 3; s += 1) {
      for (let i = 0; i < 4; i += 1) {
        const after = moveQuestion(
          before,
          { sectionIndex: 0, index: 1 },
          { sectionIndex: s, index: i },
        );
        expect(
          ids(after)
            .flat()
            .sort((a, b) => a.localeCompare(b)),
        ).toEqual(["a", "b", "c", "d", "e"]);
      }
    }
  });

  it("leaves the outline alone when the source does not exist", () => {
    const before = outline();
    expect(
      moveQuestion(
        before,
        { sectionIndex: 0, index: 9 },
        { sectionIndex: 1, index: 0 },
      ),
    ).toBe(before);
    expect(
      moveQuestion(
        before,
        { sectionIndex: 9, index: 0 },
        { sectionIndex: 1, index: 0 },
      ),
    ).toBe(before);
  });
});

describe("stepping a question with the keyboard", () => {
  it("moves within a section", () => {
    expect(stepQuestion(outline(), { sectionIndex: 0, index: 0 }, 1)).toEqual({
      sectionIndex: 0,
      index: 1,
    });
  });

  it("crosses into the next section rather than stopping at the boundary", () => {
    expect(stepQuestion(outline(), { sectionIndex: 0, index: 2 }, 1)).toEqual({
      sectionIndex: 1,
      index: 0,
    });
    expect(stepQuestion(outline(), { sectionIndex: 1, index: 0 }, -1)).toEqual({
      sectionIndex: 0,
      index: 3,
    });
  });

  it("stops at the very start and the very end", () => {
    expect(stepQuestion(outline(), { sectionIndex: 0, index: 0 }, -1)).toBeNull();
    expect(stepQuestion(outline(), { sectionIndex: 2, index: 0 }, 1)).toBeNull();
  });

  it("reaches every position by stepping, one at a time", () => {
    let sections = outline();
    let at = findQuestion(sections, "a")!;
    let steps = 0;
    for (
      let next = stepQuestion(sections, at, 1);
      next;
      next = stepQuestion(sections, at, 1)
    ) {
      sections = moveQuestion(sections, at, next);
      at = findQuestion(sections, "a")!;
      steps += 1;
      if (steps > 20) break;
    }
    // Five other questions plus the empty section: six steps to the far end.
    expect(steps).toBe(6);
    expect(at).toEqual({ sectionIndex: 2, index: 0 });
    expect(ids(sections).flat()).toEqual(["b", "c", "d", "e", "a"]);
  });
});
