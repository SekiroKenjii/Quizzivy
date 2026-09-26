import assert from "node:assert/strict";
import { createRequire } from "node:module";
import path from "node:path";
import process from "node:process";
const require = createRequire(path.join(path.resolve(process.argv[2]), "package.json"));
const { createHeadlessEditor } = require("@lexical/headless");
const {
  TextNode,
  $getRoot,
  $createParagraphNode,
  $createTextNode,
} = require("lexical");
const { HeadingNode, $createHeadingNode } = require("@lexical/rich-text");
const {
  ListNode,
  ListItemNode,
  $createListNode,
  $createListItemNode,
} = require("@lexical/list");
const {
  TableNode,
  TableRowNode,
  TableCellNode,
  $createTableNode,
  $createTableRowNode,
  $createTableCellNode,
} = require("@lexical/table");
import { formattingSample, tableSample } from "./fixtures.ts";
class GapNode extends TextNode {
  constructor(text, gapId, key) {
    super(text, key);
    this.gapId = gapId;
  }
  static getType() {
    return "gap";
  }
  static clone(node) {
    return new GapNode(node.__text, node.gapId, node.__key);
  }
  static importJSON(value) {
    const node = new GapNode(value.text, value.gapId);
    node.updateFromJSON(value);
    return node;
  }
  exportJSON() {
    return { ...super.exportJSON(), type: "gap", version: 1, gapId: this.gapId };
  }
}
function inline(node) {
  if (node.type === "gap") return new GapNode(node.label, node.id).setMode("token");
  assert.equal(node.type, "text");
  const text = $createTextNode(node.text);
  for (const mark of node.marks)
    text.toggleFormat(mark === "strike" ? "strikethrough" : mark);
  return text;
}
function block(node) {
  if (node.type === "paragraph" || node.type === "heading")
    return (
      node.type === "paragraph"
        ? $createParagraphNode()
        : $createHeadingNode(`h${node.level}`)
    ).append(...node.content.map(inline));
  if (node.type === "list")
    return $createListNode(node.ordered ? "number" : "bullet", node.start).append(
      ...node.items.map((item) => $createListItemNode().append(...item.map(block))),
    );
  assert.equal(node.type, "table");
  return $createTableNode().append(
    ...node.rows.map((row) =>
      $createTableRowNode().append(
        ...row.map((cell) =>
          $createTableCellNode(cell.header ? 1 : 0, cell.colSpan)
            .setRowSpan(cell.rowSpan)
            .append(...cell.content.map(block)),
        ),
      ),
    ),
  );
}
const nodes = [
  HeadingNode,
  ListNode,
  ListItemNode,
  TableNode,
  TableRowNode,
  TableCellNode,
  GapNode,
];
for (const [name, sample] of Object.entries({ formattingSample, tableSample })) {
  const editor = createHeadlessEditor({
    nodes,
    onError: (error) => {
      throw error;
    },
  });
  editor.update(() => $getRoot().append(...sample.blocks.map(block)), {
    discrete: true,
  });
  const state = editor.getEditorState().toJSON();
  const serialized = JSON.stringify(state);
  editor.setEditorState(editor.parseEditorState(serialized));
  assert.deepEqual(editor.getEditorState().toJSON(), state);
  if (name === "formattingSample") {
    assert.match(serialized, /gap-synthetic-1/);
    assert.match(serialized, /Tiếng Việt/);
    const paragraph = state.root.children[1];
    assert.equal(paragraph.children[1].format, 11);
    assert.equal(paragraph.children[3].format, 32);
    assert.equal(paragraph.children[5].format, 64);
  } else {
    const table = state.root.children[1];
    assert.equal(table.children[0].children[0].colSpan, 2);
    assert.equal(table.children[1].children[0].rowSpan, 2);
    assert.equal(state.root.children[2].start, 3);
  }
  console.log(
    `${name}: Lexical 0.51.0 JSON reload preserves tested marks, spans, list start and custom gap ID`,
  );
}
