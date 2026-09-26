import type { ContentBlock, SemanticContent } from "@/components/shared/content/model";

export const paragraph = (text: string): ContentBlock => ({
  type: "paragraph",
  content: text ? [{ type: "text", text, marks: [] }] : [],
});

export const formattingSample: SemanticContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "heading",
      level: 1,
      content: [{ type: "text", text: "Đọc kỹ phần được gạch chân", marks: [] }],
    },
    {
      type: "paragraph",
      content: [
        { type: "text", text: "Tiếng Việt: ", marks: [] },
        {
          type: "text",
          text: "nghiêng, đậm và gạch chân",
          marks: ["bold", "italic", "underline"],
        },
        { type: "text", text: ". H", marks: [] },
        { type: "text", text: "2", marks: ["subscript"] },
        { type: "text", text: "O; x", marks: [] },
        { type: "text", text: "2", marks: ["superscript"] },
        { type: "text", text: ". ", marks: [] },
        { type: "text", text: "Nội dung gạch ngang", marks: ["strike"] },
      ],
    },
    {
      type: "paragraph",
      content: [
        { type: "text", text: "Điền vào ", marks: [] },
        { type: "gap", id: "gap-synthetic-1", label: "1" },
        { type: "text", text: " để hoàn thành câu.", marks: [] },
      ],
    },
    paragraph(
      "Đây là mẫu tổng hợp để kiểm tra trình soạn, không phải đề của giáo viên.",
    ),
  ],
};

export const tableSample: SemanticContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "heading",
      level: 2,
      content: [{ type: "text", text: "Lịch học mẫu", marks: [] }],
    },
    {
      type: "table",
      rows: [
        [
          {
            header: true,
            rowSpan: 1,
            colSpan: 2,
            content: [paragraph("Buổi học & nội dung")],
          },
        ],
        [
          { header: false, rowSpan: 2, colSpan: 1, content: [paragraph("Thứ hai")] },
          { header: false, rowSpan: 1, colSpan: 1, content: [paragraph("Đọc hiểu")] },
        ],
        [
          {
            header: false,
            rowSpan: 1,
            colSpan: 1,
            content: [paragraph("Nghe và ghi chú")],
          },
        ],
      ],
    },
    {
      type: "list",
      ordered: true,
      start: 3,
      items: [
        [paragraph("Đọc bảng trước khi trả lời.")],
        [
          paragraph("Giữ nguyên thứ tự câu hỏi."),
          {
            type: "list",
            ordered: false,
            start: 1,
            items: [[paragraph("Kiểm tra tên buổi học.")]],
          },
        ],
      ],
    },
    paragraph(""),
  ],
};
