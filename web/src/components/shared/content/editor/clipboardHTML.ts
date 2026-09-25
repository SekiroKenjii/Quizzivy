import { parse, type DefaultTreeAdapterTypes as Tree } from "parse5";
import type {
  ContentBlock,
  ContentInline,
  ContentMark,
  SemanticContent,
} from "../model";
import { validateContent, safeContentURL } from "../validation";
import { contentStringLength } from "../unicode";
import { clipboardStyles } from "./clipboardStyles";
import { CLIPBOARD_HTML_LIMIT } from "./clipboardLimits";

const INLINE = new Set([
  "span",
  "b",
  "strong",
  "i",
  "em",
  "u",
  "s",
  "strike",
  "sub",
  "sup",
  "a",
  "br",
  "font",
  "o:p",
]);
const TAGS = new Set([
  ...INLINE,
  "html",
  "head",
  "body",
  "meta",
  "div",
  "p",
  "h1",
  "h2",
  "h3",
  "ul",
  "ol",
  "li",
  "table",
  "thead",
  "tbody",
  "tfoot",
  "tr",
  "td",
  "th",
  "colgroup",
  "col",
]);
const ATTRS = new Set([
  "style",
  "class",
  "id",
  "lang",
  "title",
  "width",
  "height",
  "align",
  "valign",
  "border",
  "cellpadding",
  "cellspacing",
  "face",
  "size",
  "color",
]);
const MARKS: Partial<Record<string, ContentMark>> = {
  b: "bold",
  strong: "bold",
  i: "italic",
  em: "italic",
  u: "underline",
  s: "strike",
  strike: "strike",
  sub: "subscript",
  sup: "superscript",
};
const EMPTY: ContentBlock = { type: "paragraph", content: [] };

/** clipboardHTML converts inert, bounded HTML into supported prose; null means no partial result may be inserted. */
export function clipboardHTML(html: string): SemanticContent | null {
  if (
    html.length > CLIPBOARD_HTML_LIMIT ||
    contentStringLength(html) > CLIPBOARD_HTML_LIMIT ||
    new TextEncoder().encode(html).byteLength > CLIPBOARD_HTML_LIMIT
  )
    return null;
  try {
    const document = parse(html, {
      sourceCodeLocationInfo: true,
      onParseError: (error) => {
        if (error.code !== "missing-doctype") throw new Error("html");
      },
    });
    inspect(document, html);
    const root = document.childNodes.find(isElement);
    const body = root?.childNodes.find(
      (node) => isElement(node) && node.tagName === "body",
    );
    if (!root || !body || !isElement(body)) return null;
    const blocks = readBlocks(body.childNodes, marksFor(body, marksFor(root, [])));
    if (!blocks.length) return null;
    const result = validateContent({ format: "semantic_v1", blocks });
    return result.ok && result.value.format === "semantic_v1" ? result.value : null;
  } catch {
    return null;
  }
}

function isElement(node: Tree.Node): node is Tree.Element {
  return "tagName" in node;
}

function inspect(document: Tree.Document, html: string) {
  const coverage = new Uint8Array(html.length);
  const pending = [{ node: document as Tree.Node, depth: 0 }];
  let nodes = 0;
  while (pending.length) {
    const { node, depth } = pending.pop()!;
    if (++nodes > 6144 || depth > 48) throw new Error("budget");
    if (
      node.nodeName === "#comment" &&
      "data" in node &&
      /\[if|\[endif/i.test(node.data)
    )
      throw new Error("conditional");
    coverNode(node, coverage);
    if ("childNodes" in node)
      for (const child of node.childNodes)
        pending.push({ node: child, depth: depth + 1 });
  }
  for (let i = 0; i < html.length; i++)
    if (!coverage[i] && !/\s/.test(html[i]!)) throw new Error("discarded");
}

function coverNode(node: Tree.Node, coverage: Uint8Array) {
  const location = node.sourceCodeLocation;
  if (isElement(node)) {
    checkElement(node);
    for (const tag of [
      node.sourceCodeLocation?.startTag,
      node.sourceCodeLocation?.endTag,
    ])
      if (tag) coverage.fill(1, tag.startOffset, tag.endOffset);
  } else if (location) coverage.fill(1, location.startOffset, location.endOffset);
}

function checkElement(node: Tree.Element) {
  if (!TAGS.has(node.tagName) || node.namespaceURI !== "http://www.w3.org/1999/xhtml")
    throw new Error("element");
  for (const attr of node.attrs) {
    const allowed =
      ATTRS.has(attr.name) ||
      (node.tagName === "a" && ["href", "target", "rel", "name"].includes(attr.name)) ||
      (["td", "th"].includes(node.tagName) &&
        ["rowspan", "colspan", "scope"].includes(attr.name)) ||
      (node.tagName === "ol" && attr.name === "start") ||
      (["col", "colgroup"].includes(node.tagName) && attr.name === "span") ||
      (node.tagName === "meta" && attr.name === "charset");
    if (!allowed || attr.namespace || attr.prefix) throw new Error("attribute");
  }
  marksFor(node, []);
}

function attribute(node: Tree.Element, name: string): string | undefined {
  return node.attrs.find((attr) => attr.name === name)?.value;
}

function marksFor(node: Tree.Element, inherited: ContentMark[]): ContentMark[] {
  const mark = MARKS[node.tagName];
  return clipboardStyles(attribute(node, "style") ?? "", inherited, mark ? [mark] : []);
}

function readInline(node: Tree.ChildNode, marks: ContentMark[]): ContentInline[] {
  if (node.nodeName === "#comment") return [];
  if ("value" in node) {
    const text = node.value.replace(/[\t\r\n\f ]+/g, " ");
    return text ? [{ type: "text", text, marks }] : [];
  }
  if (!isElement(node) || !INLINE.has(node.tagName)) throw new Error("inline");
  if (node.tagName === "br") return [{ type: "break" }];
  const content = node.childNodes.flatMap((child) =>
    readInline(child, marksFor(node, marks)),
  );
  if (node.tagName !== "a") return content;
  const href = attribute(node, "href");
  if (!href) return content;
  if (!safeContentURL(href) || !content.every((child) => child.type === "text"))
    throw new Error("link");
  return content.length ? [{ type: "link", href, content }] : [];
}

function trimInline(content: ContentInline[]): ContentInline[] {
  let precedingSpace = true;
  const result: ContentInline[] = [];
  for (const node of content) {
    if (node.type === "text") {
      const text: string = precedingSpace ? node.text.replace(/^ +/, "") : node.text;
      if (text) result.push({ ...node, text });
      precedingSpace = !text || text.endsWith(" ");
    } else {
      result.push(node);
      precedingSpace = node.type === "break";
    }
  }
  const last = result.at(-1);
  if (last?.type === "text") {
    let end = last.text.length;
    while (last.text[end - 1] === " ") end--;
    last.text = last.text.slice(0, end);
    if (!last.text) result.pop();
  }
  return result;
}

function readBlocks(nodes: Tree.ChildNode[], marks: ContentMark[]): ContentBlock[] {
  const result: ContentBlock[] = [];
  let inline: ContentInline[] = [];
  const flush = () => {
    const content = trimInline(inline);
    if (content.length) result.push({ type: "paragraph", content });
    inline = [];
  };
  for (const node of nodes) {
    if (!isElement(node) || INLINE.has(node.tagName))
      inline.push(...readInline(node, marks));
    else {
      flush();
      result.push(...readBlock(node, marks));
    }
  }
  flush();
  return result;
}

function readBlock(node: Tree.Element, inherited: ContentMark[]): ContentBlock[] {
  const marks = marksFor(node, inherited);
  switch (node.tagName) {
    case "div":
      return readBlocks(node.childNodes, marks);
    case "p":
      return [
        {
          type: "paragraph",
          content: trimInline(
            node.childNodes.flatMap((child) => readInline(child, marks)),
          ),
        },
      ];
    case "h1":
    case "h2":
    case "h3":
      return [
        {
          type: "heading",
          level: Number(node.tagName[1]) as 1 | 2 | 3,
          content: trimInline(
            node.childNodes.flatMap((child) => readInline(child, marks)),
          ),
        },
      ];
    case "ul":
    case "ol":
      return [
        {
          type: "list",
          ordered: node.tagName === "ol",
          start: integerAttribute(node, "start"),
          items: children(node).map((item) => {
            if (item.tagName !== "li") throw new Error("list");
            const blocks = readBlocks(item.childNodes, marksFor(item, marks));
            return blocks[0]?.type === "paragraph" ? blocks : [EMPTY, ...blocks];
          }),
        },
      ];
    case "table":
      return [
        {
          type: "table",
          rows: tableRows(node, marks).map(({ row, inherited }) =>
            children(row).map((cell) => {
              if (!["td", "th"].includes(cell.tagName)) throw new Error("cell");
              const content = readBlocks(
                cell.childNodes,
                marksFor(cell, marksFor(row, inherited)),
              );
              return {
                header: cell.tagName === "th",
                rowSpan: integerAttribute(cell, "rowspan"),
                colSpan: integerAttribute(cell, "colspan"),
                content: content.length ? content : [EMPTY],
              };
            }),
          ),
        },
      ];
    default:
      throw new Error("block");
  }
}

function integerAttribute(node: Tree.Element, name: string): number {
  const value = attribute(node, name) ?? "1";
  if (!/^[1-9]\d{0,4}$/.test(value)) throw new Error("number");
  return Number(value);
}

function children(node: Tree.Element): Tree.Element[] {
  return node.childNodes.filter((child): child is Tree.Element => {
    if (isElement(child)) return true;
    if (child.nodeName !== "#comment" && !("value" in child && !child.value.trim()))
      throw new Error("structure");
    return false;
  });
}

function tableRows(
  node: Tree.Element,
  inherited: ContentMark[],
): { row: Tree.Element; inherited: ContentMark[] }[] {
  return children(node).flatMap((child) => {
    if (child.tagName === "tr") return [{ row: child, inherited }];
    if (["thead", "tbody", "tfoot"].includes(child.tagName)) {
      const rows = children(child);
      if (rows.some((row) => row.tagName !== "tr")) throw new Error("row");
      return rows.map((row) => ({ row, inherited: marksFor(child, inherited) }));
    }
    if (
      child.tagName === "colgroup" &&
      children(child).every(
        (col) => col.tagName === "col" && !marksFor(col, []).length,
      ) &&
      !marksFor(child, []).length
    )
      return [];
    throw new Error("table");
  });
}
