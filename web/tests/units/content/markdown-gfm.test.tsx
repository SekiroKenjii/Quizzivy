import { render } from "@testing-library/react";
import { Markdown } from "@/components/shared/Markdown";
import {
  markdownToQuestionContent,
  questionContentToMarkdown,
} from "@/components/shared/content/editor/markdown";
import type { ContentMark } from "@/components/shared/content/model";
import type { QuestionContent } from "@/components/shared/content/questionContent";

type Block = QuestionContent["blocks"][number];
type Paragraph = Extract<Block, { type: "paragraph" }>;
type Inline = Paragraph["content"][number];
type Text = Extract<Inline, { type: "text" }>;

const text = (value: string, marks: ContentMark[] = []): Text => ({
  type: "text",
  text: value,
  marks,
});
const para = (...content: Inline[]): Paragraph => ({ type: "paragraph", content });

const everything: QuestionContent = {
  format: "semantic_v1",
  blocks: [
    { type: "heading", level: 1, content: [text("Đề đọc hiểu")] },
    { type: "heading", level: 2, content: [text("Phần "), text("A", ["bold"])] },
    { type: "heading", level: 3, content: [text("Câu hỏi #3")] },
    para(
      text("Đọc "),
      text("kỹ", ["bold"]),
      text(", "),
      text("nghiêng", ["italic"]),
      text(" và "),
      text("đậm nghiêng", ["bold", "italic"]),
      text(", "),
      text("gạch", ["strike"]),
      text(" rồi "),
      text("cả ba", ["bold", "italic", "strike"]),
      text("."),
    ),
    para(
      text("Ký tự a*b_c [x] <y> | ~z & #tag `code` \\ 1. "),
      { type: "break" },
      text("dòng hai"),
      { type: "break" },
      text("# không phải tiêu đề"),
    ),
    para(text("1. không phải danh sách")),
    para(text("- cũng không"), { type: "break" }, text("+ hay > trích dẫn")),
    para(
      text("Xem "),
      {
        type: "link",
        href: "https://example.com/đọc?q=1",
        content: [text("tài liệu "), text("này", ["bold"])],
      },
      text(" trước."),
    ),
    {
      type: "list",
      ordered: false,
      start: 1,
      items: [
        [para(text("Ý một"))],
        [
          para(text("Ý hai")),
          {
            type: "list",
            ordered: true,
            start: 3,
            items: [[para(text("ba"))], [para(text("bốn", ["italic"]))]],
          },
        ],
      ],
    },
    {
      type: "list",
      ordered: false,
      start: 1,
      items: [[para(text("Danh sách liền kề"))]],
    },
    {
      type: "table",
      rows: [
        [
          { header: true, rowSpan: 1, colSpan: 1, content: [para(text("Thành phố"))] },
          { header: true, rowSpan: 1, colSpan: 1, content: [para(text("Công viên"))] },
        ],
        [
          {
            header: false,
            rowSpan: 1,
            colSpan: 1,
            content: [para(text("Hà Nội", ["bold"]))],
          },
          { header: false, rowSpan: 1, colSpan: 1, content: [para(text("a | b"))] },
        ],
        [
          { header: false, rowSpan: 1, colSpan: 1, content: [para()] },
          { header: false, rowSpan: 1, colSpan: 1, content: [para(text("31%"))] },
        ],
      ],
    },
    { type: "list", ordered: true, start: 1, items: [[para(text("Cuối"))]] },
  ],
};

test("a document holding every kept construct goes to Markdown and back unchanged", () => {
  const markdown = questionContentToMarkdown(everything);
  expect(markdownToQuestionContent(markdown)).toEqual(everything);
});

test("the serializer drops underline, superscript and subscript and keeps their text", () => {
  const markdown = questionContentToMarkdown({
    format: "semantic_v1",
    blocks: [
      para(
        text("H"),
        text("2", ["subscript"]),
        text("O, x"),
        text("2", ["superscript"]),
        text(" và "),
        text("gạch chân", ["underline", "bold"]),
      ),
    ],
  });
  expect(markdown).toBe("H2O, x2 và **gạch chân**");
  expect(markdownToQuestionContent(markdown)).toEqual({
    format: "semantic_v1",
    blocks: [para(text("H2O, x2 và "), text("gạch chân", ["bold"]))],
  });
});

test("merged table cells are written across the cells they covered", () => {
  const markdown = questionContentToMarkdown({
    format: "semantic_v1",
    blocks: [
      {
        type: "table",
        rows: [
          [{ header: true, rowSpan: 1, colSpan: 2, content: [para(text("Buổi"))] }],
          [
            { header: false, rowSpan: 2, colSpan: 1, content: [para(text("Sáng"))] },
            { header: false, rowSpan: 1, colSpan: 1, content: [para(text("Thứ hai"))] },
          ],
          [{ header: false, rowSpan: 1, colSpan: 1, content: [para(text("Thứ ba"))] }],
        ],
      },
    ],
  });
  expect(markdown).toBe(
    "| Buổi |  |\n| --- | --- |\n| Sáng | Thứ hai |\n|  | Thứ ba |",
  );
});

test.each([
  "| a |\n| :-: |\n| b |",
  "```\ncode\n```",
  "![ảnh](https://example.com/a.png)",
  "#### quá sâu",
])("the converter still refuses what the model cannot hold: %s", (markdown) => {
  expect(markdownToQuestionContent(markdown)).toBeNull();
});

test("the reader renders a table and ~~strikethrough~~", () => {
  const { container } = render(
    <Markdown>{"Đã ~~sai~~ sửa\n\n| A | B |\n| --- | --- |\n| 1 | 2 |"}</Markdown>,
  );
  expect(container.querySelector("del")).toHaveTextContent("sai");
  expect([...container.querySelectorAll("th")].map((cell) => cell.textContent)).toEqual(
    ["A", "B"],
  );
  expect([...container.querySelectorAll("td")].map((cell) => cell.textContent)).toEqual(
    ["1", "2"],
  );
});

test("the reader adds no other GFM syntax: no autolinks, single tildes, task lists or footnotes", () => {
  const { container } = render(
    <Markdown>
      {
        "Xem https://example.com hoặc www.example.com, khoảng ~5 phút.\n\n- [ ] việc\n\nGhi chú[^1]\n\n[^1]: chú thích\n\nĐiền {{1}} và {{2}}."
      }
    </Markdown>,
  );
  expect(container.querySelector("a")).toBeNull();
  expect(container.querySelector("del")).toBeNull();
  expect(container.querySelector("input")).toBeNull();
  expect(container.querySelector("section, sup")).toBeNull();
  expect(container).toHaveTextContent("Xem https://example.com hoặc www.example.com");
  expect(container).toHaveTextContent("~5 phút");
  expect(container).toHaveTextContent("Điền {{1}} và {{2}}.");
});

test("a fill-in-the-blank prompt's {{n}} placeholders survive the converter", () => {
  expect(markdownToQuestionContent("Điền {{1}} và {{2}}.")).toEqual({
    format: "semantic_v1",
    blocks: [para(text("Điền {{1}} và {{2}}."))],
  });
});

test("a heading that ends in a hash and a space keeps the hash", () => {
  const document: QuestionContent = {
    format: "semantic_v1",
    blocks: [{ type: "heading", level: 2, content: [text("Mục #  ")] }],
  };
  const markdown = questionContentToMarkdown(document);
  expect(markdown).toBe("## Mục \\#");
  expect(markdownToQuestionContent(markdown)).toEqual({
    format: "semantic_v1",
    blocks: [{ type: "heading", level: 2, content: [text("Mục #")] }],
  });
});

test("a table cell cannot smuggle markup or a script link to the student", () => {
  const { container } = render(
    <Markdown>
      {
        '| A | B |\n| --- | --- |\n| <table onclick="x">a</table> | [b](javascript:alert(1)) |'
      }
    </Markdown>,
  );
  expect(container.querySelectorAll("table")).toHaveLength(1);
  expect(container.querySelector("table table")).toBeNull();
  expect(container.querySelector("[onclick]")).toBeNull();
  const link = container.querySelector("td a");
  expect(link).toHaveTextContent("b");
  expect(link).not.toHaveAttribute("href");
});

test("a body row wider than its header keeps the header's columns", () => {
  const wide = Array.from({ length: 400 }, (_, index) => String(index)).join(" | ");
  const { container } = render(
    <Markdown>{`| a | b |\n| --- | --- |\n| ${wide} |`}</Markdown>,
  );
  const rows = container.querySelectorAll("tr");
  expect(rows).toHaveLength(2);
  expect(rows[0]!.querySelectorAll("th")).toHaveLength(2);
  expect(rows[1]!.querySelectorAll("td")).toHaveLength(2);
});
