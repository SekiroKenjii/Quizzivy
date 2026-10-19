import type { AiLevel, AiPassage, AiSection } from "@/features/ai/kinds/generate";
import type { GroupBundle } from "@/features/question-groups/api";
import type { OutlineSection } from "./outline";
import { unitsOf } from "./outlineUnits";

type AiType = NonNullable<AiSection["types"]>[number];

interface Described {
  type: string;
  level?: string | null;
}

const TYPES = new Set<string>([
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
]);
const LEVELS = new Set<string>(["b1", "b2", "c1"]);

function commonest(levels: readonly string[]): AiLevel | undefined {
  const counts = new Map<string, number>();
  for (const level of levels)
    if (LEVELS.has(level)) counts.set(level, (counts.get(level) ?? 0) + 1);
  let best: string | undefined;
  for (const [level, count] of counts)
    if (best === undefined || count > (counts.get(best) ?? 0)) best = level;
  return best as AiLevel | undefined;
}

/**
 * aiSections describes every saved section of the builder for the Generate
 * panel: whether it has instructions, how many questions it holds with group
 * members counted, the types among them that Generate can write, and the
 * level most of them have among those it can write. `question` reads a
 * loaded standalone question by id, and a group's members come from its
 * loaded bundle.
 */
export function aiSections(
  sections: readonly OutlineSection[],
  groups: ReadonlyMap<string, GroupBundle>,
  question: (id: string) => Described | undefined,
): AiSection[] {
  return sections.flatMap((section) => {
    if (!section.id) return [];
    const described = unitsOf(section).flatMap((unit): (Described | undefined)[] =>
      unit.kind === "question"
        ? [question(unit.id)]
        : (groups.get(unit.id)?.questions ?? []).map((member) => member.input),
    );
    const count = unitsOf(section).reduce(
      (sum, unit) =>
        sum +
        (unit.kind === "question"
          ? 1
          : (groups.get(unit.id)?.group.members.length ?? 0)),
      0,
    );
    const known = described.filter((item): item is Described => item !== undefined);
    const types = [...new Set(known.map((item) => item.type))].filter(
      (type): type is AiType => TYPES.has(type),
    );
    const level = commonest(known.flatMap((item) => (item.level ? [item.level] : [])));
    return [
      {
        id: section.id,
        title: section.title,
        instructions: (section.instructions ?? "").trim() !== "",
        questions: count,
        ...(types.length > 0 ? { types } : {}),
        ...(level ? { level } : {}),
      },
    ];
  });
}

/**
 * aiPassagesIn gives each passage the id of the saved section whose outline
 * holds its group, so a whole-test plan can count a section's passages.
 */
export function aiPassagesIn(
  passages: readonly AiPassage[],
  sections: readonly OutlineSection[],
): AiPassage[] {
  const owner = new Map(
    sections.flatMap((section) =>
      section.id
        ? unitsOf(section)
            .filter((unit) => unit.kind === "group")
            .map((unit) => [unit.id, section.id as string] as const)
        : [],
    ),
  );
  return passages.map((passage) => {
    const sectionId = passage.group ? owner.get(passage.group.id) : undefined;
    return sectionId ? { ...passage, sectionId } : passage;
  });
}
