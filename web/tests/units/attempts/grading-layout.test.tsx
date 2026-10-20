import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { GradingQueueAside } from "@/features/attempts/pages/teacher/GradingQueueAside";
import type { GradingQueue, GradingQueueItem } from "@/features/attempts/api";
import "@/lib/i18n";
import "@/features/attempts/gradingMessages";
import { ASSIGNMENT_ID, ATTEMPT_ID, ESSAY_ID, STUDENT_ID } from "./fixtures";

const first: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Unit 5",
  studentId: STUDENT_ID,
  studentName: "Nguyễn Đức Minh",
  questionNumber: 2,
  type: "short_answer",
  prompt: "Morning habits",
  answer: { type: "text", value: "First line" },
  points: 1,
  score: null,
  comment: null,
};

const second: GradingQueueItem = {
  ...first,
  attemptId: "018f0000-0000-7000-8000-0000000000a8",
  studentId: "018f0000-0000-7000-8000-0000000000e2",
  studentName: "Trần Thu Hà",
};

const queue: GradingQueue = {
  items: [first, second],
  groups: [],
  answersRemaining: 2,
  studentsWaiting: 2,
};

const scrolled = vi.fn();

beforeEach(() => {
  scrolled.mockClear();
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled(this);
  };
});

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

function aside(selected: GradingQueueItem) {
  return (
    <GradingQueueAside
      queue={queue}
      items={[first, second]}
      selected={selected}
      mode="student"
      busy={false}
      onSelect={vi.fn()}
    />
  );
}

it("keeps the active queue row in view when the selection moves", () => {
  const { rerender } = render(aside(first));
  expect(scrolled).toHaveBeenLastCalledWith(
    screen.getByRole("button", { current: true }),
  );
  rerender(aside(second));
  const active = screen.getByRole("button", { current: true });
  expect(active).toHaveAccessibleName(/Trần Thu Hà/);
  expect(scrolled).toHaveBeenLastCalledWith(active);
});
