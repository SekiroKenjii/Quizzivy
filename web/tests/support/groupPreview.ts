import type { components } from "@/lib/api/schema";
import { tableSample } from "./content-editor/fixtures";

export const previewSection = {
  id: "01935000-0000-7000-8000-000000000001",
  title: "Phần đọc hiểu",
  instructions: "Đọc bài rồi trả lời các câu hỏi.",
};
export const previewQuestions: components["schemas"]["StudentQuestion"][] = [
  1, 2, 3,
].map((n) => ({
  id: `01935000-0000-7000-8000-00000000001${n}`,
  sectionId: previewSection.id,
  type: "single_choice",
  prompt: `Nội dung câu ${n}`,
  points: 1,
  options: [
    {
      id: `01935000-0000-7000-8000-00000000002${n}`,
      text: "Một lựa chọn rất dài để kiểm tra bố cục trên điện thoại",
    },
  ],
}));
const assetId = "01935000-0000-7000-8000-000000000003";
export const previewGroup: components["schemas"]["StudentGroup"] = {
  id: "01935000-0000-7000-8000-000000000002",
  sectionId: previewSection.id,
  title: "Bài đọc và bài nghe dùng chung",
  questionIds: previewQuestions.slice(1).map((q) => q.id),
  instructions: {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [
          { type: "text", text: "Giữ nguyên thứ tự câu trong nhóm.", marks: [] },
        ],
      },
    ],
  },
  stimuli: [
    {
      id: "01935000-0000-7000-8000-000000000004",
      title: "Lịch hoạt động",
      content: {
        format: "semantic_v1",
        blocks: [
          {
            type: "paragraph",
            content: [
              { type: "text", text: "Điền vào chỗ ", marks: [] },
              { type: "gap", id: "material", label: "A" },
            ],
          },
          ...tableSample.blocks,
          { type: "audio", assetId, label: "Hội thoại mẫu" },
        ],
      },
      gaps: [
        { kind: "question", gapId: "material", questionId: previewQuestions[2]!.id },
      ],
    },
  ],
  recordings: [
    {
      id: "01935000-0000-7000-8000-000000000005",
      assetId,
      policy: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
    },
  ],
  assets: [
    {
      id: assetId,
      kind: "audio",
      mimeType: "audio/mpeg",
      bytes: 12,
      durationMs: 1000,
      originalFilename: "synthetic.mp3",
      createdAt: "2026-09-24T00:00:00Z",
      url: "https://assets.example/synthetic.mp3",
    },
  ],
};
