import type { Processor } from "unified";
import { gfmTable } from "micromark-extension-gfm-table";
import { gfmStrikethrough } from "micromark-extension-gfm-strikethrough";
import { gfmTableFromMarkdown } from "mdast-util-gfm-table";
import { gfmStrikethroughFromMarkdown } from "mdast-util-gfm-strikethrough";

/**
 * gfmSubset is the unified plugin that adds GitHub's tables and `~~text~~`
 * strikethrough to Markdown parsing, and nothing else of GFM: no autolinked
 * URLs, footnotes or task lists, so a student's prompt that mentions an
 * address stays text. A single tilde is not strikethrough.
 */
export function gfmSubset(this: Processor) {
  const data = this.data();
  data.micromarkExtensions = [
    ...(data.micromarkExtensions ?? []),
    gfmTable(),
    gfmStrikethrough({ singleTilde: false }),
  ];
  data.fromMarkdownExtensions = [
    ...(data.fromMarkdownExtensions ?? []),
    gfmTableFromMarkdown(),
    gfmStrikethroughFromMarkdown(),
  ];
}
