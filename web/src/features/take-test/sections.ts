import type { StudentQuestion, StudentSection } from "./api";

export interface SectionGroup {
  section: StudentSection;
  /** 1-based, in test order: the "2" of "Phần 2". */
  ordinal: number;
  /** Positions in presentation order, ascending. */
  indexes: number[];
}

/**
 * The navigator's grouping (S-06, S-08): sections in test order, each with the
 * positions of its questions. A question whose section the payload did not
 * list lands in a group of its own at the end, so no dot is ever dropped.
 */
export function groupBySection(
  sections: StudentSection[],
  questions: StudentQuestion[],
): SectionGroup[] {
  const groups: SectionGroup[] = sections.map((section, i) => ({
    section,
    ordinal: i + 1,
    indexes: [],
  }));
  const byId = new Map(groups.map((group) => [group.section.id, group]));
  questions.forEach((question, index) => {
    let group = byId.get(question.sectionId);
    if (group === undefined) {
      group = {
        section: { id: question.sectionId, title: "", instructions: null },
        ordinal: groups.length + 1,
        indexes: [],
      };
      groups.push(group);
      byId.set(question.sectionId, group);
    }
    group.indexes.push(index);
  });
  return groups.filter((group) => group.indexes.length > 0);
}

/** The group holding the question at `index`, or null for an index off the paper. */
export function sectionAt(groups: SectionGroup[], index: number): SectionGroup | null {
  return groups.find((group) => group.indexes.includes(index)) ?? null;
}

/** S-05 states a section's instructions once, on its first question in presentation order. */
export function opensSection(group: SectionGroup, index: number): boolean {
  return group.indexes[0] === index;
}
