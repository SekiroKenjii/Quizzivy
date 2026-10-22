import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { PromptMedia } from "@/features/take-test/components/PromptMedia";
import { QuestionSheet } from "@/features/take-test/components/QuestionSheet";
import type { StudentQuestion } from "@/features/take-test/api";
import "@/lib/i18n";

const MAP = "018f0000-0000-7000-8000-0000000000c1";
const ANSWER = "018f0000-0000-7000-8000-0000000000c2";
const CREATED = "2026-10-01T00:00:00Z";

const image = {
  id: MAP,
  kind: "image" as const,
  url: "https://assets.example/map.png",
  mimeType: "image/png" as const,
  bytes: 12,
  originalFilename: "map.png",
  createdAt: CREATED,
};

const recording = {
  id: "018f0000-0000-7000-8000-0000000000c3",
  kind: "audio" as const,
  url: "https://assets.example/q.mp3",
  mimeType: "audio/mpeg" as const,
  bytes: 12,
  durationMs: 4000,
  originalFilename: "q.mp3",
  createdAt: CREATED,
};

const text = (value: string) => ({
  type: "paragraph" as const,
  content: [{ type: "text" as const, text: value, marks: [] }],
});

function question(blocks: unknown[], prompt: string): StudentQuestion {
  return {
    id: "018f0000-0000-7000-8000-0000000000d1",
    sectionId: "018f0000-0000-7000-8000-0000000000d2",
    type: "single_choice",
    prompt,
    promptContent: { format: "semantic_v1", blocks } as never,
    points: 1,
    options: [
      { id: "018f0000-0000-7000-8000-0000000000e1", text: "North" },
      { id: "018f0000-0000-7000-8000-0000000000e2", text: "South" },
    ],
  };
}

function sheet(q: StudentQuestion) {
  return render(
    <QuestionSheet
      question={q}
      number={1}
      total={1}
      audio={<div data-testid="question-audio" />}
      answer={undefined}
      onAnswer={() => undefined}
    />,
  );
}

describe("media in a student's prompt", () => {
  it("draws a prompt image from promptAssets with its own alt text", () => {
    sheet({
      ...question(
        [text("Look."), { type: "image", assetId: MAP, alt: "A map of the island" }],
        "Look.\n\nA map of the island",
      ),
      promptAssets: [image],
    });

    expect(screen.getByRole("img", { name: "A map of the island" })).toHaveAttribute(
      "src",
      image.url,
    );
  });

  it("keeps an image with empty alt decorative", () => {
    const { container } = sheet({
      ...question([text("Look."), { type: "image", assetId: MAP, alt: "" }], "Look."),
      promptAssets: [image],
    });

    expect(screen.queryByRole("img")).toBeNull();
    expect(container.querySelector(`img[src="${image.url}"]`)).toHaveAttribute(
      "alt",
      "",
    );
  });

  it("tells a student an expired image did not load and keeps its alt text", () => {
    const { container } = sheet({
      ...question(
        [text("Look."), { type: "image", assetId: MAP, alt: "A map of the island" }],
        "Look.\n\nA map of the island",
      ),
      promptAssets: [image],
    });

    fireEvent.error(container.querySelector(`img[src="${image.url}"]`)!);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Hình ảnh chưa tải được. Hãy thử lại.",
    );
    expect(screen.getByText("A map of the island")).toBeInTheDocument();
  });

  it("shows the alt text when the reader was not signed the file", () => {
    sheet(
      question(
        [text("Look."), { type: "image", assetId: MAP, alt: "A map of the island" }],
        "Look.\n\nA map of the island",
      ),
    );

    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByRole("note")).toHaveTextContent("A map of the island");
  });

  it("puts the question's recording at the marker, not above the prompt", () => {
    const { container } = sheet({
      ...question(
        [text("Listen."), { type: "audio" }, text("Answer.")],
        "Listen.\n\nAnswer.",
      ),
      media: recording,
    });

    const audio = screen.getAllByTestId("question-audio");
    expect(audio).toHaveLength(1);
    const prose = container.querySelector(".semantic-content")!;
    expect(within(prose as HTMLElement).getByTestId("question-audio")).toBe(audio[0]);
    const [before, after] = prose.querySelectorAll("p");
    expect(
      before!.compareDocumentPosition(audio[0]!) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      audio[0]!.compareDocumentPosition(after!) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps the recording above a prompt without a marker", () => {
    const { container } = sheet({
      ...question([text("Listen.")], "Listen."),
      media: recording,
    });

    const prose = container.querySelector(".semantic-content")!;
    expect(screen.getByTestId("question-audio")).toBeInTheDocument();
    expect(within(prose as HTMLElement).queryByTestId("question-audio")).toBeNull();
  });
});

describe("media in a released explanation", () => {
  it("plays the explanation's recording from explanationAssets", () => {
    render(
      <PromptMedia
        assets={[
          {
            ...recording,
            id: ANSWER,
            url: "https://assets.example/answer.mp3",
            originalFilename: "answer.mp3",
          },
        ]}
      >
        <QuestionProse
          field="explanation"
          text="Hear the answer."
          content={{
            format: "semantic_v1",
            blocks: [text("Hear the answer."), { type: "audio", assetId: ANSWER }],
          }}
        />
      </PromptMedia>,
    );

    expect(screen.getByText("Hear the answer.")).toBeInTheDocument();
    expect(
      document.querySelector('audio[src="https://assets.example/answer.mp3"]'),
    ).not.toBeNull();
  });

  it("refuses an explanation that names the prompt's marker", () => {
    render(
      <QuestionProse
        field="explanation"
        text="Hear."
        content={
          {
            format: "semantic_v1",
            blocks: [text("Hear."), { type: "audio" }],
          } as never
        }
      />,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});
