import { afterEach, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { GradingAnswerCard } from "@/features/attempts/pages/teacher/GradingAnswerCard";
import { AnswerReview } from "@/features/attempts/components/AnswerReview";
import type { GradingQueueItem } from "@/features/attempts/api";
import { setLocale } from "@/lib/i18n";
import "@/features/attempts/gradingMessages";
import { ASSIGNMENT_ID, ATTEMPT_ID, ESSAY_ID, STUDENT_ID, review } from "./fixtures";

const item: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Unit 5",
  studentId: STUDENT_ID,
  studentName: "Nguyễn Đức Minh",
  questionNumber: 2,
  type: "short_answer",
  prompt: "Morning habits",
  answer: { type: "text", value: "  First line\nSecond line  " },
  points: 1,
  score: null,
  comment: null,
};

function mountCard(answer = item.answer, onComment = vi.fn()) {
  return render(
    <MemoryRouter>
      <GradingAnswerCard
        item={{ ...item, answer }}
        draft={undefined}
        busy={false}
        finishReady={false}
        position={1}
        total={3}
        onScore={vi.fn()}
        onComment={onComment}
        onPrevious={vi.fn()}
        onNext={vi.fn()}
        onRetryMaterial={vi.fn()}
        onReview={vi.fn()}
      />
    </MemoryRouter>,
  );
}

afterEach(async () => {
  await act(async () => setLocale("vi"));
});

it("uses the deck's local English score and answer labels without changing review copy", async () => {
  await act(async () => setLocale("en"));
  mountCard();
  expect(screen.getByText("Score", { selector: "legend" })).toBeVisible();
  expect(screen.getByText("Student answer", { exact: true })).toBeVisible();
  expect(screen.queryByText("Points", { exact: true })).not.toBeInTheDocument();
  expect(
    screen.queryByText("Student's answer", { exact: true }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Give 0 points" })).toBeVisible();
  expect(screen.getByText(/First line/).textContent).toBe(
    "  First line\nSecond line  ",
  );
});

it("renders Vietnamese labels first and updates the mounted answer when language changes", async () => {
  mountCard();
  const answer = screen.getByText(/First line/);
  const label = screen.getByText("Câu trả lời của học viên", { exact: true });
  const legend = screen.getByText("Điểm", { selector: "legend" });
  expect(label).toBeVisible();
  await act(async () => setLocale("en"));
  expect(screen.getByText("Student answer", { exact: true })).toBe(label);
  expect(screen.getByText("Score", { selector: "legend" })).toBe(legend);
  expect(screen.getByText(/First line/)).toBe(answer);
});

it("keeps the default full-review label and exact unanswered behavior", async () => {
  await act(async () => setLocale("en"));
  const question = review().questions[1]!;
  const view = render(<AnswerReview question={question} answer={undefined} />);
  expect(screen.getByText("Student's answer", { exact: true })).toBeVisible();
  expect(
    screen.getByText("The student didn't answer this question.", { exact: true }),
  ).toBeVisible();
  expect(screen.queryByText("Student answer", { exact: true })).not.toBeInTheDocument();
  view.rerender(
    <AnswerReview
      question={question}
      answer={{ answer: { type: "text", value: "   " } }}
    />,
  );
  expect(
    screen.getByText("The student didn't answer this question.", { exact: true }),
  ).toBeVisible();
});

it("reports an unanswered grading item instead of inventing answer text", async () => {
  await act(async () => setLocale("en"));
  mountCard({ type: "text", value: "   " });
  expect(screen.getByText("Student answer", { exact: true })).toBeVisible();
  expect(
    screen.getByText("The student didn't answer this question.", { exact: true }),
  ).toBeVisible();
  expect(screen.queryByText(/First line/)).not.toBeInTheDocument();
});

it("retains the full-review choice key and picked-answer notes", async () => {
  await act(async () => setLocale("en"));
  const data = review();
  render(
    <AnswerReview
      question={data.questions[0]!}
      answer={data.answers[data.questions[0]!.id]}
    />,
  );
  expect(screen.getByText("Hà Nội", { exact: true })).toBeVisible();
  expect(screen.getByText("Huế", { exact: true })).toBeVisible();
  expect(screen.getByText("student's choice · correct")).toBeVisible();
  expect(screen.queryByText("Student answer", { exact: true })).not.toBeInTheDocument();
});

it("restores the full-review label when the optional grading presentation is removed", async () => {
  await act(async () => setLocale("en"));
  const question = review().questions[1]!;
  const answer = { answer: item.answer };
  const view = render(
    <AnswerReview
      question={question}
      answer={answer}
      shortAnswerPresentation={{ label: "Student answer" }}
    />,
  );
  expect(screen.getByText("Student answer", { exact: true })).toBeVisible();
  view.rerender(<AnswerReview question={question} answer={answer} />);
  expect(screen.getByText("Student's answer", { exact: true })).toBeVisible();
  expect(screen.queryByText("Student answer", { exact: true })).not.toBeInTheDocument();
  expect(screen.getByText(/First line/).textContent).toBe(
    "  First line\nSecond line  ",
  );
});

it("keeps choice and fill-blank answers intact when a grading presentation is supplied", async () => {
  await act(async () => setLocale("en"));
  const data = review();
  const view = render(
    <AnswerReview
      question={data.questions[0]!}
      answer={data.answers[data.questions[0]!.id]}
      shortAnswerPresentation={{ label: "Student answer" }}
    />,
  );
  expect(screen.getByText("student's choice · correct")).toBeVisible();
  expect(screen.getByText("Hà Nội", { exact: true })).toBeVisible();
  expect(screen.queryByText("Student answer", { exact: true })).not.toBeInTheDocument();
  view.rerender(
    <AnswerReview
      question={{
        ...data.questions[1]!,
        type: "fill_blank",
        prompt: "The city has {{1}}.",
        blanks: [
          {
            id: ESSAY_ID,
            ordinal: 1,
            acceptedAnswers: ["rivers"],
            caseSensitive: false,
          },
        ],
      }}
      answer={{ answer: { type: "fill_blank", values: { [ESSAY_ID]: " RIVERS " } } }}
      shortAnswerPresentation={{ label: "Student answer" }}
    />,
  );
  expect(screen.getByText("RIVERS", { exact: true })).toBeVisible();
  expect(screen.getByText("Accepted answers: rivers")).toBeVisible();
  expect(screen.queryByText("Student answer", { exact: true })).not.toBeInTheDocument();
});

it("keeps feedback chips keyboard-operable and appends their existing translated feedback", async () => {
  await act(async () => setLocale("en"));
  const onComment = vi.fn();
  mountCard(item.answer, onComment);
  const chip = screen.getByRole("button", { name: "Check spelling" });
  chip.focus();
  await userEvent.setup().keyboard("{Enter}");
  expect(onComment).toHaveBeenCalledExactlyOnceWith("Check spelling.");
  expect(chip).toHaveFocus();
});
