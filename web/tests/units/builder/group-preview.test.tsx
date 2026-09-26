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
    contractJson("/admin/tests/{id}/preview", "get", 200, {
      version: 1,
      questions: previewQuestions,
      sections: [previewSection],
      groups: [previewGroup],
    }),
  ).not.toThrow();
});
