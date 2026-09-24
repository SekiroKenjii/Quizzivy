import { z } from "zod";
import type { components } from "@/lib/api/schema";
import type { ContentBlock } from "./model";
import { validateContent } from "./validation";

export type QuestionContent = components["schemas"]["QuestionContent"];

function boundProse(block: ContentBlock): boolean {
  switch (block.type) {
    case "image":
    case "audio":
      return false;
    case "paragraph":
    case "heading":
      return block.content.every((node) => node.type !== "gap");
    case "list":
      return block.items.every((item) => item.every(boundProse));
    case "table":
      return block.rows.every((row) =>
        row.every((cell) => cell.content.every(boundProse)),
      );
  }
}

/** isQuestionContent validates prose recursively before any asset or gap binding is supported. */
export function isQuestionContent(value: unknown): value is QuestionContent {
  const parsed = validateContent(value);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    parsed.value.blocks.every(boundProse)
  );
}

export const questionContentSchema = z.custom<QuestionContent>(
  isQuestionContent,
  "questionEditor.errors.questionContent",
);
