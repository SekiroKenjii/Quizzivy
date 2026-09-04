import type { OutlineDraft } from "@/features/tests/api";

export type OutlineSection = OutlineDraft["sections"][number];

/** Where a question sits: which section, and its position within it. */
export interface QuestionAt {
  sectionIndex: number;
  index: number;
}

export function findQuestion(
  sections: OutlineSection[],
  questionId: string,
): QuestionAt | null {
  for (const [sectionIndex, section] of sections.entries()) {
    const index = section.questionIds.indexOf(questionId);
    if (index !== -1) return { sectionIndex, index };
  }
  return null;
}

export function moveQuestion(
  sections: OutlineSection[],
  from: QuestionAt,
  to: QuestionAt,
): OutlineSection[] {
  const source = sections[from.sectionIndex];
  if (!source) return sections;
  const questionId = source.questionIds[from.index];
  if (questionId === undefined) return sections;

  const without = sections.map((section, i) =>
    i === from.sectionIndex
      ? {
          ...section,
          questionIds: section.questionIds.filter((_, j) => j !== from.index),
        }
      : section,
  );

  const target = without[to.sectionIndex];
  if (!target) return sections;
  const at = Math.max(0, Math.min(to.index, target.questionIds.length));
  const questionIds = [...target.questionIds];
  questionIds.splice(at, 0, questionId);

  return without.map((section, i) =>
    i === to.sectionIndex ? { ...section, questionIds } : section,
  );
}

/**
 * One step up or down in reading order, crossing a section boundary rather than
 * stopping at it.
 */
export function stepQuestion(
  sections: OutlineSection[],
  at: QuestionAt,
  direction: -1 | 1,
): QuestionAt | null {
  const section = sections[at.sectionIndex];
  if (!section) return null;

  const next = at.index + direction;
  if (next >= 0 && next <= section.questionIds.length - 1) {
    return { sectionIndex: at.sectionIndex, index: next };
  }

  const neighbour = at.sectionIndex + direction;
  const into = sections[neighbour];
  if (!into) return null;
  return {
    sectionIndex: neighbour,
    index: direction === -1 ? into.questionIds.length : 0,
  };
}

/** A whole section among its siblings, the questions it holds travelling with it. */
export function moveSection(
  sections: OutlineSection[],
  from: number,
  to: number,
): OutlineSection[] {
  const section = sections[from];
  if (!section || from === to || to < 0 || to > sections.length - 1) return sections;
  const next = sections.filter((_, i) => i !== from);
  next.splice(to, 0, section);
  return next;
}

/** A question out of this test only; one that came from the bank stays in the bank. */
export function removeQuestion(
  sections: OutlineSection[],
  at: QuestionAt,
): OutlineSection[] {
  const section = sections[at.sectionIndex];
  if (!section || at.index < 0 || at.index > section.questionIds.length - 1) {
    return sections;
  }
  return sections.map((each, i) =>
    i === at.sectionIndex
      ? { ...each, questionIds: each.questionIds.filter((_, j) => j !== at.index) }
      : each,
  );
}
