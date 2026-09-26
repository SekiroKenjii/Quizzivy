import type { ImportFinding } from "./api";

/** FINDING_CODES lists the finding codes this client explains; any other code falls back to a generic message. */
export const FINDING_CODES = [
  "MISSING_ANSWER",
  "CONFLICTING_ANSWER_KEYS",
  "UNUSABLE_ANSWER_KEY",
  "UNMATCHED_ANSWER_KEY",
  "AMBIGUOUS_KEY_PAPER",
  "UNSUPPORTED_INTERACTION",
  "INVALID_QUESTION",
  "INVALID_GROUP",
  "UNASSIGNED_SOURCE_TEXT",
  "UNSUPPORTED_DOCUMENT_OBJECT",
  "MULTIPLE_PAPERS_IN_SOURCE",
  "UNDERLINE_MARKS_MISSING",
  "IRREGULAR_OPTION_COUNT",
  "SCORING_DEFAULTED",
  "FORMATTING_SIMPLIFIED",
  "EMPTY_SECTION",
  "NO_QUESTIONS",
  "NUMBERING_IRREGULAR",
  "COLORED_TEXT_IN_CONTENT",
  "OPTION_LABEL_REFERENCE",
] as const;

export type FindingCode = (typeof FINDING_CODES)[number];

const KNOWN: ReadonlySet<string> = new Set(FINDING_CODES);

/** findingKey is the i18n key under imports.findings.codes that explains a finding's code. */
export function findingKey(code: string): FindingCode | "UNKNOWN" {
  return KNOWN.has(code) ? (code as FindingCode) : "UNKNOWN";
}

/** OBJECT_REASONS are the extractor reasons an UNSUPPORTED_DOCUMENT_OBJECT finding carries in `field` that have their own explanation. */
export const OBJECT_REASONS = [
  "INLINE_OBJECT_REQUIRES_REVIEW",
  "LEGACY_NORMALIZATION_REQUIRES_REVIEW",
  "TRACKED_CHANGE_REQUIRES_REVIEW",
  "HIDDEN_TEXT_REQUIRES_REVIEW",
  "FIELD_REQUIRES_REVIEW",
  "TEXTBOX_ORDER_REQUIRES_REVIEW",
  "ANCILLARY_CONTENT_REQUIRES_REVIEW",
  "OBJECT_REQUIRES_REVIEW",
  "TABLE_GRID_REQUIRES_REVIEW",
  "COLUMN_ORDER_REQUIRES_REVIEW",
  "SOURCE_COLOR_REQUIRES_REVIEW",
  "RENDERER_LAYOUT_REQUIRES_REVIEW",
  "ALTERNATE_CONTENT_REQUIRES_RESOLUTION",
  "PDF_MARKS_UNAVAILABLE",
  "PDF_REPEATED_LINE_REQUIRES_REVIEW",
  "PDF_COLUMNS_REQUIRE_REVIEW",
] as const;

const KNOWN_REASONS: ReadonlySet<string> = new Set(OBJECT_REASONS);

/** TITLED_REASONS are the document-object reasons whose finding has its own title rather than the generic "objects not read". */
export const TITLED_REASONS = [
  "PDF_MARKS_UNAVAILABLE",
  "PDF_REPEATED_LINE_REQUIRES_REVIEW",
  "PDF_COLUMNS_REQUIRE_REVIEW",
] as const;

const TITLED: ReadonlySet<string> = new Set(TITLED_REASONS);

/** titledReason is the explained reason of a document-object finding that has its own title, or null. */
export function titledReason(finding: ImportFinding): string | null {
  const reason = objectReason(finding);
  return reason !== null && TITLED.has(reason) ? reason : null;
}

/** objectReason is the explained extractor reason of a document-object finding, or null for a generic explanation. */
export function objectReason(finding: ImportFinding): string | null {
  if (finding.code !== "UNSUPPORTED_DOCUMENT_OBJECT" || finding.field === undefined)
    return null;
  return KNOWN_REASONS.has(finding.field) ? finding.field : null;
}

export type FindingFilter = "all" | "blocking" | "review";

/** FINDING_FILTERS orders the review workspace's finding filter. */
export const FINDING_FILTERS: readonly FindingFilter[] = ["all", "blocking", "review"];

/** isFindingFilter narrows an untrusted URL value to a finding filter. */
export function isFindingFilter(value: string | null): value is FindingFilter {
  return value !== null && (FINDING_FILTERS as readonly string[]).includes(value);
}

/** isUnresolved reports whether a finding still stands between the draft and a commit. */
export function isUnresolved(finding: ImportFinding): boolean {
  if (finding.severity === "blocking") return true;
  return finding.severity === "review_required" && finding.acknowledged !== true;
}

/** matchesFilter decides whether a finding belongs to a filter's navigation set; informational findings never do. */
export function matchesFilter(finding: ImportFinding, filter: FindingFilter): boolean {
  if (finding.severity === "informational") return false;
  if (filter === "blocking") return finding.severity === "blocking";
  if (filter === "review")
    return finding.severity === "review_required" && finding.acknowledged !== true;
  return true;
}

/** findingRank is where a finding's target sits in the draft; findings about the whole import rank first. */
export function findingRank(
  finding: ImportFinding,
  positions: ReadonlyMap<string, number>,
): number {
  return finding.target === undefined ? -1 : (positions.get(finding.target) ?? -1);
}

/** orderFindings sorts findings by findingRank, keeping the server's order within a rank. */
export function orderFindings(
  findings: readonly ImportFinding[],
  positions: ReadonlyMap<string, number>,
): ImportFinding[] {
  return findings
    .map((finding, index) => ({ finding, index, at: findingRank(finding, positions) }))
    .sort((a, b) => a.at - b.at || a.index - b.index)
    .map(({ finding }) => finding);
}

/** FindingAnchor is the finding the teacher last moved to and the rank it had. */
export interface FindingAnchor {
  id: string;
  at: number;
}

/**
 * stepFinding picks the finding Previous or Next moves to in an ordered list.
 * When the anchor has left the list, for example because it was resolved,
 * Next continues from the anchor's rank and Previous from the finding before
 * it, wrapping around at either end.
 */
export function stepFinding(
  ordered: readonly ImportFinding[],
  anchor: FindingAnchor | null,
  delta: 1 | -1,
  positions: ReadonlyMap<string, number>,
): ImportFinding | undefined {
  const count = ordered.length;
  if (count === 0) return undefined;
  const index =
    anchor === null ? -1 : ordered.findIndex((item) => item.id === anchor.id);
  if (index !== -1) return ordered[(index + delta + count) % count];
  if (anchor === null) return delta === 1 ? ordered[0] : ordered[count - 1];
  if (delta === 1)
    return (
      ordered.find((item) => findingRank(item, positions) >= anchor.at) ?? ordered[0]
    );
  const before = ordered.filter((item) => findingRank(item, positions) < anchor.at);
  return before[before.length - 1] ?? ordered[count - 1];
}

/** keyPapers parses the paper numbers an AMBIGUOUS_KEY_PAPER finding offers from its field. */
export function keyPapers(finding: ImportFinding): number[] {
  if (finding.field === undefined) return [];
  const papers = finding.field
    .split(",")
    .map((part) => Number.parseInt(part.trim(), 10))
    .filter((value) => Number.isInteger(value) && value > 0);
  return [...new Set(papers)];
}
