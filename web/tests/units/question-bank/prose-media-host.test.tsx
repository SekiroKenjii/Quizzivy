import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { ProseMediaContext } from "@/components/shared/content/editor/proseMedia";
import type { ProseDocument } from "@/components/shared/content/model";
import type { MediaAsset } from "@/features/media/api";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { useProseMedia } from "@/features/question-bank/useProseMedia";
import "@/lib/i18n";

const RECORDING: MediaAsset = {
  id: "018f0000-0000-7000-8000-0000000000c3",
  kind: "audio",
  url: "https://assets.example/q.mp3",
  mimeType: "audio/mpeg",
  bytes: 12,
  durationMs: 4000,
  originalFilename: "q.mp3",
  createdAt: "2026-10-01T00:00:00Z",
};

const PROMPT: ProseDocument = {
  format: "semantic_v1",
  blocks: [
    { type: "paragraph", content: [{ type: "text", text: "Listen.", marks: [] }] },
    { type: "audio" },
  ],
};

function Page({ onChange }: Readonly<{ onChange: (value: QuestionValues) => void }>) {
  const [value, setValue] = useState<QuestionValues>(() => ({
    ...emptyQuestion(),
    prompt: "Listen.",
    promptContent: PROMPT as never,
    mediaAssetId: RECORDING.id,
    audio: {
      maxPlays: 2,
      allowSeek: false,
      allowPause: true,
      showTranscriptAfterSubmit: false,
    },
  }));
  const [asset, setAsset] = useState<MediaAsset | null>(RECORDING);
  const media = useProseMedia({
    value,
    onChange: (next) => {
      onChange(next);
      setValue(next);
    },
    asset,
    onAssetChange: setAsset,
  });
  return (
    <ProseMediaContext value={media.host}>
      <ContentEditor<ProseDocument>
        label="Đề bài"
        profile="questionPrompt"
        initialContent={PROMPT}
        onChange={(content) =>
          media.change({ ...value, promptContent: content as never })
        }
      />
      <button
        type="button"
        onClick={() =>
          media.media({
            mediaAssetId: null,
            audio: null,
            transcript: null,
            mediaAlt: null,
          })
        }
      >
        Remove question audio
      </button>
      {media.dialogs}
    </ProseMediaContext>
  );
}

describe("a page's prose media host", () => {
  it("takes the marker out in the same change that removes the question's audio", async () => {
    const onChange = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <Page onChange={onChange} />
      </QueryClientProvider>,
    );
    await screen.findByRole("textbox", { name: "Đề bài" });
    expect(screen.getByText("Âm thanh của câu hỏi")).toBeInTheDocument();

    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Remove question audio" }));

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const last = onChange.mock.lastCall![0] as QuestionValues;
    expect(last.mediaAssetId).toBeNull();
    expect(last.audio).toBeNull();
    expect(
      (last.promptContent as ProseDocument).blocks.map((block) => block.type),
    ).toEqual(["paragraph"]);
    for (const [call] of onChange.mock.calls as [QuestionValues][])
      expect(
        call.mediaAssetId === null &&
          (call.promptContent as ProseDocument).blocks.some(
            (block) => block.type === "audio",
          ),
      ).toBe(false);
  });
});
