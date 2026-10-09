import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BlankPromptField } from "@/features/question-bank/components/BlankPromptField";
import { QuestionProseField } from "@/features/question-bank/components/QuestionProseField";
import { emptyQuestion } from "@/features/question-bank/questionSchema";
import type { QuestionContent } from "@/components/shared/content/questionContent";
import "@/lib/i18n";

class StillResize {
  observe() {}
  unobserve() {}
  disconnect() {}
}

Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();

beforeEach(() => vi.stubGlobal("ResizeObserver", StillResize));
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

const RICH: QuestionContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "Đọc ", marks: [] },
        { type: "text", text: "kỹ", marks: ["bold"] },
        { type: "text", text: " và ", marks: [] },
        { type: "text", text: "gạch", marks: ["underline"] },
      ],
    },
  ],
};

type Change = { text: string; content: QuestionContent | null };

function Prose({
  text,
  content = null,
  changes,
}: Readonly<{ text: string; content?: QuestionContent | null; changes: Change[] }>) {
  const [value, setValue] = useState<Change>({ text, content });
  return (
    <QuestionProseField
      id="question-prompt"
      label="Nội dung câu hỏi"
      prompt
      text={value.text}
      content={value.content}
      onChange={(nextText, nextContent) => {
        changes.push({ text: nextText, content: nextContent });
        setValue({ text: nextText, content: nextContent });
      }}
    />
  );
}

function modes() {
  return within(screen.getByRole("group", { name: "Chế độ soạn: Nội dung câu hỏi" }));
}

function pressed() {
  return modes()
    .getAllByRole("button")
    .filter((button) => button.getAttribute("aria-pressed") === "true")
    .map((button) => button.textContent);
}

function markdownField() {
  const field = screen.getByRole("textbox", { name: "Nội dung câu hỏi" });
  expect(field.tagName).toBe("TEXTAREA");
  return field;
}

describe("a prompt's field opens in the form it is stored in", () => {
  test("stored content opens in the rich editor", async () => {
    render(<Prose text="Đọc kỹ và gạch" content={RICH} changes={[]} />);
    expect(pressed()).toEqual(["Văn bản định dạng"]);
    expect(
      await screen.findByRole("textbox", { name: "Nội dung câu hỏi" }),
    ).toHaveTextContent("Đọc kỹ và gạch");
    expect(document.querySelector("textarea")).toBeNull();
  });

  test("a Markdown string opens in the Markdown editor, and a blank field in the rich one", async () => {
    const { unmount } = render(<Prose text="**Đậm** và ~~gạch~~" changes={[]} />);
    expect(pressed()).toEqual(["Markdown"]);
    expect(markdownField()).toHaveValue("**Đậm** và ~~gạch~~");
    expect(screen.getByText("3 từ")).toBeInTheDocument();
    unmount();
    render(<Prose text="" changes={[]} />);
    expect(pressed()).toEqual(["Văn bản định dạng"]);
    expect(
      await screen.findByRole("textbox", { name: "Nội dung câu hỏi" }),
    ).toBeVisible();
  });
});

describe("the two conversions", () => {
  test("Rich text to Markdown asks in place, and Cancel changes nothing", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    render(<Prose text="Đọc kỹ và gạch" content={RICH} changes={changes} />);
    await screen.findByRole("textbox", { name: "Nội dung câu hỏi" });
    const markdown = modes().getByRole("button", { name: "Markdown" });
    await user.click(markdown);
    const ask = screen.getByRole("alertdialog");
    expect(ask).toHaveTextContent(
      "Gạch chân, chỉ số trên và chỉ số dưới không có dạng Markdown nên sẽ bị bỏ.",
    );
    expect(within(ask).getByRole("button", { name: "Huỷ" })).toHaveFocus();
    expect(pressed()).toEqual(["Văn bản định dạng"]);
    await user.click(within(ask).getByRole("button", { name: "Huỷ" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(markdown).toHaveFocus();
    await user.click(markdown);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(changes).toEqual([]);
  });

  test("Switch to Markdown writes the serializer's Markdown and opens it", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    render(<Prose text="Đọc kỹ và gạch" content={RICH} changes={changes} />);
    await screen.findByRole("textbox", { name: "Nội dung câu hỏi" });
    await user.click(modes().getByRole("button", { name: "Markdown" }));
    await user.click(screen.getByRole("button", { name: "Chuyển sang Markdown" }));
    expect(changes).toEqual([{ text: "Đọc **kỹ** và gạch", content: null }]);
    expect(pressed()).toEqual(["Markdown"]);
    expect(markdownField()).toHaveValue("Đọc **kỹ** và gạch");
    expect(markdownField()).toHaveFocus();
  });

  test("Markdown to Rich text shows the result first, and Keep Markdown changes nothing", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    render(
      <Prose
        text={"Một **hai**\n\n| A | B |\n| --- | --- |\n| 1 | 2 |"}
        changes={changes}
      />,
    );
    const rich = modes().getByRole("button", { name: "Văn bản định dạng" });
    await user.click(rich);
    expect(
      screen.getByText(
        "Kiểm tra bản chuyển đổi bên dưới trước khi áp dụng. Nội dung gốc chưa thay đổi.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(document.querySelector("textarea")).toBeNull();
    expect(screen.queryByRole("tab", { name: "Xem trước" })).toBeNull();
    expect(screen.getByRole("button", { name: "In đậm" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Giữ Markdown" }));
    expect(rich).toHaveFocus();
    expect(markdownField()).toHaveValue(
      "Một **hai**\n\n| A | B |\n| --- | --- |\n| 1 | 2 |",
    );
    expect(changes).toEqual([]);
  });

  test("Apply conversion stores the document and its plain text, and opens the rich editor", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    render(<Prose text="Một **hai**" changes={changes} />);
    await user.click(modes().getByRole("button", { name: "Văn bản định dạng" }));
    await user.click(screen.getByRole("button", { name: "Áp dụng chuyển đổi" }));
    expect(changes).toEqual([
      {
        text: "Một hai",
        content: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [
                { type: "text", text: "Một ", marks: [] },
                { type: "text", text: "hai", marks: ["bold"] },
              ],
            },
          ],
        },
      },
    ]);
    expect(pressed()).toEqual(["Văn bản định dạng"]);
    expect(
      await screen.findByRole("textbox", { name: "Nội dung câu hỏi" }),
    ).toHaveTextContent("Một hai");
  });

  test("a Markdown the converter refuses keeps today's message and stays Markdown", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    render(<Prose text={"```\nmã\n```"} changes={changes} />);
    await user.click(modes().getByRole("button", { name: "Văn bản định dạng" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Bản gốc được giữ nguyên");
    expect(screen.queryByRole("button", { name: "Áp dụng chuyển đổi" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Giữ Markdown" }));
    expect(pressed()).toEqual(["Markdown"]);
    expect(changes).toEqual([]);
  });

  test("editing in either mode never changes whether content is stored", async () => {
    const user = userEvent.setup();
    const changes: Change[] = [];
    const { unmount } = render(<Prose text="Một" changes={changes} />);
    await user.type(markdownField(), " hai");
    await user.click(screen.getByRole("button", { name: "In đậm" }));
    expect(changes.length).toBeGreaterThan(1);
    expect(changes.every((change) => change.content === null)).toBe(true);
    unmount();
    changes.length = 0;
    render(<Prose text="Đọc kỹ và gạch" content={RICH} changes={changes} />);
    await screen.findByRole("textbox", { name: "Nội dung câu hỏi" });
    await user.click(screen.getByRole("button", { name: "Thêm bảng" }));
    expect(changes.length).toBeGreaterThan(0);
    expect(changes.every((change) => change.content !== null)).toBe(true);
  });
});

test("rich blank conversion follows the pilot flag while legacy text remains editable", () => {
  vi.stubEnv("VITE_RICH_QUESTION_EDITOR", "false");
  const value = {
    ...emptyQuestion(),
    type: "fill_blank" as const,
    options: [],
    prompt: "Điền {{1}}",
  };
  const { rerender } = render(<BlankPromptField value={value} onChange={vi.fn()} />);
  expect(
    screen.queryByRole("button", { name: "Định dạng: Nội dung câu hỏi" }),
  ).toBeNull();
  vi.stubEnv("VITE_RICH_QUESTION_EDITOR", "true");
  rerender(<BlankPromptField value={value} onChange={vi.fn()} />);
  expect(
    screen.getByRole("button", { name: "Định dạng: Nội dung câu hỏi" }),
  ).toBeEnabled();
});
