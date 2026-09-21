import { describe, expect, it } from "vitest";
import { groupBySection, opensSection, sectionAt } from "@/features/take-test/sections";
import type { StudentQuestion, StudentSection } from "@/features/take-test/api";

const sections: StudentSection[] = [
  { id: "s1", title: "Ngữ pháp", instructions: null },
  { id: "s2", title: "Nghe", instructions: "Nghe đoạn hội thoại rồi trả lời." },
];

function question(id: string, sectionId: string): StudentQuestion {
  return { id, sectionId, type: "true_false", prompt: id, points: 1 };
}

describe("groupBySection", () => {
  it("keeps test order and numbers the parts from one", () => {
    const groups = groupBySection(sections, [
      question("q1", "s1"),
      question("q2", "s1"),
      question("q3", "s2"),
    ]);
    expect(groups.map((g) => [g.ordinal, g.section.title, g.indexes])).toEqual([
      [1, "Ngữ pháp", [0, 1]],
      [2, "Nghe", [2]],
    ]);
  });

  it("drops a section with no questions and keeps a question with no section", () => {
    const groups = groupBySection(sections, [
      question("q1", "s2"),
      question("q9", "s9"),
    ]);
    expect(groups.map((g) => [g.ordinal, g.section.id, g.indexes])).toEqual([
      [2, "s2", [0]],
      [3, "s9", [1]],
    ]);
  });
});

describe("sectionAt and opensSection", () => {
  const groups = groupBySection(sections, [
    question("q1", "s1"),
    question("q2", "s1"),
    question("q3", "s2"),
  ]);

  it("names the part a position belongs to", () => {
    expect(sectionAt(groups, 1)?.section.title).toBe("Ngữ pháp");
    expect(sectionAt(groups, 2)?.section.title).toBe("Nghe");
    expect(sectionAt(groups, 7)).toBeNull();
  });

  it("opens a part on its first question only", () => {
    const grammar = sectionAt(groups, 0)!;
    expect(opensSection(grammar, 0)).toBe(true);
    expect(opensSection(grammar, 1)).toBe(false);
    expect(opensSection(sectionAt(groups, 2)!, 2)).toBe(true);
  });
});
