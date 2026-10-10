import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuestionImage } from "@/features/take-test/components/QuestionImage";
import "@/lib/i18n";

const IMAGE = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  kind: "image" as const,
  url: "https://assets.example/map.png",
  mimeType: "image/png" as const,
  bytes: 12,
  originalFilename: "map.png",
  createdAt: "2026-10-01T00:00:00Z",
};

describe("a question's image", () => {
  it("is described by the question's alt text", () => {
    render(<QuestionImage question={{ media: IMAGE, mediaAlt: "Bản đồ thị trấn" }} />);

    expect(screen.getByRole("img", { name: "Bản đồ thị trấn" })).toHaveAttribute(
      "src",
      IMAGE.url,
    );
  });

  it.each([
    ["null", null],
    ["absent", undefined],
  ])("keeps the generic label when the alt text is %s", (_what, mediaAlt) => {
    render(
      <QuestionImage
        question={
          mediaAlt === undefined ? { media: IMAGE } : { media: IMAGE, mediaAlt }
        }
      />,
    );

    expect(
      screen.getByRole("img", { name: "Hình ảnh của câu hỏi" }),
    ).toBeInTheDocument();
  });

  it("draws nothing for a recording, or for no media at all", () => {
    const { container, rerender } = render(
      <QuestionImage
        question={{
          media: { ...IMAGE, kind: "audio", mimeType: "audio/mpeg", durationMs: 1000 },
          mediaAlt: null,
        }}
      />,
    );
    expect(container).toBeEmptyDOMElement();

    rerender(<QuestionImage question={{ media: null }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("offers to read again when the signed URL fails", async () => {
    const onRetry = vi.fn();
    render(
      <QuestionImage
        question={{ media: IMAGE, mediaAlt: "Bản đồ thị trấn" }}
        onRetry={onRetry}
      />,
    );

    fireEvent.error(screen.getByRole("img", { name: "Bản đồ thị trấn" }));
    await userEvent.setup().click(screen.getByRole("button", { name: "Thử lại" }));

    expect(onRetry).toHaveBeenCalledOnce();
  });
});
