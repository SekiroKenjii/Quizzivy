import { act, renderHook, waitFor } from "@testing-library/react";
import { Editor } from "@tiptap/core";
import { contentExtensions } from "@/components/shared/content/editor/extensions";
import {
  fromEditorJSON,
  toEditorJSON,
} from "@/components/shared/content/editor/adapter";
import { pasteTransaction } from "@/components/shared/content/editor/pasteTransaction";
import { useContentPaste } from "@/components/shared/content/editor/useContentPaste";
import { contentPlainText } from "@/components/shared/content/plainText";
import type { SemanticContent } from "@/components/shared/content/model";
import type { EditorProfile } from "@/components/shared/content/editor/profile";

const editors: Editor[] = [];
function prose(text: string): SemanticContent {
  return {
    format: "semantic_v1",
    blocks: [{ type: "paragraph", content: [{ type: "text", text, marks: [] }] }],
  };
}
function createEditor(profile: EditorProfile = "question", text = "ABCDEF"): Editor {
  const editor = new Editor({
    element: document.createElement("div"),
    extensions: contentExtensions(undefined, profile),
    content: toEditorJSON(prose(text)),
  });
  editors.push(editor);
  return editor;
}
afterEach(() => {
  for (const editor of editors.splice(0)) editor.destroy();
});

test("replaces only the selected text, retains marks and separates both undo boundaries", () => {
  const editor = createEditor();
  editor.commands.setTextSelection(7);
  editor.commands.insertContent("z");
  editor.commands.setTextSelection({ from: 3, to: 5 });
  const candidate: SemanticContent = {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [{ type: "text", text: "X", marks: ["underline"] }],
      },
    ],
  };
  const transaction = pasteTransaction(editor.state, candidate, "question");
  expect(transaction).not.toBeNull();
  editor.view.dispatch(transaction!);
  expect(editor.getText()).toBe("ABXEFz");
  expect(editor.getHTML()).toContain("<u>X</u>");
  editor.commands.insertContent("Y");
  expect(editor.commands.undo()).toBe(true);
  expect(editor.getText()).toBe("ABXEFz");
  expect(editor.commands.undo()).toBe(true);
  expect(editor.getText()).toBe("ABCDEFz");
  expect(editor.commands.redo()).toBe(true);
  expect(editor.getText()).toBe("ABXEFz");
});

test("rejects option profile mismatch and aggregate limits without dispatching", () => {
  const editor = createEditor("option");
  const candidate = prose("one");
  candidate.blocks.push(...prose("two").blocks);
  expect(pasteTransaction(editor.state, candidate, "option")).toBeNull();
  expect(editor.getText()).toBe("ABCDEF");
  const full = createEditor("question", "a".repeat(100_000));
  expect(pasteTransaction(full.state, prose("b"), "question")).toBeNull();
  expect(full.getText()).toHaveLength(100_000);
});

test("previews the entire resulting document and refuses a stale confirmation", async () => {
  const editor = createEditor();
  editor.commands.setTextSelection({ from: 3, to: 5 });
  const notify = vi.fn();
  const { result } = renderHook(() => useContentPaste("question", notify));
  act(() => result.current.previewPaste(editor.view, "<p><u>X</u></p>"));
  await waitFor(() => expect(result.current.paste?.result).not.toBeNull());
  expect(contentPlainText(result.current.paste!.result!)).toBe("ABXEF");
  expect(editor.getText()).toBe("ABCDEF");
  act(() => {
    editor.commands.insertContent("changed");
    result.current.applyPaste();
  });
  expect(editor.getText()).toBe("ABchangedEF");
  expect(notify).toHaveBeenLastCalledWith("pasteStale");
  expect(result.current.paste).toBeNull();
});

test("cancelling an unresolved conversion never reopens the dialog or changes the document", async () => {
  const editor = createEditor();
  const { result } = renderHook(() => useContentPaste("question", vi.fn()));
  await act(async () => {
    result.current.previewPaste(editor.view, "<p>Cancelled</p>");
    result.current.closePaste();
    await Promise.resolve();
  });
  expect(result.current.paste).toBeNull();
  const parsed = fromEditorJSON(editor.getJSON());
  expect(parsed.ok && contentPlainText(parsed.value)).toBe("ABCDEF");
});
