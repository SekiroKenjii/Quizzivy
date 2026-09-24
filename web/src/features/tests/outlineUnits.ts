import type { MixedOutlineSection, Test } from "./api";
import type { OutlineSection, QuestionAt } from "./outline";

export type OutlineUnit = MixedOutlineSection["units"][number];

export function unitKey(unit: OutlineUnit): string {
  return `${unit.kind}:${unit.id}`;
}

export function unitsOf(section: OutlineSection): OutlineUnit[] {
  return section.units ?? section.questionIds.map((id) => ({ kind: "question", id }));
}

export function withUnits(
  section: OutlineSection,
  units: OutlineUnit[],
): OutlineSection {
  return {
    ...section,
    units,
    questionIds: units
      .filter((unit) => unit.kind === "question")
      .map((unit) => unit.id),
  };
}

export function editableOutline(test: Test): OutlineSection[] {
  return test.sections.map((section) => ({
    id: section.id,
    clientId: section.id,
    title: section.title,
    instructions: section.instructions ?? null,
    questionIds: section.questionIds,
    units: section.units ?? section.questionIds.map((id) => ({ kind: "question", id })),
  }));
}

export function findUnit(sections: OutlineSection[], key: string): QuestionAt | null {
  for (const [sectionIndex, section] of sections.entries()) {
    const index = unitsOf(section).findIndex((unit) => unitKey(unit) === key);
    if (index !== -1) return { sectionIndex, index };
  }
  return null;
}

export function moveUnit(
  sections: OutlineSection[],
  from: QuestionAt,
  to: QuestionAt,
): OutlineSection[] {
  const source = sections[from.sectionIndex];
  const target = sections[to.sectionIndex];
  const unit = source && unitsOf(source)[from.index];
  if (!unit || !target) return sections;
  const next = sections.map((section, index) =>
    index === from.sectionIndex
      ? withUnits(
          section,
          unitsOf(section).filter((_, i) => i !== from.index),
        )
      : section,
  );
  const destination = next[to.sectionIndex]!;
  const units = [...unitsOf(destination)];
  units.splice(Math.max(0, Math.min(to.index, units.length)), 0, unit);
  next[to.sectionIndex] = withUnits(destination, units);
  return next;
}

export function stepUnit(
  sections: OutlineSection[],
  at: QuestionAt,
  direction: -1 | 1,
): QuestionAt | null {
  const section = sections[at.sectionIndex];
  if (!section) return null;
  const index = at.index + direction;
  if (index >= 0 && index < unitsOf(section).length)
    return { sectionIndex: at.sectionIndex, index };
  const neighbour = sections[at.sectionIndex + direction];
  return neighbour
    ? {
        sectionIndex: at.sectionIndex + direction,
        index: direction === -1 ? unitsOf(neighbour).length : 0,
      }
    : null;
}

export function removeUnit(sections: OutlineSection[], key: string): OutlineSection[] {
  return sections.map((section) =>
    withUnits(
      section,
      unitsOf(section).filter((unit) => unitKey(unit) !== key),
    ),
  );
}

/** reconcileSections attaches server identities by the submitted client identity while retaining edits made during the request. */
export function reconcileSections(
  current: OutlineSection[],
  submitted: OutlineSection[],
  saved: Test,
): OutlineSection[] {
  const ids = new Map(
    submitted.map((section, index) => [
      section.clientId ?? section.id,
      saved.sections[index]?.id,
    ]),
  );
  return current.map((section) => ({
    ...section,
    id: ids.get(section.clientId ?? section.id) ?? section.id,
  }));
}

export function groupOwners(sections: OutlineSection[]): Map<string, string | null> {
  return new Map(
    sections.flatMap((section) =>
      unitsOf(section).flatMap((unit) =>
        unit.kind === "group" ? [[unit.id, section.id] as const] : [],
      ),
    ),
  );
}
