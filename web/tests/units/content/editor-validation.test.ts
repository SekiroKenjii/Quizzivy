import { afterEach, expect, it, vi } from "vitest";
import { Editor } from "@tiptap/core";
import { contentExtensions } from "@/components/shared/content/editor/extensions";
import {
  fromEditorDoc,
  toEditorJSON,
} from "@/components/shared/content/editor/adapter";

let editor: Editor | undefined;
afterEach(() => {
  editor?.destroy();
  vi.restoreAllMocks();
});

it("validates each editor document once, however many checks read it", () => {
  editor = new Editor({
    element: document.createElement("div"),
    extensions: contentExtensions(undefined, "question"),
    content: toEditorJSON({
      format: "semantic_v1",
      blocks: [
        { type: "paragraph", content: [{ type: "text", text: "Hi", marks: [] }] },
      ],
    }),
  });
  editor.commands.insertContent("!");
  const doc = editor.state.doc;
  const serialize = vi.spyOn(doc, "toJSON");

  const first = fromEditorDoc(doc);
  const second = fromEditorDoc(doc);

  expect(first.ok).toBe(true);
  expect(second).toBe(first);
  expect(serialize).not.toHaveBeenCalled();
});
