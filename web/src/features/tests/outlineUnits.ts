import type { GroupBundle } from "@/features/question-groups/api";
import type { MixedOutlineSection, Test } from "./api";
import type { OutlineSection, QuestionAt } from "./outline";

export type OutlineUnit = MixedOutlineSection["units"][number];

const QUESTION_KEY = "question:";

export function unitKey(unit: OutlineUnit): string {
  return `${unit.kind}:${unit.id}`;
}

export function unitsOf(section: OutlineSection): OutlineUnit[] {
  return section.units ?? section.questionIds.map((id) => ({ kind: "question", id }));
}

export function sectionQuestionIds(
  section: OutlineSection,
  groups: Map<string, GroupBundle>,
): string[] {
  return unitsOf(section).flatMap((unit) =>
    unit.kind === "question"
      ? [unit.id]
      : (groups.get(unit.id)?.group.members.map((member) => member.questionId) ?? []),
  );
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

/** MemberAt is where a member of a shared-context group sits: its group, and its position among the group's members. */
export interface MemberAt {
  groupId: string;
  index: number;
}

/** findMember locates the group member whose outline key is `key` ("question:<id>"), or answers null when no group holds it. */
export function findMember(
  groups: ReadonlyMap<string, GroupBundle>,
  key: string,
): MemberAt | null {
  if (!key.startsWith(QUESTION_KEY)) return null;
  const questionId = key.slice(QUESTION_KEY.length);
  for (const [groupId, bundle] of groups) {
    const index = bundle.group.members.findIndex(
      (member) => member.questionId === questionId,
    );
    if (index !== -1) return { groupId, index };
  }
  return null;
}

/**
 * OutlinePlace is a place a dragged item can be dropped on: a position among a
 * section's units, or a position among a group's members. Its index counts the
 * item where it still is.
 */
export type OutlinePlace =
  | { kind: "unit"; sectionIndex: number; index: number }
  | { kind: "member"; groupId: string; index: number };

/**
 * QuestionMove asks for one question to cross a shared-context group's
 * boundary or to move within a group: `to.groupId` null places it among the
 * units of section `to.sectionIndex`, set places it among that group's
 * members. `to.index` counts the question once it has left its old place.
 * `groupIds` names every group the move touches, once each.
 */
export interface QuestionMove {
  questionId: string;
  to: { sectionIndex: number; groupId: string | null; index: number };
  groupIds: string[];
}

/**
 * OutlineDrop is what dropping an item on a place asks for: `units` moves a
 * whole unit between unit positions, which the outline's autosave writes;
 * `question` is a move only the server can make.
 */
export type OutlineDrop =
  | { kind: "units"; from: QuestionAt; to: QuestionAt }
  | { kind: "question"; move: QuestionMove };

/**
 * dropRequest resolves dropping the item keyed `activeKey` on `place`. When
 * `shift` is set, a place below the item in its own list moves up by one, as a
 * pointer's "after this row" does once the item has left; a keyboard drop
 * takes the place of the row it lands on and passes false. It answers null for
 * a drop that changes nothing or that the outline does not allow: a group into
 * a group.
 */
export function dropRequest(
  sections: OutlineSection[],
  groups: ReadonlyMap<string, GroupBundle>,
  activeKey: string,
  place: OutlinePlace,
  shift: boolean,
): OutlineDrop | null {
  const fromUnit = findUnit(sections, activeKey);
  const fromMember = fromUnit ? null : findMember(groups, activeKey);
  const questionId = activeKey.slice(QUESTION_KEY.length);
  if (place.kind === "unit") {
    if (fromUnit)
      return { kind: "units", from: fromUnit, to: unitPlace(fromUnit, place, shift) };
    if (!fromMember) return null;
    const to = { sectionIndex: place.sectionIndex, groupId: null, index: place.index };
    return {
      kind: "question",
      move: { questionId, to, groupIds: [fromMember.groupId] },
    };
  }
  const owner = findUnit(sections, `group:${place.groupId}`);
  if (!owner) return null;
  const to = {
    sectionIndex: owner.sectionIndex,
    groupId: place.groupId,
    index: place.index,
  };
  if (fromUnit) {
    const unit = unitsOf(sections[fromUnit.sectionIndex]!)[fromUnit.index];
    if (unit?.kind !== "question") return null;
    return {
      kind: "question",
      move: { questionId: unit.id, to, groupIds: [place.groupId] },
    };
  }
  if (!fromMember) return null;
  if (fromMember.groupId !== place.groupId)
    return {
      kind: "question",
      move: { questionId, to, groupIds: [fromMember.groupId, place.groupId] },
    };
  const index = shift && fromMember.index < place.index ? place.index - 1 : place.index;
  if (index === fromMember.index) return null;
  return {
    kind: "question",
    move: { questionId, to: { ...to, index }, groupIds: [place.groupId] },
  };
}

function unitPlace(
  from: QuestionAt,
  place: { sectionIndex: number; index: number },
  shift: boolean,
): QuestionAt {
  const below = from.sectionIndex === place.sectionIndex && from.index < place.index;
  return {
    sectionIndex: place.sectionIndex,
    index: shift && below ? place.index - 1 : place.index,
  };
}

/** gapBoundMembers lists the members of a group that a gap of its shared material is bound to; they cannot leave the group. */
export function gapBoundMembers(bundle: GroupBundle): ReadonlySet<string> {
  return new Set(
    bundle.group.stimuli.flatMap((stimulus) =>
      stimulus.gaps.map((gap) => gap.questionId),
    ),
  );
}

/**
 * keepClientIds carries the client identity of each section of `current` to
 * the section of `next` with the same server id, so a section the outline
 * already draws keeps its state when the outline is replaced from a response.
 */
export function keepClientIds(
  next: OutlineSection[],
  current: OutlineSection[],
): OutlineSection[] {
  const clientIds = new Map(
    current.flatMap((section) =>
      section.id && section.clientId ? [[section.id, section.clientId] as const] : [],
    ),
  );
  return next.map((section) => {
    const clientId = section.id === null ? undefined : clientIds.get(section.id);
    return clientId === undefined ? section : { ...section, clientId };
  });
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
