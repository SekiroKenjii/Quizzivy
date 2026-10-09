import type { ContentMark } from "../model";
import type { QuestionContent } from "../questionContent";

type Block = QuestionContent["blocks"][number];
type Inline = Extract<Block, { type: "paragraph" }>["content"][number];
type Text = Extract<Inline, { type: "text" }>;
type Cell = Extract<Block, { type: "table" }>["rows"][number][number];

const KEPT: readonly ContentMark[] = ["bold", "italic", "strike"];
const DELIMITER: Partial<Record<ContentMark, string>> = {
  bold: "**",
  italic: "*",
  strike: "~~",
};

function escapeText(text: string): string {
  return text.replace(/[\\`*_[\]<>|~&]/g, "\\$&");
}

function kept(marks: readonly ContentMark[]): ContentMark[] {
  return KEPT.filter((mark) => marks.includes(mark));
}

function sameMarks(a: readonly ContentMark[], b: readonly ContentMark[]) {
  return a.length === b.length && a.every((mark, index) => mark === b[index]);
}

function runs(texts: readonly Text[]): { text: string; marks: ContentMark[] }[] {
  const result: { text: string; marks: ContentMark[] }[] = [];
  for (const node of texts) {
    const marks = kept(node.marks);
    const last = result.at(-1);
    if (last && sameMarks(last.marks, marks)) last.text += node.text;
    else result.push({ text: node.text, marks });
  }
  return result;
}

function splitSpace(value: string): { lead: string; core: string; tail: string } {
  const core = value.trim();
  if (!core) return { lead: value, core: "", tail: "" };
  const start = value.length - value.trimStart().length;
  return { lead: value.slice(0, start), core, tail: value.slice(start + core.length) };
}

function markedText(texts: readonly Text[]): string {
  let out = "";
  let open: ContentMark[] = [];
  let trail = "";
  for (const run of runs(texts)) {
    const { lead, core, tail } = splitSpace(run.text);
    if (!core) {
      trail += lead + tail;
      continue;
    }
    let common = 0;
    while (common < open.length && run.marks.includes(open[common]!)) common++;
    const keep = open.slice(0, common);
    const add = run.marks.filter((mark) => !keep.includes(mark));
    out += open
      .slice(common)
      .reverse()
      .map((mark) => DELIMITER[mark])
      .join("");
    out += escapeText(trail + lead) + add.map((mark) => DELIMITER[mark]).join("");
    out += escapeText(core);
    open = [...keep, ...add];
    trail = tail;
  }
  return (
    out +
    [...open]
      .reverse()
      .map((mark) => DELIMITER[mark])
      .join("") +
    escapeText(trail)
  );
}

function linkDestination(href: string): string {
  return /[\s()<>]/.test(href) ? `<${href.replace(/[<>\\]/g, "\\$&")}>` : href;
}

function inlineMarkdown(content: readonly Inline[], lineBreak: string): string {
  let out = "";
  let texts: Text[] = [];
  const flush = () => {
    out += markedText(texts);
    texts = [];
  };
  for (const node of content) {
    if (node.type === "text") {
      texts.push(node);
      continue;
    }
    flush();
    if (node.type === "break") out += lineBreak;
    else out += `[${markedText(node.content)}](${linkDestination(node.href)})`;
  }
  flush();
  return out;
}

function isSpaceOrTab(character: string | undefined): boolean {
  return character === " " || character === "\t";
}

function entities(space: string): string {
  return Array.from(space, (character) => `&#${character.codePointAt(0)};`).join("");
}

function guardLine(line: string): string {
  const escaped = line.replace(/^([#>+=|-])/, "\\$1").replace(/^(\d+)([.)])/, "$1\\$2");
  let start = 0;
  while (isSpaceOrTab(escaped[start])) start++;
  let end = escaped.length;
  while (end > start && isSpaceOrTab(escaped[end - 1])) end--;
  return (
    entities(escaped.slice(0, start)) +
    escaped.slice(start, end) +
    entities(escaped.slice(end))
  );
}

function paragraph(content: readonly Inline[]): string {
  return inlineMarkdown(content, "\\\n").split("\\\n").map(guardLine).join("\\\n");
}

function cellText(cell: Cell): string {
  return cell.content
    .flatMap((block) =>
      block.type === "paragraph" || block.type === "heading"
        ? [inlineMarkdown(block.content, " ")]
        : [],
    )
    .join(" ")
    .trim();
}

function placeCell(grid: string[][], row: number, column: number, cell: Cell) {
  for (let down = 0; down < cell.rowSpan; down++) {
    const target = grid[row + down];
    if (!target) continue;
    for (let across = 0; across < cell.colSpan; across++)
      target[column + across] = down === 0 && across === 0 ? cellText(cell) : "";
  }
}

function table(rows: readonly (readonly Cell[])[]): string {
  const grid: string[][] = rows.map(() => []);
  rows.forEach((row, rowIndex) => {
    let column = 0;
    for (const cell of row) {
      while (grid[rowIndex]![column] !== undefined) column++;
      placeCell(grid, rowIndex, column, cell);
      column += cell.colSpan;
    }
  });
  const width = Math.max(1, ...grid.map((row) => row.length));
  const line = (row: string[]) =>
    `| ${Array.from({ length: width }, (_, index) => row[index] ?? "").join(" | ")} |`;
  return [
    line(grid[0] ?? []),
    `| ${Array.from({ length: width }, () => "---").join(" | ")} |`,
    ...grid.slice(1).map(line),
  ].join("\n");
}

function indent(text: string, by: number): string {
  return text
    .split("\n")
    .map((line, index) => (index === 0 || line === "" ? line : " ".repeat(by) + line))
    .join("\n");
}

function blocks(content: readonly Block[]): string {
  let previousList: { ordered: boolean; alternate: boolean } | null = null;
  return content
    .map((block) => {
      if (block.type !== "list") {
        previousList = null;
        return blockMarkdown(block);
      }
      const alternate =
        previousList !== null &&
        previousList.ordered === block.ordered &&
        !previousList.alternate;
      previousList = { ordered: block.ordered, alternate };
      return list(block, alternate);
    })
    .join("\n\n");
}

function listMarker(
  block: Extract<Block, { type: "list" }>,
  index: number,
  alternate: boolean,
): string {
  if (!block.ordered) return alternate ? "* " : "- ";
  return `${block.start + index}${alternate ? ")" : "."} `;
}

function list(block: Extract<Block, { type: "list" }>, alternate: boolean): string {
  return block.items
    .map((item, index) => {
      const marker = listMarker(block, index, alternate);
      return marker + indent(blocks(item), marker.length);
    })
    .join("\n");
}

function blockMarkdown(block: Exclude<Block, { type: "list" }>): string {
  switch (block.type) {
    case "paragraph":
      return paragraph(block.content);
    case "heading":
      return `${"#".repeat(block.level)} ${inlineMarkdown(block.content, " ").trim().replace(/#$/, "\\#")}`;
    case "table":
      return table(block.rows);
  }
}

/**
 * questionContentToMarkdown writes rich question content as the Markdown the
 * student reader renders: text, bold, italic, strikethrough, headings, lists,
 * tables and links are kept; underline, superscript and subscript are dropped;
 * a table's merged cells are written out across the cells they covered, its
 * first row becomes its header, and a cell keeps the text of its paragraphs
 * and headings on one line, dropping a list or a table nested in it.
 */
export function questionContentToMarkdown(content: QuestionContent): string {
  return blocks(content.blocks);
}
