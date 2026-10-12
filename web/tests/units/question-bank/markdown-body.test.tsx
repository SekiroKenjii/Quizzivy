import { useState, type ComponentProps } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MarkdownProseEditor } from "@/features/question-bank/components/MarkdownProseEditor";
import { blankSlots } from "@/features/question-bank/blankSlots";
import {
  insertBlock,
  markdownWords,
  prefixLine,
  wrapSelection,
} from "@/features/question-bank/markdownActions";
import "@/lib/i18n";

function Body({
  initial,
  changes = [],
  ...props
}: Readonly<
  { initial: string; changes?: string[] } & Partial<
    ComponentProps<typeof MarkdownProseEditor>
  >
>) {
  const [value, setValue] = useState(initial);
  return (
    <MarkdownProseEditor
      id="question-prompt"
      label="Nội dung câu hỏi"
      minHeight={96}
      fontSize={15}
      value={value}
      onChange={(next) => {
        changes.push(next);
        setValue(next);
      }}
      {...props}
    />
  );
}

const field = () => screen.getByRole("textbox") as HTMLTextAreaElement;

describe("the Markdown actions", () => {
  test("wrap the selection, or a placeholder they select", () => {
    expect(wrapSelection("Một hai", 4, 7, "**", "**", "chữ đậm")).toEqual({
      value: "Một **hai**",
      start: 6,
      end: 9,
    });
    expect(wrapSelection("Một ", 4, 4, "[", "](https://)", "liên kết")).toEqual({
      value: "Một [liên kết](https://)",
      start: 5,
      end: 13,
    });
  });

  test("prefix the caret's own line", () => {
    expect(prefixLine("một\nhai\nba", 5, "- ")).toEqual({
      value: "một\n- hai\nba",
      start: 7,
      end: 7,
    });
  });

  test("insert a block on lines of its own, with one blank line around it", () => {
    expect(insertBlock("", 0, "| A |")).toEqual({ value: "| A |", start: 5, end: 5 });
    expect(insertBlock("Một", 3, "| A |").value).toBe("Một\n\n| A |");
    expect(insertBlock("Một\n", 4, "| A |").value).toBe("Một\n\n| A |");
    expect(insertBlock("Một\n\nhai", 3, "| A |").value).toBe("Một\n\n| A |\n\nhai");
  });

  test("count words with the syntax read as spaces", () => {
    expect(markdownWords("")).toBe(0);
    expect(markdownWords("### **Đậm** và ~~gạch~~\n- [liên kết](https://x)")).toBe(6);
  });
});

describe("the Markdown body", () => {
  test("draws the deck's toolbar, with no Image and no Audio", () => {
    render(<Body initial="" />);
    const toolbar = screen.getByRole("toolbar", {
      name: "Thanh định dạng: Nội dung câu hỏi",
    });
    expect(
      within(toolbar)
        .getAllByRole("button")
        .map((button) => [
          button.getAttribute("aria-label"),
          button.getAttribute("title"),
        ]),
    ).toEqual([
      ["In đậm", "In đậm · **chữ**"],
      ["In nghiêng", "In nghiêng · *chữ*"],
      ["Gạch ngang", "Gạch ngang · ~~chữ~~"],
      ["Tiêu đề", "Tiêu đề · ### "],
      ["Danh sách gạch đầu dòng", "Danh sách gạch đầu dòng · - "],
      ["Danh sách đánh số", "Danh sách đánh số · 1. "],
      ["Liên kết", "Liên kết · [chữ](https://…)"],
      ["Thêm bảng", "Thêm bảng"],
    ]);
    expect(screen.queryByRole("button", { name: /Hình ảnh|Âm thanh/ })).toBeNull();
    expect(
      screen.getByText(
        "Markdown · **đậm**, *nghiêng*, ### tiêu đề, - danh sách, [liên kết](https://…)",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("0 từ")).toBeInTheDocument();
  });

  test("its actions wrap, prefix and insert at the selection, and keep the selection", async () => {
    const user = userEvent.setup();
    render(<Body initial="Một hai" />);
    field().setSelectionRange(4, 7);
    await user.click(screen.getByRole("button", { name: "In đậm" }));
    expect(field()).toHaveValue("Một **hai**");
    expect([field().selectionStart, field().selectionEnd]).toEqual([6, 9]);
    expect(field()).toHaveFocus();
    field().setSelectionRange(0, 0);
    await user.click(screen.getByRole("button", { name: "Tiêu đề" }));
    expect(field()).toHaveValue("### Một **hai**");
    field().setSelectionRange(15, 15);
    await user.click(screen.getByRole("button", { name: "Thêm bảng" }));
    expect(field()).toHaveValue(
      "### Một **hai**\n\n| Cột 1 | Cột 2 |\n| --- | --- |\n|  |  |",
    );
    expect(screen.getByText("6 từ")).toBeInTheDocument();
  });

  test("Preview draws what the student's reader draws, and an action taken there returns to Write", async () => {
    const user = userEvent.setup();
    render(<Body initial={"Đã ~~sai~~\n\n| A |\n| --- |\n| 1 |"} />);
    const preview = screen.getByRole("tab", { name: "Xem trước" });
    await user.click(preview);
    expect(preview).toHaveAttribute("aria-selected", "true");
    const panel = screen.getByRole("tabpanel");
    expect(panel).toHaveAttribute("aria-labelledby", preview.id);
    expect(panel.querySelector("del")).toHaveTextContent("sai");
    expect(within(panel).getByRole("table")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "In nghiêng" }));
    expect(screen.getByRole("tab", { name: "Soạn" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(field()).toHaveValue("Đã ~~sai~~\n\n| A |\n| --- |\n| 1 |*chữ nghiêng*");
  });

  test("Write and Preview are a tab pair moved with the arrow keys, with an empty state", async () => {
    const user = userEvent.setup();
    render(<Body initial="" />);
    const write = screen.getByRole("tab", { name: "Soạn" });
    expect(write).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tab", { name: "Xem trước" })).toHaveAttribute(
      "tabindex",
      "-1",
    );
    write.focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "Xem trước" })).toHaveFocus();
    expect(screen.getByRole("tabpanel")).toHaveTextContent(
      "Chưa có nội dung để xem trước.",
    );
    await user.keyboard("{ArrowLeft}");
    expect(write).toHaveFocus();
    expect(field()).toBeVisible();
  });

  test("a fill-in-the-blank preview draws its {{n}} placeholders as slots", async () => {
    const user = userEvent.setup();
    render(<Body initial="Điền {{1}} vào ~~đây~~" previewPlugins={[blankSlots]} />);
    await user.click(screen.getByRole("tab", { name: "Xem trước" }));
    expect(screen.getByRole("tabpanel").querySelectorAll("[data-blank]")).toHaveLength(
      1,
    );
  });

  test("a replacement takes the body's place, hides the views and disables the toolbar", () => {
    render(<Body initial="Một" replacement={<p>Bản chuyển đổi</p>} />);
    expect(screen.getByText("Bản chuyển đổi")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.getByRole("button", { name: "In đậm" })).toBeDisabled();
  });

  test("clears the builder's starter prompt on first focus without saving it", async () => {
    const user = userEvent.setup();
    const changes: string[] = [];
    render(
      <Body
        initial="Câu hỏi mới — nhập nội dung ở đây"
        clearOnFocus
        changes={changes}
      />,
    );
    await user.click(field());
    expect(field()).toHaveValue("");
    expect(changes).toEqual([]);
    await user.type(field(), "Của tôi");
    expect(changes.at(-1)).toBe("Của tôi");
  });
});
