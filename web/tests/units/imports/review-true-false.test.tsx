import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { question, option, review, section } from "./fixtures";
import { renderReview, serveReview, type ReviewServer } from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
});

afterEach(() => {
  vi.useRealTimers();
});

function serve(options: ReturnType<typeof option>[]) {
  serveReview(
    review(
      [
        section([
          question({
            id: "q1",
            label: "1",
            type: "true_false",
            options,
            answer: { state: "unknown", optionIds: [], evidence: [] },
          }),
        ]),
      ],
      [],
    ),
    state,
  );
}

describe("a true/false question's options in the review", () => {
  it("read in the teacher's language when they are stored as True and False", async () => {
    serve([option("o1", "T", "True"), option("o2", "F", "False")]);
    await renderReview();

    expect(screen.getByText("Đúng")).toBeInTheDocument();
    expect(screen.getByText("Sai")).toBeInTheDocument();
    expect(screen.queryByText("True")).toBeNull();
  });

  it("show the words as printed when they are anything else", async () => {
    serve([option("o1", "T", "Correct"), option("o2", "F", "Incorrect")]);
    await renderReview();

    expect(screen.getByText("Correct")).toBeInTheDocument();
    expect(screen.getByText("Incorrect")).toBeInTheDocument();
  });
});
