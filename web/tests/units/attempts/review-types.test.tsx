import { afterEach, beforeEach, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { AnswerReview } from "@/features/attempts/components/AnswerReview";
import type { AdminQuestion } from "@/features/attempts/api";
import { paperScoreText } from "@/features/attempts/paperScore";
import i18n, { setLocale } from "@/lib/i18n";

const id = (n: number) =>
  `018f0000-0000-7000-8000-0000000c${String(n).padStart(4, "0")}`;

function question(over: Partial<AdminQuestion>): AdminQuestion {
  return {
    id: id(1),
    type: "single_choice",
    prompt: "Prompt",
    points: 1,
    tags: [],
    level: null,
    skill: null,
    options: [],
    blanks: [],
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
    ...over,
  };
}

const option = (n: number, text: string, isCorrect: boolean, key?: string) => ({
  id: id(n),
  ordinal: n,
  text,
  isCorrect,
  ...(key ? { key } : {}),
});

beforeEach(async () => {
  await act(async () => setLocale("en"));
});

afterEach(async () => {
  await act(async () => setLocale("vi"));
});

it("labels true / false / not given by position in the reader's language", async () => {
  const tfng = question({
    type: "tfng",
    options: [
      option(10, "TRUE", false),
      option(11, "FALSE", false),
      option(12, "NOT GIVEN", true),
    ],
  });
  const answer = { answer: { type: "choice" as const, optionIds: [id(12)] } };
  render(<AnswerReview question={tfng} answer={answer} />);
  expect(screen.getByText("True")).toBeVisible();
  expect(screen.getByText("Not given")).toBeVisible();
  expect(screen.queryByText("NOT GIVEN")).toBeNull();
  await act(async () => setLocale("vi"));
  expect(screen.getByText("Không có thông tin")).toBeVisible();
  expect(screen.getByText("Đúng")).toBeVisible();
});

it("shows Choose N's picks against the key", () => {
  const choose = question({
    type: "multiple_choice",
    selectCount: 2,
    options: [
      option(20, "Rent rises", true),
      option(21, "Fewer buses", true),
      option(22, "New parks", false),
    ],
  });
  render(
    <AnswerReview
      question={choose}
      answer={{ answer: { type: "choice", optionIds: [id(20), id(22)] } }}
    />,
  );
  expect(screen.getByText("Choose 2 · 1 of 2 picks correct")).toBeVisible();
  expect(screen.getByText("student's choice · correct")).toBeVisible();
  expect(screen.getByText("student's choice")).toBeVisible();
  expect(screen.getByText("correct answer")).toBeVisible();
});

it("says an over-picked Choose N counts none of its picks", () => {
  const choose = question({
    type: "multiple_choice",
    selectCount: 2,
    options: [
      option(20, "Rent rises", true),
      option(21, "Fewer buses", true),
      option(22, "New parks", false),
    ],
  });
  render(
    <AnswerReview
      question={choose}
      answer={{ answer: { type: "choice", optionIds: [id(20), id(21), id(22)] } }}
    />,
  );
  expect(
    screen.getByText("Choose 2 · 3 picked, more than 2, so none count"),
  ).toBeVisible();
  expect(screen.queryByText(/picks correct/)).toBeNull();
});

it("shows matching as pairs against the key", () => {
  const matching = question({
    type: "matching",
    options: [option(30, "Harbour", false, "i"), option(31, "Market", false, "ii")],
    matchItems: [
      {
        id: id(40),
        ordinal: 0,
        key: "A",
        text: "Paragraph A",
        correctOptionId: id(30),
      },
      {
        id: id(41),
        ordinal: 1,
        key: "B",
        text: "Paragraph B",
        correctOptionId: id(31),
      },
      {
        id: id(42),
        ordinal: 2,
        key: "C",
        text: "Paragraph C",
        correctOptionId: id(30),
      },
    ],
  });
  render(
    <AnswerReview
      question={matching}
      answer={{
        answer: { type: "matching", pairs: { [id(40)]: id(30), [id(41)]: id(30) } },
      }}
    />,
  );
  expect(screen.getByText("Matched to i · Harbour · correct")).toBeVisible();
  expect(screen.getByText("Matched to i · Harbour")).toBeVisible();
  expect(screen.getByText("Not matched")).toBeVisible();
  expect(screen.getByText("Correct answer: ii · Market")).toBeVisible();
  expect(screen.getAllByText(/^Correct answer:/)).toHaveLength(2);
});

it("shows an essay's text, word count, band and criteria", () => {
  const essay = question({
    type: "essay",
    points: 9,
    rubric: [
      { id: id(50), ordinal: 0, label: "Task achievement" },
      { id: id(51), ordinal: 1, label: "Coherence and cohesion" },
    ],
  });
  render(
    <AnswerReview
      question={essay}
      answer={{
        answer: {
          type: "text",
          value: "The chart shows a well-known rise of 3.5 points.",
        },
        band: 6.5,
        criteria: [
          { criterionId: id(50), band: 6 },
          { criterionId: id(51), band: 7 },
        ],
      }}
    />,
  );
  expect(screen.getByText("9 words")).toBeVisible();
  expect(screen.getByText("Band 6.5")).toBeVisible();
  expect(screen.getByText("Task achievement")).toBeVisible();
  expect(screen.getByText("6.0")).toBeVisible();
  expect(screen.getByText("7.0")).toBeVisible();
});

it("badges a short answer past its word limit, counted as the engine counts", () => {
  const short = question({ type: "short_answer", maxWords: 3 });
  const { rerender } = render(
    <AnswerReview
      question={short}
      answer={{ answer: { type: "text", value: "green spaces for the public" } }}
    />,
  );
  expect(screen.getByText("Over the word limit (5 of 3)")).toBeVisible();
  rerender(
    <AnswerReview
      question={short}
      answer={{ answer: { type: "text", value: "public green space" } }}
    />,
  );
  expect(screen.queryByText(/Over the word limit/)).toBeNull();
});

it("writes a paper's band where the server gives one, else its points", () => {
  const en = i18n.getFixedT("en");
  const vi = i18n.getFixedT("vi");
  expect(
    paperScoreText({ earned: 6, total: 9, pendingManual: 0, band: 6 }, "en", en),
  ).toBe("Band 6.0");
  expect(
    paperScoreText({ earned: 6.5, total: 9, pendingManual: 0, band: 6.5 }, "vi", vi),
  ).toBe("Band 6,5");
  const points = paperScoreText({ earned: 27, total: 30, pendingManual: 0 }, "en", en);
  expect(points).toMatch(/27/);
  expect(points).not.toMatch(/band/i);
});
