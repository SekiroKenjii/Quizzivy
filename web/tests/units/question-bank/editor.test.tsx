import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
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

  it("true_false offers exactly True and False, with nothing to add or remove", () => {
    renderEditor({
      type: "true_false",
      options: [
        { id: null, text: "True", isCorrect: true },
        { id: null, text: "False", isCorrect: false },
      ],
    });

    expect(screen.getByDisplayValue("True")).toBeInTheDocument();
    expect(screen.getByDisplayValue("False")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thêm lựa chọn" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Xoá lựa chọn" })).toBeNull();
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

  it("short_answer exposes a sample answer labelled as admin-only", () => {
    renderEditor({ type: "short_answer", options: [] });

    const label = screen.getByText(/Đáp án mẫu/);
    expect(label).toHaveTextContent("chỉ bạn nhìn thấy");
    expect(screen.getByText(/cần bạn chấm tay/)).toBeInTheDocument();
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
      http.get("http://localhost:8080/admin/media", () =>
        contractJson("/admin/media", "get", 200, {
          totalBytes: 2_400_000,
          page: 1,
          pageSize: 50,
          total: 0,
          items: [AUDIO],
          nextCursor: null,
        }),
      ),
    );
    const user = renderEditor();

    expect(screen.queryByLabelText("Số lần được nghe")).toBeNull();

    await user.click(screen.getByRole("button", { name: "Chọn từ thư viện" }));
    await user.click(
      await screen.findByRole("button", { name: /unit5-listening-2\.mp3/ }),
    );

    // §11.1: 2 plays, no seek, transcript after submit.
    expect(await screen.findByLabelText("Số lần được nghe")).toHaveTextContent("2 lần");
    expect(screen.getByRole("switch", { name: "Cho tua" })).not.toBeChecked();
    expect(
      screen.getByRole("switch", { name: "Hiện lời thoại sau khi nộp" }),
    ).toBeChecked();
  });

  it("names the attached file with its length and size", () => {
    renderEditor({ mediaAssetId: AUDIO.id }, AUDIO);

    const name = screen.getByText("unit5-listening-2.mp3");
    expect(name.parentElement).toHaveTextContent("1:50");
    expect(name.parentElement).toHaveTextContent("2.3 MB");
  });
});
