import type { ContentBlock, ContentInline } from "./model";
import type { QuestionPromptContent } from "./questionContent";

export type QuestionGap = Extract<ContentInline, { type: "gap" }>;

/** questionGaps returns stable gaps in document order, including nested lists and table cells. */
export function questionGaps(document: QuestionPromptContent): QuestionGap[] {
  const gaps: QuestionGap[] = [];
  const visit = (block: ContentBlock) => {
    switch (block.type) {
      case "paragraph":
      case "heading":
        for (const node of block.content) if (node.type === "gap") gaps.push(node);
        break;
      case "list":
        block.items.forEach((item) => item.forEach(visit));
        break;
      case "table":
        block.rows.forEach((row) => row.forEach((cell) => cell.content.forEach(visit)));
        break;
    }
  };
  document.blocks.forEach(visit);
  return gaps;
}

/** gapBindingsMatch requires one metadata row for each gap, without interpreting its label or ordinal. */
export function gapBindingsMatch(
  document: QuestionPromptContent,
  blanks: ReadonlyArray<{ gapId?: string | null | undefined }>,
): boolean {
  const remaining = new Set(questionGaps(document).map((gap) => gap.id));
  if (remaining.size !== blanks.length) return false;
  for (const blank of blanks)
    if (blank.gapId == null || !remaining.delete(blank.gapId)) return false;
  return remaining.size === 0;
}
