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

/**
 * SectionHead is what the question column says above every question of a
 * section: its title and, when it has some, its instructions.
 */
export interface SectionHead {
  title: string;
  instructions: string;
}

/**
 * sectionHead is the head above the questions of `group` (S-05, DG-192): the
 * section's title over its instructions. A paper of one section with no
 * instructions has none, and neither has a section with no title and no
 * instructions.
 */
export function sectionHead(
  groups: SectionGroup[],
  group: SectionGroup | null,
): SectionHead | null {
  if (group === null) return null;
  const title = group.section.title.trim();
  const instructions = group.section.instructions?.trim() ?? "";
  if (instructions === "" && (groups.length < 2 || title === "")) return null;
  return { title, instructions };
}
