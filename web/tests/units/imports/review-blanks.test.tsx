import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import type { ContentDocument } from "@/components/shared/content/model";
import { EXAM_SOURCE_ID, finding, question, review, section } from "./fixtures";
import { renderReview, serveReview, type ReviewServer } from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

const prompt: ContentDocument = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "Yesterday she ", marks: [] },
        { type: "gap", id: "g1", label: "1" },
        { type: "text", text: " home.", marks: [] },
      ],
    },
  ],
};

const evidence = [{ sourceId: EXAM_SOURCE_ID, blockId: "block-q1", start: 0, end: 5 }];

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
  serveReview(
    review(
      [
        section([
          question({
            id: "q1",
            label: "1",
            type: "fill_blank",
            prompt,
            options: [],
            blanks: [{ gapId: "g1", label: "1", accepted: [], caseSensitive: false }],
            answer: {
              state: "conflict",
              optionIds: [],
              evidence,
              candidates: [
                { value: "went / walked", evidence },
                { value: "go", evidence },
              ],
            },
          }),
        ]),
      ],
      [
        finding({
          id: "f-conflict",
          code: "CONFLICTING_ANSWER_KEYS",
          severity: "blocking",
          target: "q1",
          field: "answer",
        }),
      ],
    ),
    state,
  );
});

afterEach(() => {
  vi.useRealTimers();
});

function lastBlank() {
  const items = state.puts.at(-1)!.sections[0]!.items;
  return items[0]!.question!.blanks[0]!;
}

describe("a fill-blank question's accepted answers", () => {
  it("shows a picked value in the answers field and keeps it when the teacher adds another", async () => {
    const { user } = await renderReview();
    await user.click(
      screen.getByRole("button", { name: "Dùng giá trị này: went / walked" }),
    );
    const field = screen.getByLabelText("Đáp án chấp nhận cho chỗ trống 1");
    expect(field).toHaveValue("went\nwalked");

    await user.type(field, "{Enter}strolled");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts.length).toBeGreaterThan(0));
    expect(lastBlank().accepted).toEqual(["went", "walked", "strolled"]);
  });

  it("never sends more accepted answers than the contract allows, and says so", async () => {
    await renderReview();
    const field = screen.getByLabelText("Đáp án chấp nhận cho chỗ trống 1");
    const lines = Array.from({ length: 22 }, (_, index) => `answer${index}`).join("\n");
    fireEvent.change(field, { target: { value: lines } });

    expect(screen.getByText(/Tối đa 20 đáp án/)).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts.length).toBeGreaterThan(0));
    expect(lastBlank().accepted).toHaveLength(20);
  });
});
