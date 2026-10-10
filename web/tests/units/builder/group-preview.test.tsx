import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "@tests/support/groupPreview";

it("places shared material once before its members and links gaps to their displayed question", async () => {
  const user = userEvent.setup();
  const { container } = render(
    <StudentPreview
      questions={previewQuestions}
      groups={[previewGroup]}
      sections={[previewSection]}
    />,
  );
  expect(screen.getByRole("heading", { name: previewSection.title })).toBeVisible();
  expect(screen.getAllByRole("heading", { name: previewGroup.title })).toHaveLength(1);
  expect(screen.getByText("Ngữ liệu dùng chung · Câu 2–3")).toBeVisible();
  const first = screen.getByText("Nội dung câu 1");
  const material = screen.getByRole("heading", { name: previewGroup.title });
  const member = screen.getByText("Nội dung câu 2");
  expect(
    first.compareDocumentPosition(material) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(
    material.compareDocumentPosition(member) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Ô A — chuyển đến câu 3" }));
  expect(document.activeElement).toHaveTextContent("Câu 3");
  expect(container.querySelectorAll("audio")).toHaveLength(1);
  expect(container.querySelector("audio")).toHaveAttribute("preload", "none");
  expect(container.querySelector("audio")).toHaveAttribute(
    "src",
    previewGroup.assets[0]!.url,
  );
  expect(screen.getByText("Nghe thử không tính lượt làm bài.")).toBeVisible();
});

it("draws the passage and the questions as the engine's panes, on a deck surface, with nothing to answer", () => {
  const { container } = render(
    <StudentPreview
      questions={[
        ...previewQuestions,
        {
          id: "01935000-0000-7000-8000-000000000014",
          sectionId: previewSection.id,
          type: "short_answer",
          prompt: "Viết một câu",
          points: 2,
        },
      ]}
      groups={[previewGroup]}
      sections={[previewSection]}
    />,
  );
  expect(container.firstElementChild).toHaveAttribute("data-scale", "deck");

  const title = screen.getByRole("heading", { level: 3, name: previewGroup.title });
  expect(title).toHaveClass("text-stat", "leading-[1.3]");
  expect(title.parentElement).toHaveClass("max-w-160", "[&_img]:bg-paper");
  expect(screen.getByText("Ngữ liệu dùng chung · Câu 2–3")).toHaveClass(
    "text-meta",
    "uppercase",
  );
  expect(screen.getByText("Lịch hoạt động")).toBeVisible();

  const line = screen.getByText("Câu 1 trên 4 · Chọn một đáp án");
  expect(line).toHaveClass("text-muted-fg", "text-sm");
  expect(line.closest(".bg-sidebar")).toHaveClass("rounded-xl", "border");
  expect(screen.getAllByRole("radio")).toHaveLength(3);
  for (const radio of screen.getAllByRole("radio")) {
    expect(radio).toBeDisabled();
    expect(radio.parentElement).toHaveClass("min-h-13", "rounded-[11px]");
  }
  expect(screen.getByText("Câu 4 trên 4 · Trả lời ngắn")).toBeVisible();
  const field = screen.getByRole("textbox", { name: "Bài làm của bạn" });
  expect(field).toBeDisabled();
  expect(field).toHaveAttribute("placeholder", "Nhập câu trả lời");
  expect(screen.getByText("2 điểm · giáo viên chấm tay")).toBeVisible();
  expect(screen.queryByRole("button", { name: "Đánh dấu xem lại" })).toBeNull();
});

it("does not repeat a material's title when it is the group's", () => {
  render(
    <StudentPreview
      questions={previewQuestions}
      groups={[
        {
          ...previewGroup,
          stimuli: [{ ...previewGroup.stimuli[0]!, title: previewGroup.title }],
        },
      ]}
    />,
  );
  expect(screen.getAllByRole("heading", { name: previewGroup.title })).toHaveLength(1);
});

it("draws a question's own recording and image in the preview", () => {
  const media = {
    id: "01935000-0000-7000-8000-000000000031",
    bytes: 12,
    createdAt: "2026-09-24T00:00:00Z",
  };
  const { container } = render(
    <StudentPreview
      questions={[
        {
          ...previewQuestions[0]!,
          media: {
            ...media,
            kind: "audio",
            mimeType: "audio/mpeg",
            durationMs: 1000,
            originalFilename: "q1.mp3",
            url: "https://assets.example/q1.mp3",
          },
          audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
        },
        {
          ...previewQuestions[1]!,
          media: {
            ...media,
            kind: "image",
            mimeType: "image/png",
            originalFilename: "map.png",
            url: "https://assets.example/map.png",
          },
        },
      ]}
    />,
  );
  expect(container.querySelector("audio")).toHaveAttribute(
    "src",
    "https://assets.example/q1.mp3",
  );
  expect(screen.getByText("Nghe thử không tính lượt làm bài.")).toBeVisible();
  expect(screen.getByAltText("Hình ảnh của câu hỏi")).toHaveAttribute(
    "src",
    "https://assets.example/map.png",
  );
});

it("describes a question's image by its frozen alt text in the preview", () => {
  render(
    <StudentPreview
      questions={[
        {
          ...previewQuestions[1]!,
          media: {
            id: "01935000-0000-7000-8000-000000000032",
            kind: "image",
            mimeType: "image/png",
            bytes: 12,
            originalFilename: "map.png",
            createdAt: "2026-09-24T00:00:00Z",
            url: "https://assets.example/map.png",
          },
          mediaAlt: "Bản đồ trung tâm thị trấn, bưu điện cạnh ngân hàng",
        },
      ]}
    />,
  );
  expect(
    screen.getByAltText("Bản đồ trung tâm thị trấn, bưu điện cạnh ngân hàng"),
  ).toHaveAttribute("src", "https://assets.example/map.png");
  expect(screen.queryByAltText("Hình ảnh của câu hỏi")).toBeNull();
});

it("does not resolve an unbound material asset or recording from its ID", () => {
  const { container, rerender } = render(
    <StudentPreview
      questions={previewQuestions}
      groups={[{ ...previewGroup, assets: [] }]}
    />,
  );
  expect(container.querySelector("audio")).toBeNull();
  expect(screen.getByRole("status")).toHaveTextContent("Ngữ liệu chưa tải được");
  rerender(
    <StudentPreview
      questions={previewQuestions}
      groups={[{ ...previewGroup, recordings: [] }]}
    />,
  );
  expect(container.querySelector("audio")).toBeNull();
});

it("renders legacy flat questions with no group or section metadata", () => {
  render(<StudentPreview questions={previewQuestions} />);
  expect(screen.getByText("Nội dung câu 3")).toBeVisible();
  expect(screen.queryByText(previewGroup.title)).not.toBeInTheDocument();
});

it("recovers failed signed images through the preview refresh action", async () => {
  const retry = vi.fn();
  const image = {
    ...previewGroup.assets[0]!,
    kind: "image" as const,
    mimeType: "image/png" as const,
    url: "https://assets.example/image.png",
  };
  const group = {
    ...previewGroup,
    recordings: [],
    assets: [image],
    stimuli: [
      {
        ...previewGroup.stimuli[0]!,
        content: {
          format: "semantic_v1" as const,
          blocks: [{ type: "image" as const, assetId: image.id, alt: "Sơ đồ mẫu" }],
        },
        gaps: [],
      },
    ],
  };
  render(
    <StudentPreview
      questions={previewQuestions}
      groups={[group]}
      onRetryMedia={retry}
    />,
  );
  fireEvent.error(screen.getByAltText("Sơ đồ mẫu"));
  expect(screen.getByRole("status")).toHaveTextContent("Ngữ liệu chưa tải được");
  await userEvent.setup().click(screen.getByRole("button", { name: "Thử lại" }));
  expect(retry).toHaveBeenCalledOnce();
  expect(screen.getByAltText("Sơ đồ mẫu")).toBeVisible();
});

it("uses a preview fixture accepted by the generated API contract", () => {
  expect(() =>
    contractJson("/teacher/tests/{id}/preview", "get", 200, {
      version: 1,
      questions: previewQuestions,
      sections: [previewSection],
      groups: [previewGroup],
    }),
  ).not.toThrow();
});
