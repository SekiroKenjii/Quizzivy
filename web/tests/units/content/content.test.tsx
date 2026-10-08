import "@/lib/i18n";
import type { ComponentProps } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Editor } from "@tiptap/react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { toEditorJSON } from "@/components/shared/content/editor/adapter";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import type { SemanticContent } from "@/components/shared/content/model";
import { ContentView } from "@/components/shared/content/ContentView";
import {
  validateContent,
  safeContentURL,
} from "@/components/shared/content/validation";
import { contentPlainText } from "@/components/shared/content/plainText";
import { CONTENT_LIMITS } from "@/components/shared/content/model";
import {
  formattingSample,
  tableSample,
  paragraph,
} from "@tests/support/content-editor/fixtures";

test.each([formattingSample, tableSample])(
  "renders the supported sample without losing text",
  (document) => {
    expect(validateContent(document).ok).toBe(true);
    const { container } = render(<ContentView document={document} />);
    expect(container.textContent).not.toBe("");
    expect(contentPlainText(document)).not.toContain("assetId");
  },
);

test("retains semantic marks, gap identity and merged table layout", () => {
  const { container, rerender } = render(<ContentView document={formattingSample} />);
  expect(container.querySelector("u em strong")).toHaveTextContent(
    "nghiêng, đậm và gạch chân",
  );
  expect(container.querySelector("sub")).toHaveTextContent("2");
  expect(container.querySelector("sup")).toHaveTextContent("2");
  expect(screen.getByLabelText("Ô trống 1")).toBeVisible();
  rerender(<ContentView document={tableSample} />);
  expect(container.querySelector("th")).toHaveAttribute("colspan", "2");
  expect(screen.getByText("Thứ hai").closest("td")).toHaveAttribute("rowspan", "2");
  expect(container.querySelector("ol")).toHaveAttribute("start", "3");
});

test.each([
  "javascript:alert(1)",
  "data:text/html,x",
  "//example.com",
  "http://example.com",
  "https://user:pass@example.com",
  "https://example.com\n",
  " https://example.com",
  "https://example.com/a b",
  "https:example.com",
  "https:\\example.com",
])("rejects unsafe link %s", (value) => {
  expect(safeContentURL(value)).toBe(false);
});

test("permits HTTPS and does not interpret escaped content as HTML", () => {
  expect(safeContentURL("https://example.com/đọc?q=1#section")).toBe(true);
  const { container } = render(
    <ContentView
      document={{
        format: "semantic_v1",
        blocks: [paragraph('<img src=x onerror="alert(1)">')],
      }}
    />,
  );
  expect(container.querySelector("img")).toBeNull();
  expect(container).toHaveTextContent("<img src=x");
});

test.each([
  "isCorrect",
  "sampleAnswer",
  "acceptedAnswers",
  "transcript",
  "sourcePath",
  "onclick",
])("rejects unexpected learner field %s", (key) => {
  expect(validateContent({ ...formattingSample, [key]: "private" }).ok).toBe(false);
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [{ ...paragraph("x"), [key]: "private" }],
    }).ok,
  ).toBe(false);
});

test("rejects duplicate gaps and invalid table spans", () => {
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: [
            { type: "gap", id: "same", label: "1" },
            { type: "gap", id: "same", label: "2" },
          ],
        },
      ],
    }),
  ).toEqual({ ok: false, issue: "duplicate_gap" });
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [
        {
          type: "table",
          rows: [
            [{ header: false, rowSpan: 2, colSpan: 1, content: [paragraph("x")] }],
          ],
        },
      ],
    }),
  ).toEqual({ ok: false, issue: "table_grid" });
});

test("rejects unknown marks, nested tables and over-budget inputs", () => {
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [
        {
          type: "table",
          rows: [
            [
              {
                header: false,
                rowSpan: 1,
                colSpan: 1,
                content: [tableSample.blocks[1]],
              },
            ],
          ],
        },
      ],
    }),
  ).toEqual({ ok: false, issue: "nested_table" });
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: [{ type: "text", text: "x", marks: ["highlight"] }],
        },
      ],
    }).ok,
  ).toBe(false);
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: Array.from({ length: CONTENT_LIMITS.nodes + 1 }, () => paragraph("")),
    }).ok,
  ).toBe(false);
  expect(
    validateContent({
      format: "semantic_v1",
      blocks: [paragraph("x".repeat(CONTENT_LIMITS.text + 1))],
    }).ok,
  ).toBe(false);
  const cyclic: Record<string, unknown> = {};
  cyclic.self = cyclic;
  expect(validateContent(cyclic).ok).toBe(false);
});

test("keeps the legacy Markdown reader and projects exact legacy strings", () => {
  const document = {
    format: "legacy_markdown_v1" as const,
    markdown: "**Câu cũ**\n\nNội dung đã lưu.",
  };
  const { container } = render(<ContentView document={document} />);
  expect(container.querySelector("strong")).toHaveTextContent("Câu cũ");
  expect(contentPlainText(document)).toBe(document.markdown);
});

test("permits shared JSON values while rejecting cyclic graphs", () => {
  const shared = paragraph("Một giá trị dùng hai lần");
  expect(validateContent({ format: "semantic_v1", blocks: [shared, shared] }).ok).toBe(
    true,
  );
  const cyclic: Record<string, unknown> = {};
  cyclic.self = cyclic;
  expect(validateContent(cyclic).ok).toBe(false);
});

class StillResize {
  observe() {}
  unobserve() {}
  disconnect() {}
}

/** jsdom does no layout; ProseMirror measures a range when it scrolls the selection into view. */
Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();

function renderEditor(
  props: Partial<ComponentProps<typeof ContentEditor>> = {},
  document: SemanticContent = { format: "semantic_v1", blocks: [paragraph("")] },
) {
  let editor: Editor | undefined;
  render(
    <ContentEditor
      initialContent={document}
      label="Đề bài"
      onChange={() => undefined}
      tools={(current) => {
        editor = current;
        return null;
      }}
      {...props}
    />,
  );
  return () => editor!;
}

function toolbarNames() {
  return within(screen.getByRole("toolbar", { name: "Thanh định dạng: Đề bài" }))
    .getAllByRole("button")
    .map((button) => button.getAttribute("aria-label"));
}

function links(editor: Editor) {
  return Array.from(editor.view.dom.querySelectorAll("a")).map((link) => [
    link.getAttribute("href"),
    link.textContent,
  ]);
}

async function submitLink(value: string) {
  const address = await screen.findByRole("textbox", { name: "Địa chỉ liên kết" });
  fireEvent.change(address, { target: { value } });
  fireEvent.submit(address.closest("form")!);
}

const MARKS = [
  "In đậm",
  "In nghiêng",
  "Gạch chân",
  "Gạch ngang",
  "Chỉ số trên",
  "Chỉ số dưới",
];
const BLOCKS = [
  "Tiêu đề",
  "Đoạn văn",
  "Danh sách gạch đầu dòng",
  "Danh sách đánh số",
  "Liên kết",
  "Thêm bảng",
];
const HISTORY = ["Hoàn tác", "Làm lại"];

describe("the content editor's frame", () => {
  beforeEach(() => vi.stubGlobal("ResizeObserver", StillResize));
  afterEach(() => vi.unstubAllGlobals());

  test.each([
    ["prompt", [...MARKS, ...BLOCKS, "Thêm ô trống", ...HISTORY], true],
    ["document", [...MARKS, ...BLOCKS, "Thêm ô trống", ...HISTORY], true],
    ["question", [...MARKS, ...BLOCKS, ...HISTORY], true],
    ["option", [...MARKS, ...HISTORY], false],
  ] as const)("orders the %s toolbar as the deck does", (profile, names, footer) => {
    renderEditor(
      { profile },
      profile === "option"
        ? plainOptionContent("")
        : { format: "semantic_v1", blocks: [paragraph("")] },
    );
    expect(toolbarNames()).toEqual(names);
    expect(screen.queryByRole("button", { name: /Hình ảnh|Âm thanh/ })).toBeNull();
    const bold = screen.getByRole("button", { name: "In đậm" });
    expect(bold).toHaveAttribute("title", "In đậm (Ctrl+B)");
    expect(bold).toHaveAttribute("aria-keyshortcuts", "Control+B");
    expect(screen.getByRole("button", { name: "Làm lại" })).toHaveAttribute(
      "title",
      "Làm lại (Ctrl+Shift+Z)",
    );
    expect(screen.getByRole("button", { name: "Hoàn tác" })).toBeDisabled();
    expect(fireEvent.pointerDown(bold)).toBe(false);
    expect(fireEvent.mouseDown(bold)).toBe(false);
    expect(screen.queryByText("0 từ") !== null).toBe(footer);
  });

  test("is one tab stop whose arrow keys skip disabled buttons, and reports pressed marks", async () => {
    const editor = renderEditor(
      {},
      { format: "semantic_v1", blocks: [paragraph("Một")] },
    );
    const buttons = within(
      screen.getByRole("toolbar", { name: "Thanh định dạng: Đề bài" }),
    ).getAllByRole("button");
    expect(buttons.filter((button) => button.tabIndex === 0)).toEqual([buttons[0]]);
    act(() => buttons[0]!.focus());
    fireEvent.keyDown(buttons[0]!, { key: "ArrowLeft" });
    const gap = screen.getByRole("button", { name: "Thêm ô trống" });
    expect(gap).toHaveFocus();
    expect(gap.tabIndex).toBe(0);
    fireEvent.keyDown(gap, { key: "Home" });
    expect(buttons[0]).toHaveFocus();
    fireEvent.keyDown(buttons[0]!, { key: "ArrowRight" });
    expect(screen.getByRole("button", { name: "In nghiêng" })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: "End" });
    expect(gap).toHaveFocus();
    expect(screen.getByRole("button", { name: "In đậm" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    act(() => {
      editor().commands.selectAll();
      editor().commands.toggleBold();
    });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "In đậm" })).toHaveAttribute(
        "aria-pressed",
        "true",
      ),
    );
  });

  test("toggles a level-3 heading and is pressed inside any heading", async () => {
    const editor = renderEditor({}, formattingSample);
    act(() => editor().commands.setTextSelection(2));
    const heading = screen.getByRole("button", { name: "Tiêu đề" });
    await waitFor(() => expect(heading).toHaveAttribute("aria-pressed", "true"));
    fireEvent.click(heading);
    expect(editor().getHTML()).toMatch(/^<p>Đọc kỹ/);
    fireEvent.click(heading);
    expect(editor().getHTML()).toMatch(/^<h3>Đọc kỹ/);
  });

  test("shows the table row while the caret is in a table and explains disabled merge and split", async () => {
    const editor = renderEditor();
    fireEvent.click(screen.getByRole("button", { name: "Thêm bảng" }));
    const row = await screen.findByRole("toolbar", { name: "Bảng" });
    expect(
      within(row)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual([
      "Thêm hàng",
      "Thêm cột",
      "Xoá hàng",
      "Xoá cột",
      "Gộp ô",
      "Tách ô",
      "Xoá bảng",
    ]);
    for (const name of ["Gộp ô", "Tách ô"]) {
      const button = within(row).getByRole("button", { name });
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute(
        "title",
        "Chọn từ hai ô trở lên để gộp, hoặc chọn một ô đã gộp để tách",
      );
    }
    expect(screen.getByRole("button", { name: "Đã ở trong bảng" })).toBeDisabled();
    const table = editor().state.doc.firstChild!;
    expect(table.type.name).toBe("table");
    expect(table.childCount).toBe(2);
    expect(table.firstChild!.childCount).toBe(2);
    expect(table.firstChild!.firstChild!.type.name).toBe("tableHeader");
    fireEvent.click(within(row).getByRole("button", { name: "Thêm hàng" }));
    expect(editor().state.doc.firstChild!.childCount).toBe(3);
    fireEvent.click(within(row).getByRole("button", { name: "Xoá bảng" }));
    await waitFor(() =>
      expect(screen.queryByRole("toolbar", { name: "Bảng" })).toBeNull(),
    );
  });

  test("opens the address with the caret after https:// so a typed host follows it", async () => {
    const user = userEvent.setup();
    const editor = renderEditor(
      {},
      { format: "semantic_v1", blocks: [paragraph("Đọc thêm")] },
    );
    act(() => editor().commands.setTextSelection({ from: 1, to: 4 }));
    await user.click(screen.getByRole("button", { name: "Liên kết" }));
    const address = await screen.findByRole("textbox", { name: "Địa chỉ liên kết" });
    await waitFor(() => expect(address).toHaveFocus());
    expect(address).toHaveProperty("selectionStart", "https://".length);
    expect(address).toHaveProperty("selectionEnd", "https://".length);
    await user.keyboard("example.com");
    expect(address).toHaveValue("https://example.com");
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(links(editor())).toEqual([["https://example.com", "Đọc"]]),
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("links a selection, refuses an unsafe address, removes a link and links a bare address", async () => {
    const editor = renderEditor(
      {},
      { format: "semantic_v1", blocks: [paragraph("Đọc thêm")] },
    );
    const button = screen.getByRole("button", { name: "Liên kết" });
    act(() => editor().commands.setTextSelection({ from: 1, to: 4 }));
    fireEvent.click(button);
    expect(
      await screen.findByRole("textbox", { name: "Địa chỉ liên kết" }),
    ).toHaveValue("https://");
    await submitLink("http://example.com");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Hãy dùng liên kết đầy đủ, bắt đầu bằng https://",
    );
    expect(editor().getHTML()).not.toContain("<a");
    await submitLink("https://example.com/doc");
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Địa chỉ liên kết" })).toBeNull(),
    );
    expect(links(editor())).toEqual([["https://example.com/doc", "Đọc"]]);

    act(() => editor().commands.setTextSelection(2));
    await waitFor(() => expect(button).toHaveAttribute("data-active", "true"));
    expect(button).not.toHaveAttribute("aria-pressed");
    expect(button).toHaveAttribute("aria-haspopup", "dialog");
    expect(button).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(button);
    expect(
      await screen.findByRole("textbox", { name: "Địa chỉ liên kết" }),
    ).toHaveValue("https://example.com/doc");
    expect(button).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(screen.getByRole("button", { name: "Gỡ liên kết" }));
    await waitFor(() => expect(links(editor())).toEqual([]));

    act(() => editor().commands.setTextSelection(editor().state.doc.content.size - 1));
    fireEvent.click(button);
    await submitLink("https://quizzivy.example/a");
    await waitFor(() =>
      expect(links(editor())).toEqual([
        ["https://quizzivy.example/a", "quizzivy.example/a"],
      ]),
    );
  });

  test("closes the link popover on Escape and returns focus to its button", async () => {
    renderEditor();
    const button = screen.getByRole("button", { name: "Liên kết" });
    fireEvent.click(button);
    const address = await screen.findByRole("textbox", { name: "Địa chỉ liên kết" });
    await waitFor(() => expect(address).toHaveFocus());
    fireEvent.keyDown(address, { key: "Escape" });
    await waitFor(() => expect(button).toHaveFocus());
    expect(screen.queryByRole("textbox", { name: "Địa chỉ liên kết" })).toBeNull();
  });

  test("counts words on a deferred value and shows the placeholder only while blank", async () => {
    const editor = renderEditor({ placeholder: "Viết câu hỏi" });
    expect(screen.getByText("0 từ")).toBeVisible();
    expect(screen.getByText("Viết câu hỏi")).toHaveAttribute("aria-hidden", "true");
    expect(
      screen.getByText(
        "Định dạng từ Word hoặc Google Docs được giữ lại · Ctrl+Shift+V để dán văn bản thuần",
      ),
    ).toBeVisible();
    act(() => {
      editor().commands.insertContent("Bốn từ tiếng Việt");
    });
    expect(await screen.findByText("4 từ")).toBeVisible();
    expect(screen.queryByText("Viết câu hỏi")).toBeNull();
    act(() => {
      editor().commands.setContent(toEditorJSON(tableSample));
    });
    await waitFor(() => expect(screen.queryByText("4 từ")).toBeNull());
  });

  test("reads no document text when only the selection moves", () => {
    const editor = renderEditor({}, formattingSample);
    const read = vi.spyOn(Object.getPrototypeOf(editor().state.doc), "textBetween");
    act(() => {
      editor().commands.setTextSelection(2);
      editor().commands.setTextSelection(4);
    });
    expect(read).not.toHaveBeenCalled();
    read.mockRestore();
  });

  test("takes the profile's frame unless the host overrides it", () => {
    const { container, unmount } = render(
      <ContentEditor
        initialContent={{ format: "semantic_v1", blocks: [paragraph("")] }}
        label="Đề bài"
        profile="prompt"
        onChange={() => undefined}
      />,
    );
    const box = container.querySelector<HTMLElement>(".content-editor")!;
    expect(box.style.getPropertyValue("--content-editor-min-height")).toBe("96px");
    expect(box.style.getPropertyValue("--content-editor-font-size")).toBe("15px");
    unmount();
    const review = render(
      <ContentEditor
        initialContent={{ format: "semantic_v1", blocks: [paragraph("")] }}
        label="Câu hỏi"
        profile="question"
        minHeight={64}
        footer={false}
        onChange={() => undefined}
      />,
    );
    const reviewBox = review.container.querySelector<HTMLElement>(".content-editor")!;
    expect(reviewBox.style.getPropertyValue("--content-editor-min-height")).toBe(
      "64px",
    );
    expect(reviewBox.style.getPropertyValue("--content-editor-font-size")).toBe("14px");
    expect(screen.queryByText("0 từ")).toBeNull();
  });

  test("renders the read view with no toolbar, no footer and nothing editable", () => {
    renderEditor({ readOnly: true }, formattingSample);
    expect(screen.queryByRole("toolbar")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByText(/ từ$/)).toBeNull();
    expect(screen.getByText("Đọc kỹ phần được gạch chân")).toBeVisible();
  });

  test("shows content its profile cannot hold in the notice band", () => {
    renderEditor({ profile: "option" }, formattingSample);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Nội dung có cấu trúc chưa được hỗ trợ. Bản gốc không bị thay đổi.",
    );
    expect(screen.queryByRole("textbox")).toBeNull();
  });
});
