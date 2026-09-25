import { z } from "zod";
import type { components } from "@/lib/api/schema";
import type { ContentBlock } from "./model";
import { validateContent } from "./validation";

export type QuestionContent = components["schemas"]["QuestionContent"];

export type QuestionPromptContent = components["schemas"]["QuestionPromptContent"];

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
