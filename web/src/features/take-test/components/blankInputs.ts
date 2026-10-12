const PLACEHOLDER = /\{\{(\d+)\}\}/g;

interface TextNode {
  type: "text";
  value: string;
}

interface ElementNode {
  type: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
}

type HastNode = TextNode | ElementNode;

/**
 * blankInputs is a rehype plugin that turns each `{{n}}` in a rendered prompt
 * into an empty span carrying `data-blank="n"`, the slot the student's input
 * is mounted into, and `data-in-table` when the slot stands in a table.
 */
export function blankInputs() {
  return (tree: ElementNode) => splice(tree, false);
}

function splice(node: ElementNode, inTable: boolean): void {
  if (!node.children) return;

  const next: HastNode[] = [];
  for (const child of node.children) {
    if (isText(child)) {
      next.push(...tokenize(child.value, inTable));
      continue;
    }
    splice(child, inTable || child.tagName === "table");
    next.push(child);
  }
  node.children = next;
}

function tokenize(value: string, inTable: boolean): HastNode[] {
  const nodes: HastNode[] = [];
  let cursor = 0;

  for (const match of value.matchAll(PLACEHOLDER)) {
    const at = match.index;
    if (at > cursor) nodes.push({ type: "text", value: value.slice(cursor, at) });
    nodes.push(slot(match[1] ?? "", inTable));
    cursor = at + match[0].length;
  }

  if (cursor === 0) return [{ type: "text", value }];
  if (cursor < value.length) nodes.push({ type: "text", value: value.slice(cursor) });
  return nodes;
}

function slot(ordinal: string, inTable: boolean): ElementNode {
  return {
    type: "element",
    tagName: "span",
    properties: inTable
      ? { "data-blank": ordinal, "data-in-table": "true" }
      : { "data-blank": ordinal },
    children: [],
  };
}

function isText(node: HastNode): node is TextNode {
  return node.type === "text";
}
