import { useState } from "react";
import { describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import type { MediaAsset } from "@/features/media/api";
import "@/lib/i18n";

const AUDIO: MediaAsset = {
  id: "018f0000-0000-7000-8000-000000000001",
  kind: "audio",
  originalFilename: "unit5-listening-2.mp3",
  bytes: 2_400_000,
  durationMs: 110_000,
  mimeType: "audio/mpeg",
  createdAt: "2026-01-01T00:00:00Z",
  url: "https://example.test/unit5-listening-2.mp3",
};

function stubDuration(seconds: number) {
  Object.defineProperty(HTMLMediaElement.prototype, "duration", {
    configurable: true,
    get: () => seconds,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set(this: HTMLMediaElement) {
      setTimeout(() => this.onloadedmetadata?.(new Event("loadedmetadata")), 0);
    },
  });
}

function dropOnWindow(file: File) {
  const event = new Event("drop", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "dataTransfer", {
    value: { types: ["Files"], files: [file] },
  });
  act(() => {
    window.dispatchEvent(event);
  });
}

/**
 * §7's five types, one test each, driven through the real controlled component
 * rather than a snapshot: what matters is that switching type keeps the work
 * and that each type's answer editor is the one a teacher can actually operate.
 */
function renderEditor(
  overrides: Partial<QuestionValues> = {},
  asset: MediaAsset | null = null,
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  function Harness() {
    const [value, setValue] = useState<QuestionValues>({
      ...emptyQuestion(),
      ...overrides,
    });
    const [current, setCurrent] = useState(asset);
    return (
      <QuestionEditor
        value={value}
        asset={current}
        onChange={setValue}
        onAssetChange={setCurrent}
      />
    );
  }

  render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the question editor, per question type", () => {
  it("single_choice marks exactly one option correct", async () => {
    const user = renderEditor({
      type: "single_choice",
      options: [
        { id: null, text: "went", isCorrect: true },
        { id: null, text: "have gone", isCorrect: false },
      ],
    });

    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(2);

    await user.click(radios[1]!);

    expect(screen.getAllByRole("radio")[0]).not.toBeChecked();
    expect(screen.getAllByRole("radio")[1]).toBeChecked();
  });

  it("multiple_choice lets two options be correct at once", async () => {
    const user = renderEditor({
      type: "multiple_choice",
      options: [
        { id: null, text: "a", isCorrect: true },
        { id: null, text: "b", isCorrect: false },
      ],
    });

    await user.click(screen.getAllByRole("checkbox")[1]!);

    for (const box of screen.getAllByRole("checkbox")) expect(box).toBeChecked();
  });

  it("true_false is a radio group of two cards, with nothing to add, remove or rename", () => {
    renderEditor({
      type: "true_false",
      options: [
        { id: null, text: "True", isCorrect: true },
        { id: null, text: "False", isCorrect: false },
      ],
    });

    const group = screen.getByRole("radiogroup", {
      name: "Đáp án đúng · học viên chọn một",
    });
    expect(within(group).getAllByRole("radio")).toHaveLength(2);
    expect(within(group).getByRole("radio", { name: "Đúng" })).toBeChecked();
    expect(
      within(group).getByRole("radio", { name: "Đúng" }),
    ).toHaveAccessibleDescription("Đáp án đúng");
    expect(screen.queryByRole("textbox", { name: /Lựa chọn/ })).toBeNull();
    expect(screen.queryByRole("button", { name: "Thêm lựa chọn" })).toBeNull();
    expect(screen.getByText("Chấm tự động khi học viên nộp bài.")).toBeInTheDocument();
  });

  it("fill_blank edits blanks rather than options", async () => {
    const user = renderEditor({
      type: "fill_blank",
      prompt: "She {{1}} here.",
      options: [],
    });

    expect(screen.queryByRole("radio")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Thêm chỗ trống" }));

    expect(screen.getByText("Chỗ trống 1")).toBeInTheDocument();
    expect(
      screen.getByLabelText("Đáp án được chấp nhận cho chỗ trống 1"),
    ).toBeInTheDocument();
  });

  it("short_answer offers an optional sample answer for graders and says it is graded by hand", () => {
    renderEditor({ type: "short_answer", options: [] });

    const field = screen.getByRole("textbox", { name: "Đáp án mẫu" });
    expect(field).toHaveAccessibleDescription(
      "Không bắt buộc. Người chấm thấy đáp án này cạnh câu trả lời của từng học viên.",
    );
    expect(field).toHaveAttribute("placeholder", "Viết một câu trả lời mẫu");
    expect(
      screen.getByText("Chấm tay. Mỗi câu trả lời được đưa vào hàng chờ chấm."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("radio")).toBeNull();
  });

  it("switching type keeps the prompt and the points and swaps only the answer editor", async () => {
    const user = renderEditor({
      type: "single_choice",
      prompt: "They ___ to the museum.",
      points: 2,
    });

    await user.click(screen.getByRole("tab", { name: "Tự luận" }));

    expect(screen.getByLabelText("Nội dung câu hỏi")).toHaveValue(
      "They ___ to the museum.",
    );
    expect(screen.getByLabelText("Điểm")).toHaveValue(2);
    expect(screen.queryByRole("radio")).toBeNull();
  });

  it("attaching audio applies §11.1's defaults without the teacher opening the panel", async () => {
    server.use(
      http.get("http://localhost:8080/teacher/media", () =>
        contractJson("/teacher/media", "get", 200, {
          totalBytes: 2_400_000,
          facets: { all: 1, audio: 1, image: 0, unused: 1 },
          usage: { audioBytes: 2_400_000, imageBytes: 0, quotaBytes: 5_368_709_120 },
          page: 1,
          pageSize: 50,
          total: 0,
          items: [
            {
              ...AUDIO,
              displayName: AUDIO.originalFilename,
              defaultMaxPlays: null,
              width: null,
              height: null,
              questionCount: 0,
            },
          ],
          nextCursor: null,
        }),
      ),
    );
    const user = renderEditor();

    expect(screen.queryByRole("radiogroup", { name: "Số lần nghe" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Từ Media" }));
    await user.click(
      await screen.findByRole("button", { name: /unit5-listening-2\.mp3/ }),
    );
    await user.click(screen.getByRole("button", { name: "Đính kèm" }));

    // §11.1: 2 plays, no seek, transcript after submit.
    const plays = await screen.findByRole("radiogroup", { name: "Số lần nghe" });
    expect(within(plays).getByRole("radio", { name: "Hai lần" })).toBeChecked();
    expect(screen.getByRole("switch", { name: "Cho tua tới" })).not.toBeChecked();
    expect(
      screen.getByRole("switch", { name: "Hiện lời thoại sau khi nộp" }),
    ).toBeChecked();
  });

  it.each(["removed", "replaced by a recording"])(
    "drops an image's alt text when the image is %s, as the server requires",
    async (change) => {
      const IMAGE: MediaAsset = {
        id: "018f0000-0000-7000-8000-000000000002",
        kind: "image",
        originalFilename: "map.png",
        bytes: 24_000,
        mimeType: "image/png",
        createdAt: "2026-01-01T00:00:00Z",
        url: "https://example.test/map.png",
      };
      server.use(
        http.get("http://localhost:8080/teacher/media", () =>
          contractJson("/teacher/media", "get", 200, {
            totalBytes: 2_400_000,
            facets: { all: 1, audio: 1, image: 0, unused: 1 },
            usage: { audioBytes: 2_400_000, imageBytes: 0, quotaBytes: 5_368_709_120 },
            page: 1,
            pageSize: 50,
            total: 0,
            items: [
              {
                ...AUDIO,
                displayName: AUDIO.originalFilename,
                defaultMaxPlays: null,
                width: null,
                height: null,
                questionCount: 0,
              },
            ],
            nextCursor: null,
          }),
        ),
        http.post("http://localhost:8080/teacher/media", () =>
          contractJson("/teacher/media", "post", 201, AUDIO),
        ),
      );
      stubDuration(30);
      const seen: QuestionValues[] = [];
      function Harness() {
        const [value, setValue] = useState<QuestionValues>({
          ...emptyQuestion(),
          mediaAssetId: IMAGE.id,
          mediaAlt: "Bản đồ thị trấn",
        });
        const [current, setCurrent] = useState<MediaAsset | null>(IMAGE);
        return (
          <QuestionEditor
            value={value}
            asset={current}
            onChange={(next) => {
              seen.push(next);
              setValue(next);
            }}
            onAssetChange={setCurrent}
          />
        );
      }
      render(
        <QueryClientProvider
          client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
        >
          <Harness />
        </QueryClientProvider>,
      );
      const user = userEvent.setup();

      if (change === "removed") {
        await user.click(screen.getByRole("button", { name: "Gỡ media" }));
      } else {
        dropOnWindow(new File([new Uint8Array(64)], "unit5-listening-2.mp3"));
        await waitFor(() =>
          expect(seen.at(-1)).toMatchObject({ mediaAssetId: AUDIO.id }),
        );
      }

      expect(seen.at(-1)).toMatchObject({
        mediaAssetId: change === "removed" ? null : AUDIO.id,
        mediaAlt: null,
      });
    },
  );

  it("names the attached file with its length and size", () => {
    renderEditor({ mediaAssetId: AUDIO.id }, AUDIO);

    const name = screen.getByText("unit5-listening-2.mp3");
    expect(name.parentElement).toHaveTextContent("1:50");
    expect(name.parentElement).toHaveTextContent("2.3 MB");
  });
});
