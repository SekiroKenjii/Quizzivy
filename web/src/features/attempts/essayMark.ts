import { bandOfCriteria, pointsOfBand } from "@/lib/bands";
import type { components } from "@/lib/api/schema";

type Criterion = components["schemas"]["RubricCriterion"];
type CriterionBand = components["schemas"]["CriterionBand"];

/**
 * EssayMark is what a teacher has given an essay so far: the band they set
 * directly, or a band for each rubric criterion they have reached, keyed by
 * criterion id.
 */
export interface EssayMark {
  band: number | null;
  criteria: Readonly<Record<string, number>>;
}

export const NO_MARK: EssayMark = { band: null, criteria: {} };

/** handMarked says a question type waits for the teacher's mark: a short answer or an essay. */
export function handMarked(type: string): boolean {
  return type === "short_answer" || type === "essay";
}

/** markOf reads the mark a review answer, a by-question row or a queue item carries. */
export function markOf(source: {
  band?: number | null | undefined;
  criteria?: readonly CriterionBand[] | undefined;
}): EssayMark {
  return {
    band: source.band ?? null,
    criteria: Object.fromEntries(
      (source.criteria ?? []).map((entry) => [entry.criterionId, entry.band]),
    ),
  };
}

/**
 * markBand is the band a mark gives: the band set directly when the essay has
 * no rubric, otherwise the criteria's derived band, which is null until every
 * criterion has one.
 */
export function markBand(rubric: readonly Criterion[], mark: EssayMark): number | null {
  return rubric.length === 0 ? mark.band : bandOfCriteria(rubric, mark.criteria);
}

/** markPoints is the score the mark stores on a question worth `points`, or null while it is incomplete. */
export function markPoints(
  rubric: readonly Criterion[],
  mark: EssayMark,
  points: number,
): number | null {
  const band = markBand(rubric, mark);
  return band === null ? null : pointsOfBand(band, points);
}

/** markStarted says a rubric has some criteria but not all, which no save can write. */
export function markStarted(rubric: readonly Criterion[], mark: EssayMark): boolean {
  return (
    rubric.length > 0 &&
    rubric.some((criterion) => mark.criteria[criterion.id] !== undefined) &&
    markBand(rubric, mark) === null
  );
}

/** markSignature compares two complete marks for the fields a save writes. */
export function markSignature(rubric: readonly Criterion[], mark: EssayMark): string {
  if (rubric.length === 0) return String(mark.band);
  return rubric.map((criterion) => String(mark.criteria[criterion.id])).join(",");
}

/**
 * markFields is the grade item body for an essay: `criteria` naming every
 * criterion in rubric order when it has a rubric, else `band`. Call it only
 * with a complete mark.
 */
export function markFields(
  rubric: readonly Criterion[],
  mark: EssayMark,
): { band: number } | { criteria: CriterionBand[] } {
  if (rubric.length === 0) return { band: mark.band! };
  return {
    criteria: rubric.map((criterion) => ({
      criterionId: criterion.id,
      band: mark.criteria[criterion.id]!,
    })),
  };
}
