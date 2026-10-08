import "@/lib/i18n";
import { createElement } from "react";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { Editor } from "@tiptap/core";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { CLIPBOARD_HTML_LIMIT } from "@/components/shared/content/editor/clipboardLimits";
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

function clipboard(data: Record<string, string>, files: File[] = []) {
  return {
    clipboardData: {
      files,
      types: Object.keys(data),
      getData: (type: string) => data[type] ?? "",
    },
  };
}

const IMAGE = new File(["x"], "anh.png", { type: "image/png" });

test("previews a paste without its images and counts what was left out", async () => {
  const editor = createEditor();
  const { result } = renderHook(() => useContentPaste("question", vi.fn()));
  act(() =>
    result.current.previewPaste(
      editor.view,
      '<p>Chọn<img src="https://example.test/a.png"> đáp án<svg><circle r="1"/></svg></p>',
    ),
  );
  await waitFor(() => expect(result.current.paste?.result).not.toBeNull());
  expect(result.current.paste?.imagesLeftOut).toBe(2);
  expect(contentPlainText(result.current.paste!.content!)).toBe("Chọn đáp án");
});

test.each([
  ['<p>&nbsp;<img src="https://example.test/a.png"></p>', "file"],
  ["<p>A<del>B</del></p>", "pasteBlocked"],
])(
  "closes the preview and reports a paste it cannot take: %s",
  async (html, notice) => {
    const editor = createEditor();
    const notify = vi.fn();
    const { result } = renderHook(() => useContentPaste("question", notify));
    act(() => result.current.previewPaste(editor.view, html));
    await waitFor(() => expect(notify).toHaveBeenLastCalledWith(notice));
    expect(result.current.paste).toBeNull();
    expect(editor.getText()).toBe("ABCDEF");
  },
);

test("a paste over the limit opens no preview", () => {
  const editor = createEditor();
  const notify = vi.fn();
  const { result } = renderHook(() => useContentPaste("question", notify));
  act(() =>
    result.current.previewPaste(
      editor.view,
      `<p>${"a".repeat(CLIPBOARD_HTML_LIMIT)}</p>`,
    ),
  );
  expect(result.current.paste).toBeNull();
  expect(notify).toHaveBeenCalledWith("pasteBlocked");
});

describe("what the editor does with a paste or a drop beside a file", () => {
  function pasteInto(
    profile: EditorProfile,
    data: Record<string, string>,
    files: File[] = [],
  ) {
    const notify = vi.fn();
    const preview = vi.fn();
    const editor = new Editor({
      element: document.createElement("div"),
      extensions: contentExtensions(notify, profile, preview),
      content: toEditorJSON(prose("ABC")),
    });
    editors.push(editor);
    editor.commands.setTextSelection(4);
    fireEvent.paste(editor.view.dom, clipboard(data, files));
    return { editor, notify, preview };
  }

  test("a file alone gets the file notice and changes nothing", () => {
    const { editor, notify, preview } = pasteInto("question", {}, [IMAGE]);
    expect(notify).toHaveBeenCalledWith("file");
    expect(preview).not.toHaveBeenCalled();
    expect(editor.getText()).toBe("ABC");
  });

  test("a copied image whose HTML shows no text gets the file notice", () => {
    const { notify, preview } = pasteInto("prompt", {
      "text/html": '<meta charset="utf-8"><img src="https://example.test/a.png">',
    });
    expect(notify).toHaveBeenCalledWith("file");
    expect(preview).not.toHaveBeenCalled();
  });

  test("HTML with text beside a file is previewed", () => {
    const { notify, preview } = pasteInto(
      "question",
      { "text/html": "<p>Đoạn <b>có chữ</b></p>", "text/plain": "Đoạn có chữ" },
      [IMAGE],
    );
    expect(preview).toHaveBeenCalledWith(
      expect.anything(),
      "<p>Đoạn <b>có chữ</b></p>",
    );
    expect(notify).not.toHaveBeenCalled();
  });

  test.each(["question", "option"] as const)(
    "plain text beside a file is inserted in the %s profile",
    (profile) => {
      const { editor, notify, preview } = pasteInto(
        profile,
        { "text/plain": " thêm" },
        [IMAGE],
      );
      expect(editor.getText()).toBe("ABC thêm");
      expect(notify).not.toHaveBeenCalled();
      expect(preview).not.toHaveBeenCalled();
    },
  );

  test("a dropped file gets the file notice", () => {
    const notify = vi.fn();
    const editor = new Editor({
      element: document.createElement("div"),
      extensions: contentExtensions(notify, "question"),
      content: toEditorJSON(prose("ABC")),
    });
    editors.push(editor);
    fireEvent.drop(editor.view.dom, {
      dataTransfer: { files: [IMAGE], types: ["Files"] },
    });
    expect(notify).toHaveBeenCalledWith("file");
    expect(editor.getText()).toBe("ABC");
  });
});

describe("the notice band", () => {
  beforeAll(() => {
    Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
    Range.prototype.getBoundingClientRect ??= () => new DOMRect();
  });

  function renderEditor(fileNotice?: string) {
    let current: Editor | undefined;
    render(
      createElement(ContentEditor, {
        initialContent: prose("Đề bài"),
        label: "Đề bài",
        profile: "question",
        fileNotice,
        onChange: () => undefined,
        tools: (editor: Editor) => {
          current = editor;
          return null;
        },
      }),
    );
    return {
      textbox: screen.getByRole("textbox", { name: "Đề bài" }),
      editor: () => current!,
    };
  }

  test("carries the host's file notice, is dismissed, and is cleared by the next edit", async () => {
    const { textbox, editor } = renderEditor(
      "Thêm âm thanh hoặc hình ảnh ở mục Phương tiện.",
    );
    fireEvent.paste(textbox, clipboard({}, [IMAGE]));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Thêm âm thanh hoặc hình ảnh ở mục Phương tiện.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Đóng thông báo" }));
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.paste(textbox, clipboard({}, [IMAGE]));
    expect(screen.getByRole("alert")).toBeVisible();
    act(() => {
      editor().commands.insertContent(" mới");
    });
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  test("says the default file notice and opens no dialog for an image alone", () => {
    const { textbox } = renderEditor();
    fireEvent.paste(
      textbox,
      clipboard({ "text/html": '<img src="https://example.test/a.png">' }),
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Không thể thêm tệp vào phần văn bản. Nội dung hiện tại được giữ nguyên.",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  test("reports a refused conversion in the band once the dialog closes", async () => {
    const { textbox } = renderEditor();
    fireEvent.paste(textbox, clipboard({ "text/html": "<p>A<del>B</del></p>" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Không thể chuyển đổi nội dung dán: nội dung có định dạng chưa được hỗ trợ hoặc vượt quá giới hạn.",
      ),
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  test("the preview says how many copied images were left out", async () => {
    const { textbox } = renderEditor();
    fireEvent.paste(
      textbox,
      clipboard({
        "text/html":
          '<p>Chữ<img src="https://example.test/a.png"><img src="https://example.test/b.png"></p>',
      }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Xem trước nội dung sau khi dán",
    });
    expect(
      await within(dialog).findByText("Đã bỏ 2 hình ảnh trong nội dung sao chép."),
    ).toBeVisible();
    expect(dialog.querySelector("img")).toBeNull();
  });
});
