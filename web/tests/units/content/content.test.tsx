import "@/lib/i18n";
import { render, screen } from "@testing-library/react";
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
