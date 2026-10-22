import { z } from "zod";
import type { components } from "@/lib/api/schema";
import type { ContentBlock } from "./model";
import { validateContent } from "./validation";

export type QuestionContent = components["schemas"]["QuestionContent"];

/** The contract's media blocks (DG-109), which the editor and the paper do not draw until R6-14. */
type MediaBlock = { type: "image" | "audio" };

/** ProseOnly narrows a document to its prose blocks, the subset this client writes and reads until R6-14. */
type ProseOnly<T extends { blocks: readonly unknown[] }> = Omit<T, "blocks"> & {
  blocks: Exclude<T["blocks"][number], MediaBlock>[];
};

/** A prompt as this client handles it: the guards below refuse a media block, as the server does until R6-13. */
export type QuestionPromptContent = ProseOnly<
  components["schemas"]["QuestionPromptContent"]
>;

/** A prompt or explanation as the server sends it, which may hold media; render it only through the guards below. */
export type StoredQuestionProse =
  | components["schemas"]["QuestionPromptContent"]
  | components["schemas"]["QuestionExplanationContent"];

/** An explanation as this client handles it, prose only, as for prompts. */
export type QuestionExplanationContent = ProseOnly<
  components["schemas"]["QuestionExplanationContent"]
>;

function boundProse(block: ContentBlock, gaps = false): boolean {
  switch (block.type) {
    case "image":
    case "audio":
      return false;
    case "paragraph":
    case "heading":
      return gaps || block.content.every((node) => node.type !== "gap");
    case "list":
      return block.items.every((item) => item.every((node) => boundProse(node, gaps)));
    case "table":
      return block.rows.every((row) =>
        row.every((cell) => cell.content.every((node) => boundProse(node, gaps))),
      );
  }
}

/** isQuestionContent validates prose recursively before any asset or gap binding is supported. */
export function isQuestionContent(value: unknown): value is QuestionContent {
  const parsed = validateContent(value);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    parsed.value.blocks.every((node) => boundProse(node))
  );
}

export const questionContentSchema = z.custom<QuestionContent>(
  isQuestionContent,
  "questionEditor.errors.questionContent",
);

/** isQuestionPromptContent validates bounded prose and gaps before question-level binding checks. */
export function isQuestionPromptContent(
  value: unknown,
): value is QuestionPromptContent {
  const parsed = validateContent(value);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    parsed.value.blocks.every((node) => boundProse(node, true))
  );
}

export const questionPromptContentSchema = z.custom<QuestionPromptContent>(
  isQuestionPromptContent,
  "questionEditor.errors.questionContent",
);

/**
 * formProse types stored prose for the editor's form. A media block (stored once R6-13 lands) is kept, never
 * dropped: the form's guards above refuse it, so a save cannot silently remove it before R6-14 draws it.
 */
export function formProse<T extends StoredQuestionProse>(
  content: T | null | undefined,
): ProseOnly<T> | null {
  return (content ?? null) as ProseOnly<T> | null;
}
