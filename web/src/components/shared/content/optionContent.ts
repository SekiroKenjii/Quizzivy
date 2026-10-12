import { z } from "zod";
import type { components } from "@/lib/api/schema";
import { validateContent } from "./validation";

export type OptionContent = components["schemas"]["OptionContent"];

/** isOptionContent narrows validated content to a single paragraph of marked text and breaks. */
export function isOptionContent(value: unknown): value is OptionContent {
  const parsed = validateContent(value);
  if (!parsed.ok || parsed.value.format !== "semantic_v1") return false;
  const blocks = parsed.value.blocks;
  return (
    blocks.length === 1 &&
    blocks[0]?.type === "paragraph" &&
    blocks[0].content.every((node) => node.type === "text" || node.type === "break")
  );
}

export const optionContentSchema = z.custom<OptionContent>(
  isOptionContent,
  "questionEditor.errors.optionContent",
);

/** plainOptionContent preserves literal option text, including line breaks, without interpreting Markdown. */
export function plainOptionContent(text: string): OptionContent {
  return {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: text
          .split("\n")
          .flatMap((line, index) => [
            ...(index === 0 ? [] : [{ type: "break" as const }]),
            ...(line === "" ? [] : [{ type: "text" as const, text: line, marks: [] }]),
          ]),
      },
    ],
  };
}

/**
 * isPlainOptionContent says whether content is one paragraph of unmarked text
 * (and line breaks) that reads exactly `text`: the shape plainOptionContent
 * writes and the Word import commits for an unformatted option.
 */
export function isPlainOptionContent(content: unknown, text: string): boolean {
  if (!isOptionContent(content)) return false;
  let plain = "";
  for (const node of content.blocks[0]!.content) {
    if (node.type === "break") plain += "\n";
    else if (node.type === "text" && node.marks.length === 0) plain += node.text;
    else return false;
  }
  return plain === text;
}
