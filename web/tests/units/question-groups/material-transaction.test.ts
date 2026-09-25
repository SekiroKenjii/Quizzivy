import { Editor } from "@tiptap/core";
import { contentExtensions } from "@/components/shared/content/editor/extensions";
import {
  toEditorJSON,
  fromEditorJSON,
} from "@/components/shared/content/editor/adapter";
import { materialTransaction } from "@/features/question-groups/materialTransaction";
import type { MediaAsset } from "@/features/media/api";

const editors: Editor[] = [];
afterEach(() => {
  for (const editor of editors.splice(0)) editor.destroy();
});
function editor(text = "Passage") {
  const instance = new Editor({
    element: document.createElement("div"),
    extensions: contentExtensions(),
    content: toEditorJSON({
      format: "semantic_v1",
      blocks: [{ type: "paragraph", content: [{ type: "text", text, marks: [] }] }],
    }),
  });
  editors.push(instance);
  return instance;
}
const asset = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9f",
  kind: "image",
  url: "https://assets.example/image.png",
} as MediaAsset;
test("asset insertion uses a stable authorized ID, validates the complete document and supports undo", () => {
  const current = editor();
  current.commands.setTextSelection(8);
  const transaction = materialTransaction(
    current.state,
    current.state.selection.getBookmark(),
    asset,
    "Diagram",
  );
  expect(transaction).not.toBeNull();
  current.view.dispatch(transaction!);
  const result = fromEditorJSON(current.getJSON());
  expect(
    result.ok &&
      result.value.format === "semantic_v1" &&
      result.value.blocks.some(
        (block) => block.type === "image" && block.assetId === asset.id,
      ),
  ).toBe(true);
  expect(JSON.stringify(current.getJSON())).not.toContain(asset.url);
  expect(current.commands.undo()).toBe(true);
  expect(current.getText()).toBe("Passage");
});
test("invalid asset and aggregate content limits do not dispatch partial edits", () => {
  const current = editor("a".repeat(100_000));
  expect(
    materialTransaction(
      current.state,
      current.state.selection.getBookmark(),
      asset,
      "Diagram",
    ),
  ).toBeNull();
  const normal = editor();
  expect(
    materialTransaction(
      normal.state,
      normal.state.selection.getBookmark(),
      { ...asset, id: "not-an-id" },
      "Diagram",
    ),
  ).toBeNull();
  expect(normal.getText()).toBe("Passage");
});
