import { Editor } from "@tiptap/core";
import {
  fromEditorJSON,
  toEditorJSON,
} from "@/components/shared/content/editor/adapter";
import { contentExtensions } from "@/components/shared/content/editor/extensions";
import { formattingSample, tableSample } from "@tests/support/content-editor/fixtures";
import type { SemanticContent } from "@/components/shared/content/model";

test("preserves supported links, line breaks and inert media references", () => {
  const document: SemanticContent = {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [
          {
            type: "link",
            href: "https://example.com/reading",
            content: [{ type: "text", text: "Đọc thêm", marks: ["underline"] }],
          },
          { type: "break" },
        ],
      },
      {
        type: "image",
        assetId: "00000000-0000-4000-8000-000000000001",
        alt: "Biểu đồ mẫu",
      },
      {
        type: "audio",
        assetId: "00000000-0000-4000-8000-000000000002",
        label: "Bài nghe mẫu",
      },
    ],
  };
  const editor = new Editor({
    extensions: contentExtensions(),
    content: toEditorJSON(document),
    enableContentCheck: true,
  });
  try {
    expect(fromEditorJSON(editor.getJSON())).toEqual({ ok: true, value: document });
    expect(editor.getHTML()).not.toMatch(/<(img|audio|iframe)/);
  } finally {
    editor.destroy();
  }
});

test.each([formattingSample, tableSample])(
  "round trips the content through the real editor schema",
  (document) => {
    const editor = new Editor({
      extensions: contentExtensions(),
      content: toEditorJSON(document),
      enableContentCheck: true,
    });
    try {
      expect(fromEditorJSON(editor.getJSON())).toEqual({ ok: true, value: document });
      editor.commands.insertContentAt(1, "Thêm tiếng Việt. ");
      expect(fromEditorJSON(editor.getJSON()).ok).toBe(true);
      expect(editor.getText()).toContain("Thêm tiếng Việt.");
      expect(editor.commands.undo()).toBe(true);
      expect(fromEditorJSON(editor.getJSON())).toEqual({ ok: true, value: document });
    } finally {
      editor.destroy();
    }
  },
);

test("rejects unknown editor marks/attributes rather than silently stripping them", () => {
  const json = toEditorJSON(formattingSample);
  expect(fromEditorJSON({ ...json, attrs: { answer: "private" } }).ok).toBe(false);
  expect(
    fromEditorJSON({
      type: "doc",
      content: [
        {
          type: "paragraph",
          content: [{ type: "text", text: "x", marks: [{ type: "code" }] }],
        },
      ],
    }).ok,
  ).toBe(false);
});

test("prevents a duplicate stable gap from changing the document", () => {
  const editor = new Editor({
    extensions: contentExtensions(),
    content: toEditorJSON(formattingSample),
  });
  try {
    const before = editor.getJSON();
    editor.commands.insertContentAt(1, {
      type: "gap",
      attrs: { id: "gap-synthetic-1", label: "2" },
    });
    expect(editor.getJSON()).toEqual(before);
  } finally {
    editor.destroy();
  }
});

test("accepts shared default table attributes created by row insertion", () => {
  const editor = new Editor({
    extensions: contentExtensions(),
    content: toEditorJSON(tableSample),
  });
  try {
    let cellPosition = 0;
    editor.state.doc.descendants((node, position) => {
      if (!cellPosition && node.type.name === "tableHeader")
        cellPosition = position + 2;
    });
    editor.commands.setTextSelection(cellPosition);
    expect(editor.commands.addRowAfter()).toBe(true);
    const document = fromEditorJSON(editor.getJSON());
    expect(document.ok).toBe(true);
    expect(editor.getHTML().match(/<tr>/g)).toHaveLength(4);
  } finally {
    editor.destroy();
  }
});
